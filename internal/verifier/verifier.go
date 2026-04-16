package verifier

import (
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"sg-emulator/internal/scalegraph"
	"time"
	. "sg-emulator/internal/crypto"
)

const (
	// DefaultTimestampWindow is the default time window for signature freshness
	DefaultTimestampWindow = 5 * time.Minute
)

// Verifier verifies signatures and certificate chains
type Verifier struct {
	caCert          *x509.Certificate
	timestampWindow time.Duration
	scalegraphApp   *scalegraph.App
}

// NewVerifier creates a new Verifier with the given CA certificate
func NewVerifier(caCert *x509.Certificate, scalegraphApp *scalegraph.App) *Verifier {
	return &Verifier{
		caCert:          caCert,
		timestampWindow: DefaultTimestampWindow,
		scalegraphApp:   scalegraphApp,
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

// VerifyEnvelopeData implements crypto.SignatureVerifier. It resolves the signer's
// certificate and public key (using the CA cert for CA-signed requests, or the
// scalegraph app for account-signed requests), then verifies the cert chain,
// account ID, signature, and timestamp.
func (v *Verifier) VerifyEnvelopeData(signerID string, data SignableData, sig *Signature) error {
	cert, pubKey, err := v.resolveSigner(signerID)
	if err != nil {
		return err
	}

	if err := v.VerifyCertificate(cert); err != nil {
		return fmt.Errorf("certificate chain verification failed: %w", err)
	}

	if err := v.VerifyAccountID(sig, pubKey); err != nil {
		return err
	}

	if err := v.VerifySignature(data, sig, pubKey); err != nil {
		return err
	}

	return v.VerifyTimestamp(sig)
}

// resolveSigner returns the certificate and public key for the given signer ID.
// If the signer ID matches the CA, the CA certificate is returned directly.
// Otherwise the scalegraph app is queried for an account certificate.
func (v *Verifier) resolveSigner(signerID string) (*x509.Certificate, ed25519.PublicKey, error) {
	// Check if this is the CA's own signer ID
	if caPubKey, ok := v.caCert.PublicKey.(ed25519.PublicKey); ok {
		caIDBytes := DeriveAccountID(caPubKey)
		caID := fmt.Sprintf("%x", caIDBytes)
		if signerID == caID {
			return v.caCert, caPubKey, nil
		}
	}

	// Account-signed: look up from scalegraph app
	id, err := scalegraph.ScalegraphIdFromString(signerID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid signer ID: %w", err)
	}

	cert, _, err := v.scalegraphApp.GetAccountCertAndPublicKey(id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get account certificate and public key: %w", err)
	}

	pubKey, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("certificate public key is not Ed25519")
	}

	return cert, pubKey, nil
}

// VerifyEnvelope performs complete verification of a signed envelope:
// 1. Parse and verify the certificate chain
// 2. Verify the derived account ID matches the signer ID
// 3. Verify the signature on the payload
// 4. Verify the timestamp freshness
func VerifyEnvelope[T SignableData](v *Verifier, envelope *SignedEnvelope[T]) (ed25519.PublicKey, error) {
	if err := v.VerifyEnvelopeData(envelope.Signature.SignerID, envelope.Payload, &envelope.Signature); err != nil {
		return nil, err
	}

	_, pubKey, err := v.resolveSigner(envelope.Signature.SignerID)
	if err != nil {
		return nil, err
	}
	return pubKey, nil
}

