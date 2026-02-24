package scalegraph

import (
	"crypto/ed25519"
	"fmt"

	"sg-emulator/internal/crypto"
)

// CreateAccountRequest is the request to create a new account.
// Signed by the CA.
type CreateAccountRequest struct {
	InitialBalance float64
	SignedEnvelope *crypto.SignedEnvelope[*crypto.CreateAccountPayload]
}

// CreateAccountResponse is the response from creating a new account.
type CreateAccountResponse struct {
	Account     *Account
	Certificate string // PEM-encoded X.509 certificate
	PrivateKey  string // PEM-encoded Ed25519 private key
	PublicKey   string // PEM-encoded Ed25519 public key
}

func (r *CreateAccountRequest) RequiresSignature() bool { return true }

func (r *CreateAccountRequest) Verify(verifier *crypto.Verifier, caPublicKey ed25519.PublicKey) error {
	return crypto.VerifyRequest(verifier, caPublicKey, r.SignedEnvelope, "",
		func(signed *crypto.CreateAccountPayload) error {
			if signed.InitialBalance != r.InitialBalance {
				return fmt.Errorf("InitialBalance mismatch")
			}
			return nil
		})
}

// GetAccountRequest is the request to get account details.
// Signed by the account owner.
type GetAccountRequest struct {
	AccountID      ScalegraphId
	SignedEnvelope *crypto.SignedEnvelope[*crypto.GetAccountPayload]
}

// GetAccountResponse is the response from getting account details.
type GetAccountResponse struct {
	Account *Account
}

func (r *GetAccountRequest) RequiresSignature() bool { return true }

func (r *GetAccountRequest) Verify(verifier *crypto.Verifier, caPublicKey ed25519.PublicKey) error {
	return crypto.VerifyRequest(verifier, caPublicKey, r.SignedEnvelope, r.AccountID.String(),
		func(signed *crypto.GetAccountPayload) error {
			if signed.AccountID != r.AccountID.String() {
				return fmt.Errorf("AccountID mismatch")
			}
			return nil
		})
}

// GetAccountsRequest is the request to list all accounts.
// Not signed.
type GetAccountsRequest struct{}

// GetAccountsResponse is the response from listing all accounts.
type GetAccountsResponse struct {
	Accounts []*Account
}

// TransferRequest is the request to transfer funds.
// Signed by the sender.
type TransferRequest struct {
	From           ScalegraphId
	To             ScalegraphId
	Amount         float64
	Nonce          uint64
	SignedEnvelope *crypto.SignedEnvelope[*crypto.TransferPayload]
}

// TransferResponse is the response from a transfer.
type TransferResponse struct{}

func (r *TransferRequest) RequiresSignature() bool { return true }

func (r *TransferRequest) Verify(verifier *crypto.Verifier, caPublicKey ed25519.PublicKey) error {
	return crypto.VerifyRequest(verifier, caPublicKey, r.SignedEnvelope, r.From.String(),
		func(signed *crypto.TransferPayload) error {
			if signed.From != r.From.String() {
				return fmt.Errorf("From mismatch")
			}
			if signed.To != r.To.String() {
				return fmt.Errorf("To mismatch")
			}
			if signed.Amount != r.Amount {
				return fmt.Errorf("Amount mismatch")
			}
			if signed.Nonce != r.Nonce {
				return fmt.Errorf("Nonce mismatch")
			}
			return nil
		})
}

// MintRequest is the request to mint funds into an account.
// Not signed (server-side only operation).
type MintRequest struct {
	To     ScalegraphId
	Amount float64
}

// MintResponse is the response from minting.
type MintResponse struct{}

// MintTokenRequest is the request to mint a token.
// Signed by the account owner.
type MintTokenRequest struct {
	TokenValue      string
	ClawbackAddress *ScalegraphId
	SignedEnvelope  *crypto.SignedEnvelope[*crypto.MintTokenPayload]
}

// MintTokenResponse is the response from minting a token.
type MintTokenResponse struct {
	TokenID string
}

func (r *MintTokenRequest) RequiresSignature() bool { return true }

func (r *MintTokenRequest) Verify(verifier *crypto.Verifier, caPublicKey ed25519.PublicKey) error {
	// Signer ID comes from the signed envelope
	signerID := r.SignedEnvelope.Signature.SignerID
	return crypto.VerifyRequest(verifier, caPublicKey, r.SignedEnvelope, signerID,
		func(signed *crypto.MintTokenPayload) error {
			if signed.TokenValue != r.TokenValue {
				return fmt.Errorf("TokenValue mismatch")
			}
			// Compare clawback addresses
			switch {
			case r.ClawbackAddress == nil && signed.ClawbackAddress != nil:
				return fmt.Errorf("ClawbackAddress mismatch: request nil, signed %s", *signed.ClawbackAddress)
			case r.ClawbackAddress != nil && signed.ClawbackAddress == nil:
				return fmt.Errorf("ClawbackAddress mismatch: request %s, signed nil", r.ClawbackAddress)
			case r.ClawbackAddress != nil && signed.ClawbackAddress != nil && r.ClawbackAddress.String() != *signed.ClawbackAddress:
				return fmt.Errorf("ClawbackAddress mismatch")
			}
			return nil
		})
}

type TransferTokenRequest struct {
	From           ScalegraphId
	To             ScalegraphId
	TokenId        string
	SignedEnvelope *crypto.SignedEnvelope[*crypto.TransferTokenPayload]
}

func (r *TransferTokenRequest) RequiresSignature() bool { return true }

func (r *TransferTokenRequest) Verify(verifier *crypto.Verifier, caPublicKey ed25519.PublicKey) error {
	return crypto.VerifyRequest(verifier, caPublicKey, r.SignedEnvelope, r.From.String(),
		func(signed *crypto.TransferTokenPayload) error {
			if signed.TokenID != r.TokenId {
				return fmt.Errorf("TokenId mismatch")
			}
			if signed.From != r.From.String() {
				return fmt.Errorf("From mismatch")
			}
			if signed.To != r.To.String() {
				return fmt.Errorf("To mismatch")
			}
			return nil
		})
}

type TransferTokenResponse struct{}

type AuthorizeTokenTransferRequest struct {
	AccountID      ScalegraphId
	TokenId        string
	SignedEnvelope *crypto.SignedEnvelope[*crypto.AuthorizeTokenTransferPayload]
}

func (r *AuthorizeTokenTransferRequest) RequiresSignature() bool { return true }

func (r *AuthorizeTokenTransferRequest) Verify(verifier *crypto.Verifier, caPublicKey ed25519.PublicKey) error {
	return crypto.VerifyRequest(verifier, caPublicKey, r.SignedEnvelope, r.AccountID.String(),
		func(signed *crypto.AuthorizeTokenTransferPayload) error {
			if signed.TokenID != r.TokenId {
				return fmt.Errorf("TokenId mismatch")
			}
			if signed.AccountID != r.AccountID.String() {
				return fmt.Errorf("From mismatch")
			}
			return nil
		})
}

type AuthorizeTokenTransferResponse struct{}

type UnauthorizeTokenTransferRequest struct {
	AccountID      ScalegraphId
	TokenId        string
	SignedEnvelope *crypto.SignedEnvelope[*crypto.UnauthorizeTokenTransferPayload]
}

func (r *UnauthorizeTokenTransferRequest) RequiresSignature() bool { return true }

func (r *UnauthorizeTokenTransferRequest) Verify(verifier *crypto.Verifier, caPublicKey ed25519.PublicKey) error {
	return crypto.VerifyRequest(verifier, caPublicKey, r.SignedEnvelope, r.AccountID.String(),
		func(signed *crypto.UnauthorizeTokenTransferPayload) error {
			if signed.TokenID != r.TokenId {
				return fmt.Errorf("TokenId mismatch")
			}
			if signed.AccountID != r.AccountID.String() {
				return fmt.Errorf("From mismatch")
			}
			return nil
		})
}

type UnauthorizeTokenTransferResponse struct{}

// AccountCountRequest is the request to get the number of accounts.
// Not signed.
type AccountCountRequest struct{}

// AccountCountResponse is the response from getting the account count.
type AccountCountResponse struct {
	Count int
}
