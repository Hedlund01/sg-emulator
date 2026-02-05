package ca

import (
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"log/slog"

	"sg-emulator/internal/crypto"
)

// CA represents a Certificate Authority for the Scalegraph emulator
type CA struct {
	privateKey  ed25519.PrivateKey
	publicKey   ed25519.PublicKey
	certificate *x509.Certificate
	store       *Store
	logger      *slog.Logger
}

// New creates a new CA, either by loading existing credentials or bootstrapping new ones
func New(baseDir string, logger *slog.Logger) (*CA, error) {
	store := NewStore(baseDir)
	ca := &CA{
		store:  store,
		logger: logger,
	}

	if store.CAExists() {
		logger.Info("Loading existing CA credentials")
		if err := ca.load(); err != nil {
			return nil, fmt.Errorf("failed to load CA: %w", err)
		}
		logger.Info("CA loaded successfully")
	} else {
		logger.Info("Bootstrapping new CA")
		if err := ca.bootstrap(); err != nil {
			return nil, fmt.Errorf("failed to bootstrap CA: %w", err)
		}
		logger.Info("CA bootstrapped successfully")
	}

	return ca, nil
}

// bootstrap creates a new CA with fresh keys and certificate
func (ca *CA) bootstrap() error {
	// Generate CA key pair
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate CA key pair: %w", err)
	}

	ca.privateKey = keyPair.PrivateKey
	ca.publicKey = keyPair.PublicKey

	// Create self-signed CA certificate
	cert, err := CreateCACertificate(ca.publicKey, ca.privateKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}
	ca.certificate = cert

	// Persist to storage
	if err := ca.store.SaveCA(ca.privateKey, ca.certificate); err != nil {
		return fmt.Errorf("failed to save CA credentials: %w", err)
	}

	return nil
}

// load loads existing CA credentials from storage
func (ca *CA) load() error {
	privKey, cert, err := ca.store.LoadCA()
	if err != nil {
		return err
	}

	ca.privateKey = privKey
	ca.publicKey = privKey.Public().(ed25519.PublicKey)
	ca.certificate = cert

	return nil
}

// Certificate returns the CA certificate
func (ca *CA) Certificate() *x509.Certificate {
	return ca.certificate
}

// CertificatePEM returns the CA certificate in PEM format
func (ca *CA) CertificatePEM() string {
	return crypto.EncodeCertificatePEM(ca.certificate)
}

// IssueCertificate creates and signs a certificate for an account
func (ca *CA) IssueCertificate(accountID string, pubKey ed25519.PublicKey) (*x509.Certificate, error) {
	cert, err := CreateAccountCertificate(accountID, pubKey, ca.certificate, ca.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to issue certificate: %w", err)
	}

	ca.logger.Info("Issued certificate", "account_id", accountID)
	return cert, nil
}

// CreateAccountCredentials generates a key pair and certificate for a new account
// Returns the key pair, certificate, and the derived account ID
func (ca *CA) CreateAccountCredentials() (*crypto.KeyPair, *x509.Certificate, string, error) {
	// Generate account key pair
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to generate account key pair: %w", err)
	}

	// Derive account ID from public key
	accountIDBytes := crypto.DeriveAccountID(keyPair.PublicKey)
	accountID := fmt.Sprintf("%x", accountIDBytes)

	// Issue certificate
	cert, err := ca.IssueCertificate(accountID, keyPair.PublicKey)
	if err != nil {
		return nil, nil, "", err
	}

	// Store credentials
	if err := ca.store.SaveAccountCredentials(keyPair.PrivateKey, keyPair.PublicKey, cert); err != nil {
		return nil, nil, "", fmt.Errorf("failed to save account credentials: %w", err)
	}

	ca.logger.Info("Created account credentials", "account_id", accountID)
	return keyPair, cert, accountID, nil
}

// GetAccountPrivateKeyPEM retrieves the PEM-encoded private key for an account
func (ca *CA) GetAccountPrivateKeyPEM(accountID string) (string, error) {
	return ca.store.GetAccountPrivateKeyPEM(accountID)
}

// GetAccountCertificatePEM retrieves the PEM-encoded certificate for an account
func (ca *CA) GetAccountCertificatePEM(accountID string) (string, error) {
	return ca.store.GetAccountCertificatePEM(accountID)
}

// VerifyCertificate verifies that a certificate was signed by this CA
func (ca *CA) VerifyCertificate(cert *x509.Certificate) error {
	verifier := crypto.NewVerifier(ca.certificate)
	return verifier.VerifyCertificate(cert)
}

// NewVerifier creates a new Verifier using this CA's certificate
func (ca *CA) NewVerifier() *crypto.Verifier {
	return crypto.NewVerifier(ca.certificate)
}
