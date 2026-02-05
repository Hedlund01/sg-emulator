package crypto

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

const (
	// DefaultTimestampWindow is the default time window for signature freshness
	DefaultTimestampWindow = 5 * time.Minute
)

// Verifier verifies signatures and certificate chains
type Verifier struct {
	caCert          *x509.Certificate
	timestampWindow time.Duration
}

// NewVerifier creates a new Verifier with the given CA certificate
func NewVerifier(caCert *x509.Certificate) *Verifier {
	return &Verifier{
		caCert:          caCert,
		timestampWindow: DefaultTimestampWindow,
	}
}

// SetTimestampWindow sets the time window for signature freshness checks
func (v *Verifier) SetTimestampWindow(d time.Duration) {
	v.timestampWindow = d
}

// VerifyCertificate verifies that a certificate was signed by the CA
func (v *Verifier) VerifyCertificate(cert *x509.Certificate) error {
	if cert == nil {
		return fmt.Errorf("certificate is nil")
	}

	// Create a certificate pool with the CA certificate
	roots := x509.NewCertPool()
	roots.AddCert(v.caCert)

	// Verify the certificate chain
	opts := x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: time.Now(),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	if _, err := cert.Verify(opts); err != nil {
		return fmt.Errorf("certificate verification failed: %w", err)
	}

	return nil
}

// VerifySignature verifies that a signature is valid for the given data and public key
func (v *Verifier) VerifySignature(data SignableData, sig *Signature, pubKey ed25519.PublicKey) error {
	if sig == nil {
		return fmt.Errorf("signature is nil")
	}

	if sig.Algorithm != "Ed25519" {
		return fmt.Errorf("unsupported signature algorithm: %s", sig.Algorithm)
	}

	if len(sig.Value) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature size: got %d, want %d", len(sig.Value), ed25519.SignatureSize)
	}

	// Verify signature
	if !ed25519.Verify(pubKey, data.Bytes(), sig.Value) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// VerifyTimestamp verifies that the signature timestamp is within the allowed window
func (v *Verifier) VerifyTimestamp(sig *Signature) error {
	if sig == nil {
		return fmt.Errorf("signature is nil")
	}

	sigTime := time.Unix(sig.Timestamp, 0)
	now := time.Now()

	// Check if timestamp is in the future
	if sigTime.After(now.Add(time.Minute)) {
		return fmt.Errorf("signature timestamp is in the future")
	}

	// Check if timestamp is too old
	if now.Sub(sigTime) > v.timestampWindow {
		return fmt.Errorf("signature timestamp is too old (older than %v)", v.timestampWindow)
	}

	return nil
}

// VerifyAccountID verifies that the signer ID matches the derived account ID from the public key
func (v *Verifier) VerifyAccountID(sig *Signature, pubKey ed25519.PublicKey) error {
	derivedID := DeriveAccountID(pubKey)
	derivedIDHex := fmt.Sprintf("%x", derivedID)

	if sig.SignerID != derivedIDHex {
		return fmt.Errorf("signer ID mismatch: signature says %s, but public key derives to %s", sig.SignerID, derivedIDHex)
	}

	return nil
}

// VerifyEnvelope performs complete verification of a signed envelope:
// 1. Parse and verify the certificate chain
// 2. Verify the derived account ID matches the signer ID
// 3. Verify the signature on the payload
// 4. Verify the timestamp freshness
func VerifyEnvelope[T SignableData](v *Verifier, envelope *SignedEnvelope[T]) (ed25519.PublicKey, error) {
	// Parse certificate from PEM
	cert, err := ParseCertificatePEM(envelope.Certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Verify certificate chain
	if err := v.VerifyCertificate(cert); err != nil {
		return nil, fmt.Errorf("certificate chain verification failed: %w", err)
	}

	// Extract public key from certificate
	pubKey, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("certificate public key is not Ed25519")
	}

	// Verify account ID matches
	if err := v.VerifyAccountID(&envelope.Signature, pubKey); err != nil {
		return nil, err
	}

	// Verify signature
	if err := v.VerifySignature(envelope.Payload, &envelope.Signature, pubKey); err != nil {
		return nil, err
	}

	// Verify timestamp
	if err := v.VerifyTimestamp(&envelope.Signature); err != nil {
		return nil, err
	}

	return pubKey, nil
}

// ParseCertificatePEM parses a PEM-encoded X.509 certificate
func ParseCertificatePEM(certPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("unexpected PEM block type: %s", block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// EncodeCertificatePEM encodes an X.509 certificate to PEM format
func EncodeCertificatePEM(cert *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}))
}
