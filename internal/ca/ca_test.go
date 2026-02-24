package ca

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"sg-emulator/internal/crypto"
)

func TestCABootstrapAndLoad(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "ca-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.Default()

	// Bootstrap new CA
	ca1, err := NewCA(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	// Verify CA files exist
	caDir := filepath.Join(tmpDir, "bin", "ca")
	if _, err := os.Stat(filepath.Join(caDir, "ca.key")); os.IsNotExist(err) {
		t.Error("CA private key file not created")
	}
	if _, err := os.Stat(filepath.Join(caDir, "ca.crt")); os.IsNotExist(err) {
		t.Error("CA certificate file not created")
	}

	// Load existing CA
	ca2, err := NewCA(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to load CA: %v", err)
	}

	// Verify same certificate
	if ca1.CertificatePEM() != ca2.CertificatePEM() {
		t.Error("Loaded CA has different certificate than bootstrapped CA")
	}
}

func TestCAIssueCertificate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ca-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.Default()
	certAuth, err := NewCA(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	// Generate account key pair
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	accountID := "test-account-id-123"
	cert, err := certAuth.IssueCertificate(accountID, kp.PublicKey)
	if err != nil {
		t.Fatalf("Failed to issue certificate: %v", err)
	}

	if cert.Subject.CommonName != accountID {
		t.Errorf("Certificate CN mismatch: expected %s, got %s", accountID, cert.Subject.CommonName)
	}

	// Verify certificate chain
	err = certAuth.VerifyCertificate(cert)
	if err != nil {
		t.Errorf("Certificate verification failed: %v", err)
	}
}

func TestCACreateAccountCredentials(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ca-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.Default()
	certAuth, err := NewCA(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	kp, cert, accountID, err := certAuth.CreateAccountCredentials()
	if err != nil {
		t.Fatalf("Failed to create account credentials: %v", err)
	}

	// Verify account ID is derived from public key
	derivedID := crypto.DeriveAccountID(kp.PublicKey)
	derivedIDHex := make([]byte, 40)
	for i, b := range derivedID {
		const hex = "0123456789abcdef"
		derivedIDHex[i*2] = hex[b>>4]
		derivedIDHex[i*2+1] = hex[b&0xf]
	}

	if accountID != string(derivedIDHex) {
		t.Errorf("Account ID mismatch: expected %s, got %s", string(derivedIDHex), accountID)
	}

	// Verify certificate
	err = certAuth.VerifyCertificate(cert)
	if err != nil {
		t.Errorf("Account certificate verification failed: %v", err)
	}

	// Verify credentials are stored
	privKeyPEM, err := certAuth.GetAccountPrivateKeyPEM(accountID)
	if err != nil {
		t.Errorf("Failed to get stored private key: %v", err)
	}
	if privKeyPEM == "" {
		t.Error("Stored private key is empty")
	}

	certPEM, err := certAuth.GetAccountCertificatePEM(accountID)
	if err != nil {
		t.Errorf("Failed to get stored certificate: %v", err)
	}
	if certPEM == "" {
		t.Error("Stored certificate is empty")
	}
}

func TestVerifier(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ca-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.Default()
	certAuth, err := NewCA(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	verifier := certAuth.NewVerifier()

	// Create account credentials
	kp, cert, accountID, err := certAuth.CreateAccountCredentials()
	if err != nil {
		t.Fatalf("Failed to create account credentials: %v", err)
	}

	// Create and sign a transfer request
	payload := &crypto.TransferPayload{
		From:      accountID,
		To:        "destination-account",
		Amount:    100.0,
		Nonce:     1,
		Timestamp: 1234567890,
	}

	sig, err := crypto.Sign(payload, kp.PrivateKey, accountID)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	// Verify certificate
	err = verifier.VerifyCertificate(cert)
	if err != nil {
		t.Errorf("Certificate verification failed: %v", err)
	}

	// Verify signature
	err = verifier.VerifySignature(payload, sig, kp.PublicKey)
	if err != nil {
		t.Errorf("Signature verification failed: %v", err)
	}

	// Verify account ID
	err = verifier.VerifyAccountID(sig, kp.PublicKey)
	if err != nil {
		t.Errorf("Account ID verification failed: %v", err)
	}
}
