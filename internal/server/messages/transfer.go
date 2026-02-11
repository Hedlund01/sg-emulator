package messages

import (
	"fmt"
	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

// TransferPayload contains parameters for Transfer
type TransferPayload struct {
	From   scalegraph.ScalegraphId
	To     scalegraph.ScalegraphId
	Amount float64
	Nonce  uint64
	// SignedRequest contains the signed transfer request.
	// This field is mandatory - transfers require valid cryptographic signatures.
	// The server must be configured with a CA for signature verification.
	SignedRequest *crypto.SignedEnvelope[*crypto.TransferRequest]
}

// GetSignedRequest returns the signed envelope for this transfer
func (p *TransferPayload) GetSignedRequest() *crypto.SignedEnvelope[*crypto.TransferRequest] {
	return p.SignedRequest
}

// GetSignerID returns the account ID that should sign this transfer (the sender)
func (p *TransferPayload) GetSignerID() scalegraph.ScalegraphId {
	return p.From
}

// RequiresSignature returns true because transfers require valid signatures
func (p *TransferPayload) RequiresSignature() bool {
	return true
}

// VerifyPayloadData verifies that the transfer payload data matches the signed data
func (p *TransferPayload) VerifyPayloadData() error {
	if p.SignedRequest == nil {
		// No signed request to verify against
		return nil
	}

	signedData := p.SignedRequest.Payload
	if signedData == nil {
		return fmt.Errorf("signed request payload is nil")
	}

	// Verify From account matches
	if signedData.From != p.From.String() {
		return fmt.Errorf("payload From (%s) does not match signed From (%s)", p.From, signedData.From)
	}

	// Verify To account matches
	if signedData.To != p.To.String() {
		return fmt.Errorf("payload To (%s) does not match signed To (%s)", p.To, signedData.To)
	}

	// Verify Amount matches
	if signedData.Amount != p.Amount {
		return fmt.Errorf("payload Amount (%.2f) does not match signed Amount (%.2f)", p.Amount, signedData.Amount)
	}

	// Verify Nonce matches
	if signedData.Nonce != p.Nonce {
		return fmt.Errorf("payload Nonce (%d) does not match signed Nonce (%d)", p.Nonce, signedData.Nonce)
	}

	return nil
}

// TransferResponse contains the result of Transfer (empty on success)
type TransferResponse struct{}
