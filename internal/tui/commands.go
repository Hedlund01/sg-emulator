package tui

import (
	tea "github.com/charmbracelet/bubbletea"
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
