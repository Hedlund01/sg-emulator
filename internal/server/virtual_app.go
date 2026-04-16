package server

import (
	"context"
	"log/slog"
	"sync"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"

	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server/messages"
)

// slogWatermillLogger adapts a *slog.Logger to the watermill.LoggerAdapter interface
// so that watermill logs are routed through slog (and in TUI mode go to the log file).
type slogWatermillLogger struct {
	logger *slog.Logger
}

func (l *slogWatermillLogger) fields(f watermill.LogFields) []any {
	attrs := make([]any, 0, len(f)*2)
	for k, v := range f {
		attrs = append(attrs, k, v)
	}
	return attrs
}

func (l *slogWatermillLogger) Error(msg string, err error, fields watermill.LogFields) {
	l.logger.Error(msg, append(l.fields(fields), "error", err)...)
}

func (l *slogWatermillLogger) Info(msg string, fields watermill.LogFields) {
	l.logger.Info(msg, l.fields(fields)...)
}

func (l *slogWatermillLogger) Debug(msg string, fields watermill.LogFields) {
	l.logger.Debug(msg, l.fields(fields)...)
}

func (l *slogWatermillLogger) Trace(msg string, fields watermill.LogFields) {
	l.logger.Debug(msg, l.fields(fields)...) // slog has no Trace; map to Debug
}

func (l *slogWatermillLogger) With(fields watermill.LogFields) watermill.LoggerAdapter {
	return &slogWatermillLogger{logger: l.logger.With(l.fields(fields)...)}
}

// VirtualApp represents a virtual application instance with its own ID and transports.
// Each VirtualApp can have multiple transports (REST, gRPC, etc.) running simultaneously.
type VirtualApp struct {
	id         scalegraph.ScalegraphId
	client     *Client
	transports []Transport
	goChannel  *gochannel.GoChannel
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

	gc := gochannel.NewGoChannel(
		gochannel.Config{
			OutputChannelBuffer:            256,
			Persistent:                     false,
			BlockPublishUntilSubscriberAck: false,
		},
		&slogWatermillLogger{logger: logger.With("component", "watermill")},
	)

	return &VirtualApp{
		id:         id,
		client:     NewClient(requestChan, logger.With("vapp_id", id)),
		transports: make([]Transport, 0),
		goChannel:  gc,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// ID returns the VirtualApp's unique identifier.
func (v *VirtualApp) ID() scalegraph.ScalegraphId {
	return v.id
}

// Client returns the VirtualApp's client for making requests.
func (v *VirtualApp) Client() *Client {
	return v.client
}

// Publisher returns the Watermill publisher for this VirtualApp.
func (v *VirtualApp) Publisher() message.Publisher {
	return v.goChannel
}

// Subscriber returns the Watermill subscriber for this VirtualApp.
func (v *VirtualApp) Subscriber() message.Subscriber {
	return v.goChannel
}

// Context returns the VirtualApp's context.
func (v *VirtualApp) Context() context.Context {
	return v.ctx
}

// AddTransport adds a transport to the VirtualApp.
func (v *VirtualApp) AddTransport(t Transport) {
	v.transports = append(v.transports, t)
}

// Transports returns all transports.
func (v *VirtualApp) Transports() []Transport {
	return v.transports
}

// Addresses returns a map of transport type to address.
func (v *VirtualApp) Addresses() map[string]string {
	addresses := make(map[string]string)
	for _, t := range v.transports {
		addresses[t.Type()] = t.Address()
	}
	return addresses
}

// Start starts all transports.
func (v *VirtualApp) Start() {
	for _, t := range v.transports {
		v.wg.Add(1)
		go func(transport Transport) {
			defer v.wg.Done()
			transport.Start(v.ctx)
		}(t)
	}
}

// Stop gracefully shuts down the GoChannel and all transports.
func (v *VirtualApp) Stop() {
	v.cancel()
	v.goChannel.Close()
	for _, t := range v.transports {
		t.Stop()
	}
	v.wg.Wait()
}
