package server

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"

	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

// RequestType identifies the type of operation to perform
type RequestType int

const (
	ReqCreateAccount RequestType = iota
	ReqGetAccount
	ReqGetAccounts
	ReqTransfer
	ReqMint
	ReqAccountCount
)

// Request is sent from clients to the Server
type Request struct {
	ID           string
	Type         RequestType
	ResponseChan chan<- Response
	Payload      any
	Context      context.Context
}

// Response is sent from Server back to clients
type Response struct {
	ID      string
	Success bool
	Error   string
	Payload any
}

// CreateAccountPayload contains parameters for CreateAccount
type CreateAccountPayload struct {
	InitialBalance float64
}

// CreateAccountResponse contains the result of CreateAccount
type CreateAccountResponse struct {
	Account     *scalegraph.Account
	Certificate string // PEM-encoded X.509 certificate
	PrivateKey  string // PEM-encoded Ed25519 private key
}

// GetAccountPayload contains parameters for GetAccount
type GetAccountPayload struct {
	ID scalegraph.ScalegraphId
}

// GetAccountResponse contains the result of GetAccount
type GetAccountResponse struct {
	Account *scalegraph.Account
}

// GetAccountsPayload contains parameters for GetAccounts (empty)
type GetAccountsPayload struct{}

// GetAccountsResponse contains the result of GetAccounts
type GetAccountsResponse struct {
	Accounts []*scalegraph.Account
}

// TransferPayload contains parameters for Transfer
type TransferPayload struct {
	From   scalegraph.ScalegraphId
	To     scalegraph.ScalegraphId
	Amount float64
	// SignedRequest contains the signed transfer request (optional for backwards compatibility)
	SignedRequest *crypto.SignedEnvelope[*crypto.TransferRequest]
}

// TransferResponse contains the result of Transfer (empty on success)
type TransferResponse struct{}

// SignedTransferRequest represents a signed transfer request for verification
type SignedTransferRequest struct {
	From      scalegraph.ScalegraphId
	To        scalegraph.ScalegraphId
	Amount    float64
	Nonce     string
	Timestamp int64
}

// CreateAccountWithKeysPayload contains parameters for CreateAccount with cryptographic keys
type CreateAccountWithKeysPayload struct {
	InitialBalance float64
	PublicKey      ed25519.PublicKey
	Certificate    *x509.Certificate
}

// CreateAccountWithKeysResponse contains the result of CreateAccount with keys
type CreateAccountWithKeysResponse struct {
	Account     *scalegraph.Account
	Certificate string // PEM-encoded X.509 certificate
	PrivateKey  string // PEM-encoded Ed25519 private key
}

// MintPayload contains parameters for Mint
type MintPayload struct {
	To     scalegraph.ScalegraphId
	Amount float64
}

// MintResponse contains the result of Mint (empty on success)
type MintResponse struct{}

// AccountCountPayload contains parameters for AccountCount (empty)
type AccountCountPayload struct{}

// AccountCountResponse contains the result of AccountCount
type AccountCountResponse struct {
	Count int
}
