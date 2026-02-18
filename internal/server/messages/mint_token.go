package messages

import (
	"fmt"

	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

type MintTokenPayload struct {
	TokenValue      string                                           `json:"token_value"`
	ClawbackAddress *string                                          `json:"clawback_address"`
	SignedRequest   *crypto.SignedEnvelope[*crypto.MintTokenRequest] `json:"signed_request"`
}

// GetSignedRequest returns the signed envelope for this mint token request
func (p *MintTokenPayload) GetSignedRequest() *crypto.SignedEnvelope[*crypto.MintTokenRequest] {
	return p.SignedRequest
}

// GetSignerID returns zero value to indicate the CA must sign this request
func (p *MintTokenPayload) GetSignerID() scalegraph.ScalegraphId {
	return p.GetSignedRequest().Signature.SignerID
}

// RequiresSignature returns true because minting tokens requires CA authorization
func (p *MintTokenPayload) RequiresSignature() bool {
	return true
}

// VerifyPayloadData verifies that the mint token payload data matches the signed data
func (p *MintTokenPayload) VerifyPayloadData() error {
	if p.SignedRequest == nil {
		return nil
	}

	signedData := p.SignedRequest.Payload
	if signedData == nil {
		return fmt.Errorf("signed request payload is nil")
	}

	if signedData.TokenValue != p.TokenValue {
		return fmt.Errorf("payload TokenValue (%s) does not match signed TokenValue (%s)", p.TokenValue, signedData.TokenValue)
	}

	switch {
	case p.ClawbackAddress == nil && signedData.ClawbackAddress != nil:
		return fmt.Errorf("payload ClawbackAddress is nil but signed ClawbackAddress is %s", *signedData.ClawbackAddress)
	case p.ClawbackAddress != nil && signedData.ClawbackAddress == nil:
		return fmt.Errorf("payload ClawbackAddress is %s but signed ClawbackAddress is nil", *p.ClawbackAddress)
	case p.ClawbackAddress != nil && signedData.ClawbackAddress != nil && *p.ClawbackAddress != *signedData.ClawbackAddress:
		return fmt.Errorf("payload ClawbackAddress (%s) does not match signed ClawbackAddress (%s)", *p.ClawbackAddress, *signedData.ClawbackAddress)
	}

	return nil
}

type MintTokenResponse struct{}
