package server

import (
	"context"
	"log/slog"
	"sync"

	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server/messages"
)

// VirtualApp represents a virtual application instance with its own ID and transports.
// Each VirtualApp can have multiple transports (REST, gRPC, etc.) running simultaneously.
type VirtualApp struct {
	id         scalegraph.ScalegraphId
	client     *Client
	transports []Transport
	eventBus   *EventBus
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// newVirtualApp creates a new VirtualApp without any transports.
// This is internal - use Server.CreateVirtualApp() to create and register.
func newVirtualApp(requestChan chan<- messages.Request, logger *slog.Logger) (*VirtualApp, error) {
	id, err := scalegraph.NewScalegraphId()
	if err != nil {
		return nil, err
	}

	logger.Debug("Creating virtual app", "id", id)
	ctx, cancel := context.WithCancel(context.Background())
	return &VirtualApp{
		id:         id,
		client:     NewClient(requestChan, logger.With("vapp_id", id)),
		transports: make([]Transport, 0),
		eventBus:   NewEventBus(logger.With("vapp_id", id, "component", "event-bus")),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// ID returns the VirtualApp's unique identifier
func (v *VirtualApp) ID() scalegraph.ScalegraphId {
	return v.id
}

// Client returns the VirtualApp's client for making requests
func (v *VirtualApp) Client() *Client {
	return v.client
}

// EventBus returns the VirtualApp's event bus for pub/sub
func (v *VirtualApp) EventBus() *EventBus {
	return v.eventBus
}

// Context returns the VirtualApp's context
func (v *VirtualApp) Context() context.Context {
	return v.ctx
}

// AddTransport adds a transport to the VirtualApp.
// If the transport implements EventTransport, its event channel is
// registered with the EventBus so filtered events are delivered to it.
func (v *VirtualApp) AddTransport(t Transport) {
	v.transports = append(v.transports, t)
	if et, ok := t.(EventTransport); ok {
		v.eventBus.RegisterTransport(et.EventChannel())
	}
}

// Transports returns all transports
func (v *VirtualApp) Transports() []Transport {
	return v.transports
}

// Addresses returns a map of transport type to address
func (v *VirtualApp) Addresses() map[string]string {
	addresses := make(map[string]string)
	for _, t := range v.transports {
		addresses[t.Type()] = t.Address()
	}
	return addresses
}

// Start starts the EventBus dispatcher and all transports.
func (v *VirtualApp) Start() {
	go v.eventBus.Run(v.ctx)

	for _, t := range v.transports {
		v.wg.Add(1)
		go func(transport Transport) {
			defer v.wg.Done()
			transport.Start(v.ctx)
		}(t)
	}
}

// Stop gracefully shuts down the EventBus and all transports.
func (v *VirtualApp) Stop() {
	v.cancel()
	v.eventBus.Stop()
	for _, t := range v.transports {
		t.Stop()
	}
	v.wg.Wait()
}
