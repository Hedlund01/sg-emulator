package messages

import (
	"fmt"
	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

// CreateAccountWithKeysPayload contains parameters for CreateAccount with cryptographic keys
type CreateAccountWithKeysPayload struct {
	InitialBalance float64
	SignedRequest  *crypto.SignedEnvelope[*crypto.CreateAccountRequest]
}

func (p CreateAccountWithKeysPayload) GetSignedRequest() *crypto.SignedEnvelope[*crypto.CreateAccountRequest] {
	return p.SignedRequest
}

func (p CreateAccountWithKeysPayload) GetSignerID() scalegraph.ScalegraphId {
	// Return zero value — account creation must be signed by the CA,
	// which is verified separately in server.verifySignedRequest.
	return scalegraph.ScalegraphId{}
}

func (p CreateAccountWithKeysPayload) RequiresSignature() bool {
	return true
}

func (p CreateAccountWithKeysPayload) VerifyPayloadData() error {
	if p.SignedRequest == nil {
		// No signed request to verify against, fail if signature is required
		if p.RequiresSignature() {
			return fmt.Errorf("No signed request provided for CreateAccountWithKeys, but signature is required")
		}
		return nil
	}

	signedData := p.SignedRequest.Payload
	if signedData == nil {
		if p.RequiresSignature() {
			return fmt.Errorf("Signed request payload is nil for CreateAccountWithKeys, but signature is required")
		}
		return nil
	}

	// Verify InitialBalance matches
	if signedData.InitialBalance != p.InitialBalance {
		return fmt.Errorf("payload InitialBalance (%.2f) does not match signed InitialBalance (%.2f)", p.InitialBalance, signedData.InitialBalance)
	}

	return nil
}

// CreateAccountWithKeysResponse contains the result of CreateAccount with keys
type CreateAccountWithKeysResponse struct {
	Account     *scalegraph.Account
	Certificate string // PEM-encoded X.509 certificate
	PrivateKey  string // PEM-encoded Ed25519 private key
	PublicKey   string // PEM-encoded Ed25519 public key
}
