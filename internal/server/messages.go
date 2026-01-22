package server

import (
	"context"

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
	Account *scalegraph.Account
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
}

// TransferResponse contains the result of Transfer (empty on success)
type TransferResponse struct{}

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
