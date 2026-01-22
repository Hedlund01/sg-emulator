package grpc

import (
	"context"

	"sg-emulator/internal/server"
)

// Transport implements the gRPC transport for VirtualApps.
// This is currently a stub that will be implemented in the future.
type Transport struct {
	address string
	client  *server.Client
}

// New creates a new gRPC transport with the given address and client
func New(address string, client *server.Client) *Transport {
	return &Transport{
		address: address,
		client:  client,
	}
}

// Start begins listening for gRPC requests.
// Currently a stub that waits for context cancellation.
// Future: Start gRPC server here.
func (t *Transport) Start(ctx context.Context) error {
	// TODO: Implement gRPC server
	// Example future implementation:
	// lis, err := net.Listen("tcp", t.address)
	// if err != nil { return err }
	// grpcServer := grpc.NewServer()
	// pb.RegisterScalegraphServer(grpcServer, &grpcHandler{client: t.client})
	// go grpcServer.Serve(lis)

	<-ctx.Done()
	return nil
}

// Stop gracefully shuts down the gRPC transport
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
	return "grpc"
}
