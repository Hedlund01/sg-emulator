package scalegraph

import (
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"testing"

	"sg-emulator/internal/ca"
	camocks "sg-emulator/internal/ca/mocks"
	"sg-emulator/internal/crypto"
)

// testLogger creates a logger configured for testing
func testLogger() *slog.Logger {
	// Create a no-op logger for tests
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

// setupMockCAForApp creates a mock CA configured for app testing
func setupMockCAForApp(t *testing.T) *camocks.MockCertificateAuthority {
	t.Helper()
	mockCA := camocks.NewMockCertificateAuthority(t)

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

	return mockCA
}

// createTestAccountWithKeys creates a test account with generated keys
func createTestAccountWithKeys(t *testing.T, app *App, mockCA *camocks.MockCertificateAuthority, balance float64) *Account {
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

// newTestAccount creates a test account with generated keys for account tests
func newTestAccount() (*Account, error) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	accountIDBytes := crypto.DeriveAccountID(keyPair.PublicKey)
	accountID := fmt.Sprintf("%x", accountIDBytes)

	// Create a CA keypair for signing
	caKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	// Create CA certificate
	caCert, err := ca.CreateCACertificate(caKeyPair.PublicKey, caKeyPair.PrivateKey)
	if err != nil {
		return nil, err
	}

	// Create account certificate signed by CA
	cert, err := ca.CreateAccountCertificate(accountID, keyPair.PublicKey, caCert, caKeyPair.PrivateKey)
	if err != nil {
		return nil, err
	}

	return newAccountWithPublicKey(keyPair.PublicKey, cert)
}
