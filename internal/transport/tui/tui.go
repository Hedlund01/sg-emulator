package tui

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"sg-emulator/internal/server"
	tuiview "sg-emulator/internal/tui"
)

// Transport implements the TUI transport for VirtualApps.
type Transport struct {
	client *server.Client
	server *server.Server
}

// New creates a new TUI transport with the given client and server
func New(client *server.Client, srv *server.Server) *Transport {
	return &Transport{
		client: client,
		server: srv,
	}
}

// Start begins the TUI interface.
func (t *Transport) Start(ctx context.Context) error {
	p := tea.NewProgram(tuiview.NewModel(t.client, t.server), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		return err
	}

	return nil
}

// Stop gracefully shuts down the TUI transport
func (t *Transport) Stop() error {
	return nil
}

// Address returns the transport address (TUI has no network address)
func (t *Transport) Address() string {
	return "local"
}

// Type returns the transport type
func (t *Transport) Type() string {
	return "tui"
}
