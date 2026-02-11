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

// Context returns the VirtualApp's context
func (v *VirtualApp) Context() context.Context {
	return v.ctx
}

// AddTransport adds a transport to the VirtualApp
func (v *VirtualApp) AddTransport(t Transport) {
	v.transports = append(v.transports, t)
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

// Start starts all transports
func (v *VirtualApp) Start() {
	for _, t := range v.transports {
		v.wg.Add(1)
		go func(transport Transport) {
			defer v.wg.Done()
			transport.Start(v.ctx)
		}(t)
	}
}

// Stop gracefully shuts down all transports
func (v *VirtualApp) Stop() {
	v.cancel()
	for _, t := range v.transports {
		t.Stop()
	}
	v.wg.Wait()
}
