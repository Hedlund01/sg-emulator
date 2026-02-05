package ca

import (
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"sg-emulator/internal/crypto"
)

const (
	caDir       = "bin/ca"
	accountsDir = "bin/accounts"
	caKeyFile   = "ca.key"
	caCertFile  = "ca.crt"
	privKeyFile = "private.key"
	pubKeyFile  = "public.key"
	certFile    = "certificate.crt"
)

// Store handles persistence of CA and account keys/certificates
type Store struct {
	baseDir string
}

// NewStore creates a new Store with the given base directory
func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// CAExists checks if CA keys and certificate already exist
func (s *Store) CAExists() bool {
	keyPath := filepath.Join(s.baseDir, caDir, caKeyFile)
	certPath := filepath.Join(s.baseDir, caDir, caCertFile)

	_, keyErr := os.Stat(keyPath)
	_, certErr := os.Stat(certPath)

	return keyErr == nil && certErr == nil
}

// SaveCA saves the CA private key and certificate
func (s *Store) SaveCA(privKey ed25519.PrivateKey, cert *x509.Certificate) error {
	dir := filepath.Join(s.baseDir, caDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create CA directory: %w", err)
	}

	// Save private key
	keyPEM, err := crypto.EncodePrivateKeyPEM(privKey)
	if err != nil {
		return fmt.Errorf("failed to encode CA private key: %w", err)
	}
	keyPath := filepath.Join(dir, caKeyFile)
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write CA private key: %w", err)
	}

	// Save certificate
	certPEM := crypto.EncodeCertificatePEM(cert)
	certPath := filepath.Join(dir, caCertFile)
	if err := os.WriteFile(certPath, []byte(certPEM), 0644); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	return nil
}

// LoadCA loads the CA private key and certificate
func (s *Store) LoadCA() (ed25519.PrivateKey, *x509.Certificate, error) {
	dir := filepath.Join(s.baseDir, caDir)

	// Load private key
	keyPath := filepath.Join(dir, caKeyFile)
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA private key: %w", err)
	}
	privKey, err := crypto.DecodePrivateKeyPEM(keyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode CA private key: %w", err)
	}

	// Load certificate
	certPath := filepath.Join(dir, caCertFile)
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}
	cert, err := crypto.ParseCertificatePEM(string(certPEM))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	return privKey, cert, nil
}

// SaveAccountCredentials saves an account's private key, public key, and certificate
func (s *Store) SaveAccountCredentials(privKey ed25519.PrivateKey, pubKey ed25519.PublicKey, cert *x509.Certificate) error {
	accountID := fmt.Sprintf("%x", crypto.DeriveAccountID(pubKey))
	// Use first 16 characters of account ID as directory name
	dirName := accountID
	dir := filepath.Join(s.baseDir, accountsDir, dirName)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create account directory: %w", err)
	}

	// Save private key
	privKeyPEM, err := crypto.EncodePrivateKeyPEM(privKey)
	if err != nil {
		return fmt.Errorf("failed to encode account private key: %w", err)
	}
	privKeyPath := filepath.Join(dir, privKeyFile)
	if err := os.WriteFile(privKeyPath, privKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write account private key: %w", err)
	}

	// Save public key
	pubKeyPEM, err := crypto.EncodePublicKeyPEM(pubKey)
	if err != nil {
		return fmt.Errorf("failed to encode account public key: %w", err)
	}
	pubKeyPath := filepath.Join(dir, pubKeyFile)
	if err := os.WriteFile(pubKeyPath, pubKeyPEM, 0644); err != nil {
		return fmt.Errorf("failed to write account public key: %w", err)
	}

	// Save certificate
	certPEM := crypto.EncodeCertificatePEM(cert)
	certPath := filepath.Join(dir, certFile)
	if err := os.WriteFile(certPath, []byte(certPEM), 0644); err != nil {
		return fmt.Errorf("failed to write account certificate: %w", err)
	}

	return nil
}

// LoadAccountCredentials loads an account's credentials
func (s *Store) LoadAccountCredentials(accountID string) (ed25519.PrivateKey, ed25519.PublicKey, *x509.Certificate, error) {
	// Use first 16 characters of account ID as directory name
	dirName := accountID
	dir := filepath.Join(s.baseDir, accountsDir, dirName)

	// Load private key
	privKeyPath := filepath.Join(dir, privKeyFile)
	privKeyPEM, err := os.ReadFile(privKeyPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read account private key: %w", err)
	}
	privKey, err := crypto.DecodePrivateKeyPEM(privKeyPEM)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode account private key: %w", err)
	}

	// Load public key
	pubKeyPath := filepath.Join(dir, pubKeyFile)
	pubKeyPEM, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read account public key: %w", err)
	}
	pubKey, err := crypto.DecodePublicKeyPEM(pubKeyPEM)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode account public key: %w", err)
	}

	// Load certificate
	certPath := filepath.Join(dir, certFile)
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read account certificate: %w", err)
	}
	cert, err := crypto.ParseCertificatePEM(string(certPEM))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse account certificate: %w", err)
	}

	return privKey, pubKey, cert, nil
}

// GetAccountPrivateKeyPEM returns the PEM-encoded private key for an account
func (s *Store) GetAccountPrivateKeyPEM(accountID string) (string, error) {
	privKeyPath := filepath.Join(s.baseDir, accountsDir, accountID, privKeyFile)
	data, err := os.ReadFile(privKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read account private key: %w", err)
	}
	return string(data), nil
}

// GetAccountCertificatePEM returns the PEM-encoded certificate for an account
func (s *Store) GetAccountCertificatePEM(accountID string) (string, error) {
	certPath := filepath.Join(s.baseDir, accountsDir, accountID, certFile)
	data, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read account certificate: %w", err)
	}
	return string(data), nil
}
