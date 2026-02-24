package messages

import "context"

// Request is sent from clients to the Server.
// The Payload field carries a typed request struct (e.g. *scalegraph.TransferRequest).
type Request struct {
	ID           string
	ResponseChan chan<- Response
	Payload      any
	Context      context.Context
}

// Response is sent from Server back to clients.
// Payload carries a typed response struct (e.g. *scalegraph.TransferResponse).
// Error is nil on success.
type Response struct {
	ID      string
	Payload any
	Error   error
}
