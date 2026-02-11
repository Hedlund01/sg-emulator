package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
)

// Custom message types for async operations
type (
	// ErrMsg represents an error message
	ErrMsg struct{ Err error }

	// StatusMsg represents a status update message
	StatusMsg string
)

// Error implements the error interface for ErrMsg
func (e ErrMsg) Error() string {
	return e.Err.Error()
}

// Example command that returns a status message
func doSomethingAsync() tea.Cmd {
	return func() tea.Msg {
		// Perform async operation here
		return StatusMsg("Operation completed")
	}
}

// createAccountWithCA creates a new account using the server's CA to sign the request.
// This replaces the old direct CreateAccount call that was removed in the x509-refactor.
func (m Model) createAccountWithCA(ctx context.Context, balance float64) (*scalegraph.Account, error) {
	ca := m.server.CA()
	if ca == nil {
		return nil, fmt.Errorf("no CA available on server")
	}

	// Use the CA's own private key and certificate directly for account creation
	systemAccountID := scalegraph.ScalegraphIdFromPublicKey(ca.PublicKey())
	accountIDStr := systemAccountID.String()

	createReq := &crypto.CreateAccountRequest{
		InitialBalance: balance,
	}

	signedReq, err := crypto.CreateSignedEnvelope(createReq, ca.PrivateKey(), accountIDStr, ca.CertificatePEM())
	if err != nil {
		return nil, fmt.Errorf("failed to create signed request: %v", err)
	}

	resp, err := m.app.CreateAccountWithCredentials(ctx, balance, signedReq)
	if err != nil {
		return nil, err
	}

	return resp.Account, nil
}
