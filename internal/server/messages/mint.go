package messages

import "sg-emulator/internal/scalegraph"

// MintPayload contains parameters for Mint
type MintPayload struct {
	To     scalegraph.ScalegraphId
	Amount float64
}

// MintResponse contains the result of Mint (empty on success)
type MintResponse struct{}
