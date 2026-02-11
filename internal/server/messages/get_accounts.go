package messages

import "sg-emulator/internal/scalegraph"

// GetAccountsPayload contains parameters for GetAccounts (empty)
type GetAccountsPayload struct{}

// GetAccountsResponse contains the result of GetAccounts
type GetAccountsResponse struct {
	Accounts []*scalegraph.Account
}
