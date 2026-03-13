package server

import (
	"context"

	eventv1 "sg-emulator/gen/event/v1"
)

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

// EventDelivery carries a filtered event to a transport along with the
// account ID it was matched for, so the transport can route without re-filtering.
type EventDelivery struct {
	Event     *eventv1.Event
	AccountID string
}

// EventTransport is an optional interface that transports can implement
// to receive events from the EventBus. Transports that do not implement
// this interface simply never receive events.
type EventTransport interface {
	Transport
	// EventChannel returns the channel the EventBus should send filtered
	// EventDelivery messages to. The transport owns the channel.
	EventChannel() chan<- EventDelivery
}
