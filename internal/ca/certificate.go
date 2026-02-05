package ca

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

const (
	// CACertValidityYears is the validity period for CA certificates
	CACertValidityYears = 10
	// AccountCertValidityYears is the validity period for account certificates
	AccountCertValidityYears = 1
)

// CreateCACertificate creates a self-signed CA certificate
func CreateCACertificate(pubKey ed25519.PublicKey, privKey ed25519.PrivateKey) (*x509.Certificate, error) {
	serialNumber, err := generateSerialNumber()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{"Scalegraph Emulator"},
			OrganizationalUnit: []string{"Certificate Authority"},
			CommonName:         "Scalegraph CA",
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(CACertValidityYears, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		MaxPathLen:            1,
		MaxPathLenZero:        false,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created CA certificate: %w", err)
	}

	return cert, nil
}

// CreateAccountCertificate creates a certificate for an account signed by the CA
func CreateAccountCertificate(accountID string, pubKey ed25519.PublicKey, caCert *x509.Certificate, caPrivKey ed25519.PrivateKey) (*x509.Certificate, error) {
	serialNumber, err := generateSerialNumber()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{"Scalegraph Emulator"},
			OrganizationalUnit: []string{"Account"},
			CommonName:         accountID,
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(AccountCertValidityYears, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, pubKey, caPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create account certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created account certificate: %w", err)
	}

	return cert, nil
}

// generateSerialNumber generates a random serial number for certificates
func generateSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}
	return serialNumber, nil
}
