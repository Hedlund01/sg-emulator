package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

// SignatureVerifier is implemented by types that can verify signed envelopes.
// It is passed to Verifiable.Verify() and VerifyRequest to avoid an import cycle
// between internal/crypto and internal/verifier.
type SignatureVerifier interface {
	VerifyEnvelopeData(signerID string, data SignableData, sig *Signature) error
}

// Verifiable is implemented by request types that require signature verification.
// The server auto-calls Verify() before dispatching to the handler.
type Verifiable interface {
	RequiresSignature() bool
	Verify(verifier SignatureVerifier, caPublicKey ed25519.PublicKey) error
}

// VerifyRequest is a generic helper that handles the common verification pattern.
// Individual request types call this from their Verify() method.
func VerifyRequest[T SignableData](
	verifier SignatureVerifier,
	caPublicKey ed25519.PublicKey,
	envelope *SignedEnvelope[T],
	expectedSignerID string, // empty = must be signed by CA
	verifyData func(signed T) error,
) error {
	if envelope == nil {
		return ErrMissingSignature
	}

	// 1. Verify envelope (cert chain, crypto signature, timestamp)
	if err := verifier.VerifyEnvelopeData(envelope.Signature.SignerID, envelope.Payload, &envelope.Signature); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	// 2. Verify signer matches expected account (or CA)
	if expectedSignerID == "" {
		id := DeriveAccountID(caPublicKey)
		caID := hex.EncodeToString(id[:])
		if envelope.Signature.SignerID != caID {
			return fmt.Errorf("%w: expected CA %s, got %s", ErrSignerMismatch, caID, envelope.Signature.SignerID)
		}
	} else if envelope.Signature.SignerID != expectedSignerID {
		return fmt.Errorf("%w: expected %s, got %s", ErrSignerMismatch, expectedSignerID, envelope.Signature.SignerID)
	}

	// 3. Verify payload data matches signed data
	if verifyData != nil {
		if err := verifyData(envelope.Payload); err != nil {
			return fmt.Errorf("%w: %v", ErrPayloadMismatch, err)
		}
	}

	return nil
}
