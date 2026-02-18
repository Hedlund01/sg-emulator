package messages

import (
	"context"

	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

// Package server/messages defines the message types used for communication between
// clients and the Server through a request-response pattern.
//
// # Message Flow
//
// The communication follows this pattern:
//
//  1. Client creates a Request with a specific Type and Payload
//  2. Client sends Request through the server's request channel
//  3. Server processes the Request and generates a Response
//  4. Server sends Response back through the Request's ResponseChan
//  5. Client receives and processes the Response
//
// # Request
//
// A Request represents an operation that a client wants the server to perform.
// Each Request contains:
//   - ID: Unique identifier for tracking and correlation
//   - Type: The operation to perform (e.g., ReqTransfer, ReqGetAccount)
//   - ResponseChan: Channel where the server will send the Response
//   - Payload: Operation-specific data (must match the Type)
//   - Context: For tracing, cancellation, and deadlines
//
// Example:
//
//	req := Request{
//	    ID:           "req-123",
//	    Type:         ReqTransfer,
//	    ResponseChan: responseChan,
//	    Payload:      TransferPayload{From: fromID, To: toID, Amount: 100},
//	    Context:      ctx,
//	}
//
// # Response
//
// A Response contains the result of processing a Request. Each Response includes:
//   - ID: Matches the Request ID for correlation
//   - Success: True if the operation succeeded, false if it failed
//   - Error: Error message if Success is false (empty string otherwise)
//   - Payload: Operation-specific result data (type depends on request Type)
//
// Example success:
//
//	resp := Response{
//	    ID:      "req-123",
//	    Success: true,
//	    Payload: TransferResponse{},
//	}
//
// Example failure:
//
//	resp := Response{
//	    ID:      "req-123",
//	    Success: false,
//	    Error:   "insufficient funds",
//	}
//
// # Payload
//
// Payloads are strongly-typed data structures that carry operation-specific parameters
// (for Requests) or results (for Responses). Each RequestType has a corresponding
// payload type:
//
//   - ReqCreateAccount -> CreateAccountPayload / CreateAccountResponse
//   - ReqGetAccount    -> GetAccountPayload / GetAccountResponse
//   - ReqTransfer      -> TransferPayload / TransferResponse
//   - ReqMint          -> MintPayload / MintResponse
//
// Some operations support signature verification through the SignablePayload interface,
// which requires cryptographic signatures for authentication and authorization.

// RequestType identifies the type of operation to perform
type RequestType int

const (
	ReqCreateAccount RequestType = iota
	ReqGetAccount
	ReqGetAccounts
	ReqTransfer
	ReqMint
	ReqAccountCount
	ReqMintToken
	ReqTransferToken
	ReqAuthorizeTokenTransfer
)

// Request is sent from clients to the Server.
// It represents an operation request with a unique ID, operation type,
// response channel, payload data, and context for tracing/cancellation.
type Request struct {
	ID           string
	Type         RequestType
	ResponseChan chan<- Response
	Payload      any
	Context      context.Context
}

// Response is sent from Server back to clients.
// It contains the result of processing a Request, including success status,
// error message (if failed), and operation-specific result payload.
type Response struct {
	ID      string
	Success bool
	Error   string
	Payload any
}

// SignablePayload is an interface for payloads that support cryptographic signature verification.
// It uses Go generics to support different SignableData types while maintaining type safety.
type SignablePayload[T crypto.SignableData] interface {
	// GetSignedRequest returns the signed envelope containing the request, or nil if not signed
	GetSignedRequest() *crypto.SignedEnvelope[T]
	// GetSignerID returns the account ID that is expected to have signed the request
	GetSignerID() scalegraph.ScalegraphId
	// RequiresSignature returns true if a valid signature is mandatory for this operation
	RequiresSignature() bool
	// VerifyPayloadData verifies that the payload data matches the signed data
	VerifyPayloadData() error
}
