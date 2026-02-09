package server

import (
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"sg-emulator/internal/ca"
	camocks "sg-emulator/internal/ca/mocks"
	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

// newTestLogger creates a logger configured for testing
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
}

// setupTestServerWithMockCA creates a server with a mock CA configured for testing
func setupTestServerWithMockCA(tb testing.TB) (*Server, *camocks.MockCertificateAuthority) {
	tb.Helper()
	logger := newTestLogger()
	mockCA := camocks.NewMockCertificateAuthority(tb)

	// Configure mock to return valid credentials when CreateAccountCredentials is called
	mockCA.EXPECT().CreateAccountCredentials().RunAndReturn(func() (*crypto.KeyPair, *x509.Certificate, string, error) {
		// Generate real keypair and certificate for testing
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			return nil, nil, "", err
		}

		// Derive account ID from public key
		accountIDBytes := crypto.DeriveAccountID(keyPair.PublicKey)
		accountID := fmt.Sprintf("%x", accountIDBytes)

		// Create a CA keypair for signing
		caKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			return nil, nil, "", err
		}

		// Create CA certificate
		caCert, err := ca.CreateCACertificate(caKeyPair.PublicKey, caKeyPair.PrivateKey)
		if err != nil {
			return nil, nil, "", err
		}

		// Create account certificate signed by CA
		cert, err := ca.CreateAccountCertificate(accountID, keyPair.PublicKey, caCert, caKeyPair.PrivateKey)
		if err != nil {
			return nil, nil, "", err
		}

		return keyPair, cert, accountID, nil
	}).Maybe()

	// Configure NewVerifier to return a basic verifier
	mockCA.EXPECT().NewVerifier().Return(&crypto.Verifier{}).Maybe()

	srv := NewWithCA(logger, mockCA)
	return srv, mockCA
}

// createTestAccount creates a test account with generated keys
func createTestAccount(t *testing.T, app *scalegraph.App, mockCA *camocks.MockCertificateAuthority, balance float64) *scalegraph.Account {
	t.Helper()

	// Generate keypair and certificate
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Derive account ID
	accountIDBytes := crypto.DeriveAccountID(keyPair.PublicKey)
	accountID := fmt.Sprintf("%x", accountIDBytes)

	// Create a CA keypair for signing
	caKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate CA keypair: %v", err)
	}

	// Create CA certificate
	caCert, err := ca.CreateCACertificate(caKeyPair.PublicKey, caKeyPair.PrivateKey)
	if err != nil {
		t.Fatalf("Failed to create CA certificate: %v", err)
	}

	// Create account certificate signed by CA
	cert, err := ca.CreateAccountCertificate(accountID, keyPair.PublicKey, caCert, caKeyPair.PrivateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Create account with keys
	acc, err := app.CreateAccountWithKeys(context.Background(), keyPair.PublicKey, cert, balance)
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	return acc
}
