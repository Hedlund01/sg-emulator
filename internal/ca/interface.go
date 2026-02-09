package ca

import (
	"crypto/ed25519"
	"crypto/x509"

	"sg-emulator/internal/crypto"
)

// CertificateAuthority defines the interface for certificate authority operations.
// The CA struct implements this interface.
type CertificateAuthority interface {
	// Certificate returns the CA certificate
	Certificate() *x509.Certificate

	// CertificatePEM returns the CA certificate in PEM format
	CertificatePEM() string

	// IssueCertificate creates and signs a certificate for an account
	IssueCertificate(accountID string, pubKey ed25519.PublicKey) (*x509.Certificate, error)

	// CreateAccountCredentials generates a key pair and certificate for a new account.
	// Returns the key pair, certificate, and the derived account ID.
	CreateAccountCredentials() (*crypto.KeyPair, *x509.Certificate, string, error)

	// GetAccountPrivateKeyPEM retrieves the PEM-encoded private key for an account
	GetAccountPrivateKeyPEM(accountID string) (string, error)

	// GetAccountCertificatePEM retrieves the PEM-encoded certificate for an account
	GetAccountCertificatePEM(accountID string) (string, error)

	// VerifyCertificate verifies that a certificate was signed by this CA
	VerifyCertificate(cert *x509.Certificate) error

	// NewVerifier creates a new Verifier using this CA's certificate
	NewVerifier() *crypto.Verifier
}
