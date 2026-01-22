package rest

import (
	"context"
	"log/slog"

	"sg-emulator/internal/server"
)

// Transport implements the REST transport for VirtualApps.
// This is currently a stub that will be implemented in the future.
type Transport struct {
	address string
	client  *server.Client
	logger  *slog.Logger
}

// New creates a new REST transport with the given address and client
func New(address string, client *server.Client, logger *slog.Logger) *Transport {
	logger.Info("REST transport created", "address", address)
	return &Transport{
		address: address,
		client:  client,
		logger:  logger,
	}
}

// Start begins listening for REST requests.
// Currently a stub that waits for context cancellation.
// Future: Start HTTP server here.
func (t *Transport) Start(ctx context.Context) error {
	// TODO: Implement HTTP server
	// Example future implementation:
	// router := mux.NewRouter()
	// router.HandleFunc("/accounts", t.handleGetAccounts).Methods("GET")
	// router.HandleFunc("/accounts", t.handleCreateAccount).Methods("POST")
	// router.HandleFunc("/transfer", t.handleTransfer).Methods("POST")
	// server := &http.Server{Addr: t.address, Handler: router}
	// go server.ListenAndServe()

	<-ctx.Done()
	return nil
}

// Stop gracefully shuts down the REST transport
func (t *Transport) Stop() error {
	// TODO: Implement graceful shutdown
	return nil
}

// Address returns the listening address
func (t *Transport) Address() string {
	return t.address
}

// Type returns the transport type
func (t *Transport) Type() string {
	return "rest"
}
