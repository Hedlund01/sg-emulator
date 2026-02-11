package messages

import (
	"fmt"
	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

// GetAccountPayload contains parameters for GetAccount
type GetAccountPayload struct {
	AccountID     scalegraph.ScalegraphId
	SignedRequest *crypto.SignedEnvelope[*crypto.GetAccountRequest]
}

// GetAccountResponse contains the result of GetAccount
type GetAccountResponse struct {
	Account *scalegraph.Account
}

func (p *GetAccountPayload) GetSignedRequest() *crypto.SignedEnvelope[*crypto.GetAccountRequest] {
	return p.SignedRequest
}

func (p *GetAccountPayload) GetSignerID() scalegraph.ScalegraphId {
	return p.AccountID
}

func (p *GetAccountPayload) RequiresSignature() bool {
	return true
}

func (p *GetAccountPayload) VerifyPayloadData() error {
	if p.SignedRequest == nil {
		// No signed request to verify against, fail if signature is required
		if p.RequiresSignature() {
			return fmt.Errorf("No signed request provided for GetAccount, but signature is required")
		}
		return nil
	}

	signedData := p.SignedRequest.Payload
	if signedData == nil {
		if p.RequiresSignature() {
			return fmt.Errorf("Signed request payload is nil for GetAccount, but signature is required")
		}
		return nil
	}

	// Verify ID matches
	if signedData.AccountID != p.AccountID.String() {
		return fmt.Errorf("payload ID (%s) does not match signed ID (%s)", p.AccountID, signedData.AccountID)
	}

	return nil
}
