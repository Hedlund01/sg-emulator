package server

import "context"

// Transport defines the interface for network transports (REST, gRPC, etc.)
type Transport interface {
	// Start begins listening. Called in a goroutine.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the transport
	Stop() error
	// Address returns the listening address
	Address() string
	// Type returns the transport type (e.g., "rest", "grpc")
	Type() string
}
