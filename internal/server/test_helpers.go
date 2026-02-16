package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"testing"
	"time"

	"sg-emulator/internal/ca"
	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

// newTestLogger creates a logger configured for testing
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
}

// createTestAccountDirect creates an account directly on the app using generated keys.
// Use this in tests that don't go through the full server/client/CA flow.
func createTestAccountDirect(t *testing.T, app *scalegraph.App, balance float64) *scalegraph.Account {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Create a self-signed cert
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pub, ed25519.NewKeyFromSeed(make([]byte, 32)))
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	acc, err := app.CreateAccountWithKeys(context.Background(), pub, cert, balance)
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}
	return acc
}

// newTestServer creates a server with a temporary CA for testing
func newTestServer(logger *slog.Logger) (*Server, func(), error) {
	// Create temporary directory for CA
	tmpDir, err := os.MkdirTemp("", "ca-test-*")
	if err != nil {
		return nil, nil, err
	}

	// Create CA
	certAuth, err := ca.New(tmpDir, logger)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, nil, err
	}

	// Create server with CA
	srv := NewWithCA(logger, certAuth)

	// Return cleanup function
	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return srv, cleanup, nil
}

// createSignedAccountRequest creates a signed account creation request for testing.
// It uses the CA's identity (system account) to sign the request, mirroring how the MCP transport works.
func createSignedAccountRequest(srv *Server, balance float64) (*crypto.SignedEnvelope[*crypto.CreateAccountRequest], error) {
	ca := srv.CA()
	if ca == nil {
		return nil, fmt.Errorf("no CA available on server")
	}

	// Use the CA's own private key and certificate directly for account creation
	systemAccountID := scalegraph.ScalegraphIdFromPublicKey(ca.PublicKey())
	accountIDStr := systemAccountID.String()

	createReq := &crypto.CreateAccountRequest{
		InitialBalance: balance,
	}

	return crypto.CreateSignedEnvelope(createReq, ca.PrivateKey(), accountIDStr, ca.CertificatePEM())
}

// createTestAccount is a convenience wrapper that creates a signed account request,
// calls CreateAccountWithCredentials, and returns the account.
// This replaces the old client.CreateAccount() pattern in tests.
func createTestAccount(ctx context.Context, srv *Server, client *Client, balance float64) (*scalegraph.Account, error) {
	signedReq, err := createSignedAccountRequest(srv, balance)
	if err != nil {
		return nil, fmt.Errorf("failed to create signed account request: %v", err)
	}

	resp, err := client.CreateAccountWithCredentials(ctx, balance, signedReq)
	if err != nil {
		return nil, err
	}
	return resp.Account, nil
}

// createSignedGetAccountRequest creates a signed get account request for testing.
// It uses the account's own credentials to sign the request.
func createSignedGetAccountRequest(srv *Server, accountID scalegraph.ScalegraphId) (*crypto.SignedEnvelope[*crypto.GetAccountRequest], error) {
	ca := srv.CA()
	if ca == nil {
		return nil, fmt.Errorf("no CA available on server")
	}

	accountIDStr := accountID.String()

	privKeyPEM, err := ca.GetAccountPrivateKeyPEM(accountIDStr)
	if err != nil {
		return nil, err
	}

	certPEM, err := ca.GetAccountCertificatePEM(accountIDStr)
	if err != nil {
		return nil, err
	}

	privKey, err := crypto.DecodePrivateKeyPEM([]byte(privKeyPEM))
	if err != nil {
		return nil, err
	}

	getReq := &crypto.GetAccountRequest{
		AccountID: accountIDStr,
	}

	return crypto.CreateSignedEnvelope(getReq, privKey, accountIDStr, certPEM)
}

// getTestAccount is a convenience wrapper that creates a signed get account request
// and calls client.GetAccount. This replaces direct client.GetAccount() calls in tests.
func getTestAccount(ctx context.Context, srv *Server, client *Client, accountID scalegraph.ScalegraphId) (*scalegraph.Account, error) {
	signedReq, err := createSignedGetAccountRequest(srv, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to create signed get account request: %v", err)
	}
	return client.GetAccount(ctx, accountID, signedReq)
}

// createSignedTransfer creates a signed transfer envelope for testing
func createSignedTransfer(ctx context.Context, srv *Server, client *Client, fromID, toID scalegraph.ScalegraphId, amount float64) (*crypto.SignedEnvelope[*crypto.TransferRequest], error) {
	// Get the CA from server
	ca := srv.CA()
	if ca == nil {
		return nil, context.DeadlineExceeded
	}

	// Get from account ID as string
	fromIDStr := fromID.String()

	// Retrieve private key for the from account
	privKeyPEM, err := ca.GetAccountPrivateKeyPEM(fromIDStr)
	if err != nil {
		return nil, err
	}

	// Retrieve certificate for the from account
	certPEM, err := ca.GetAccountCertificatePEM(fromIDStr)
	if err != nil {
		return nil, err
	}

	// Decode private key
	privKey, err := crypto.DecodePrivateKeyPEM([]byte(privKeyPEM))
	if err != nil {
		return nil, err
	}

	// Get account to calculate nonce
	fromAccount, err := getTestAccount(ctx, srv, client, fromID)
	if err != nil {
		return nil, err
	}
	nonce := fromAccount.GetNonce() + 1

	// Create transfer request
	transferReq := &crypto.TransferRequest{
		From:      fromIDStr,
		To:        toID.String(),
		Amount:    amount,
		Nonce:     nonce,
		Timestamp: time.Now().Unix(),
	}

	// Create signed envelope
	return crypto.CreateSignedEnvelope(transferReq, privKey, fromIDStr, certPEM)
}
