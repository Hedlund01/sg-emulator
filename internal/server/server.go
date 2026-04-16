package server

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sync"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"google.golang.org/protobuf/proto"

	"sg-emulator/internal/ca"
	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server/messages"
	"sg-emulator/internal/trace"
	sgverifier "sg-emulator/internal/verifier"
)

type handlerFunc func(ctx context.Context, payload any) (any, error)

// Server wraps a scalegraph.App and processes requests in its own goroutine.
// It also manages VirtualApps through its Registry.
type Server struct {
	app         *scalegraph.App
	registry    *Registry
	requestChan chan messages.Request
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	logger      *slog.Logger
	ca          *ca.CA
	verifier    *sgverifier.Verifier
	handlers    map[reflect.Type]handlerFunc
}

// RegisterHandler registers a typed handler for a request type.
func RegisterHandler[Req, Resp any](s *Server, handler func(ctx context.Context, req *Req) (*Resp, error)) {
	reqType := reflect.TypeOf((*Req)(nil))
	s.handlers[reqType] = func(ctx context.Context, payload any) (any, error) {
		req, ok := payload.(*Req)
		if !ok {
			return nil, fmt.Errorf("invalid payload type: got %T, want *%s", payload, reqType.Elem().Name())
		}
		return handler(ctx, req)
	}
}

// NewWithCA creates a new Server with a Certificate Authority
func NewWithCA(logger *slog.Logger, certAuth *ca.CA) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	app := scalegraph.NewApp(logger.With("component", "app"))
	s := &Server{
		app:         app,
		registry:    NewRegistry(logger.With("component", "registry")),
		requestChan: make(chan messages.Request, 1000),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		ca:          certAuth,
		verifier:    sgverifier.NewVerifier(certAuth.Certificate(), app),
		handlers:    make(map[reflect.Type]handlerFunc),
	}
	s.registerHandlers()
	return s
}

// App returns the underlying scalegraph.App.
func (s *Server) App() *scalegraph.App {
	return s.app
}

func (s *Server) registerHandlers() {

	// Money
	RegisterHandler(s, s.handleTransfer)
	RegisterHandler(s, s.handleMint)

	//Accounts
	RegisterHandler(s, s.handleAccountCount)
	RegisterHandler(s, s.handleCreateAccount)
	RegisterHandler(s, s.handleGetAccount)
	RegisterHandler(s, s.handleGetAccounts)

	// Admin (unauthenticated, flag-gated at transport layer)
	RegisterHandler(s, s.handleAdminCreateAccount)
	RegisterHandler(s, s.handleAdminMint)

	//Tokens
	RegisterHandler(s, s.handleMintToken)
	RegisterHandler(s, s.handleAuthorizeTokenTransfer)
	RegisterHandler(s, s.handleTransferToken)
	RegisterHandler(s, s.handleUnauthorizeTokenTransfer)
	RegisterHandler(s, s.handleBurnToken)
	RegisterHandler(s, s.handleClawbackToken)
	RegisterHandler(s, s.handleTokenLookup)
}

func (s *Server) handleCreateAccount(ctx context.Context, req *scalegraph.CreateAccountRequest) (*scalegraph.CreateAccountResponse, error) {
	// Create account credentials via CA
	keyPair, cert, _, err := s.ca.CreateAccountCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to create account credentials: %w", err)
	}

	acc, err := s.app.CreateAccountWithKeys(ctx, keyPair.PublicKey, cert, req.InitialBalance)
	if err != nil {
		return nil, err
	}

	certPEM := crypto.EncodeCertificatePEM(cert)
	privKeyPEM, _ := crypto.EncodePrivateKeyPEM(keyPair.PrivateKey)
	pubKeyPEM, _ := crypto.EncodePublicKeyPEM(keyPair.PublicKey)

	return &scalegraph.CreateAccountResponse{
		Account:     acc,
		Certificate: certPEM,
		PrivateKey:  string(privKeyPEM),
		PublicKey:   string(pubKeyPEM),
	}, nil
}

func (s *Server) handleGetAccount(ctx context.Context, req *scalegraph.GetAccountRequest) (*scalegraph.GetAccountResponse, error) {
	return s.app.GetAccount(ctx, req)
}

func (s *Server) handleGetAccounts(ctx context.Context, req *scalegraph.GetAccountsRequest) (*scalegraph.GetAccountsResponse, error) {
	return s.app.GetAccounts(ctx, req)
}

func (s *Server) handleTransfer(ctx context.Context, req *scalegraph.TransferRequest) (*scalegraph.TransferResponse, error) {
	return s.app.Transfer(ctx, req)
}

func (s *Server) handleMint(ctx context.Context, req *scalegraph.MintRequest) (*scalegraph.MintResponse, error) {
	err := s.app.Mint(ctx, req)
	if err != nil {
		return nil, err
	}
	return &scalegraph.MintResponse{}, nil
}

func (s *Server) handleMintToken(ctx context.Context, req *scalegraph.MintTokenRequest) (*scalegraph.MintTokenResponse, error) {
	return s.app.MintToken(ctx, req)
}

func (s *Server) handleAccountCount(ctx context.Context, req *scalegraph.AccountCountRequest) (*scalegraph.AccountCountResponse, error) {
	return s.app.AccountCount(ctx, req)
}

func (s *Server) handleAuthorizeTokenTransfer(ctx context.Context, req *scalegraph.AuthorizeTokenTransferRequest) (*scalegraph.AuthorizeTokenTransferResponse, error) {
	err := s.app.AuthorizeTokenTransfer(ctx, req)
	if err != nil {
		return nil, err
	}
	return &scalegraph.AuthorizeTokenTransferResponse{}, nil
}

func (s *Server) handleUnauthorizeTokenTransfer(ctx context.Context, req *scalegraph.UnauthorizeTokenTransferRequest) (*scalegraph.UnauthorizeTokenTransferResponse, error) {
	err := s.app.UnauthorizeTokenTransfer(ctx, req)
	if err != nil {
		return nil, err
	}
	return &scalegraph.UnauthorizeTokenTransferResponse{}, nil
}

func (s *Server) handleTransferToken(ctx context.Context, req *scalegraph.TransferTokenRequest) (*scalegraph.TransferTokenResponse, error) {
	err := s.app.TransferToken(ctx, req)
	if err != nil {
		return nil, err
	}
	return &scalegraph.TransferTokenResponse{}, nil
}

func (s *Server) handleBurnToken(ctx context.Context, req *scalegraph.BurnTokenRequest) (*scalegraph.BurnTokenResponse, error) {
	err := s.app.BurnToken(ctx, req)
	if err != nil {
		return nil, err
	}
	return &scalegraph.BurnTokenResponse{}, nil
}

func (s *Server) handleClawbackToken(ctx context.Context, req *scalegraph.ClawbackTokenRequest) (*scalegraph.ClawbackTokenResponse, error) {
	err := s.app.ClawbackToken(ctx, req)
	if err != nil {
		return nil, err
	}
	return &scalegraph.ClawbackTokenResponse{}, nil
}

func (s *Server) handleTokenLookup(ctx context.Context, req *scalegraph.LookupTokenRequest) (*scalegraph.LookupTokenResponse, error) {
	resp, err := s.app.LookupToken(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *Server) handleAdminCreateAccount(ctx context.Context, req *scalegraph.AdminCreateAccountRequest) (*scalegraph.CreateAccountResponse, error) {
	keyPair, cert, _, err := s.ca.CreateAccountCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to create account credentials: %w", err)
	}
	acc, err := s.app.CreateAccountWithKeys(ctx, keyPair.PublicKey, cert, req.InitialBalance)
	if err != nil {
		return nil, err
	}
	certPEM := crypto.EncodeCertificatePEM(cert)
	privKeyPEM, _ := crypto.EncodePrivateKeyPEM(keyPair.PrivateKey)
	pubKeyPEM, _ := crypto.EncodePublicKeyPEM(keyPair.PublicKey)
	return &scalegraph.CreateAccountResponse{
		Account:     acc,
		Certificate: certPEM,
		PrivateKey:  string(privKeyPEM),
		PublicKey:   string(pubKeyPEM),
	}, nil
}

func (s *Server) handleAdminMint(ctx context.Context, req *scalegraph.AdminMintRequest) (*scalegraph.AdminMintResponse, error) {
	err := s.app.Mint(ctx, &scalegraph.MintRequest{To: req.To, Amount: req.Amount})
	if err != nil {
		return nil, err
	}
	return &scalegraph.AdminMintResponse{}, nil
}

// CA returns the server's Certificate Authority (may be nil)
func (s *Server) CA() *ca.CA {
	return s.ca
}

// Start begins processing requests in a separate goroutine.
// The server requires a CA (Certificate Authority) to be configured for operations
// that mandate cryptographic signatures (e.g., transfers).
func (s *Server) Start() {
	if s.ca == nil || s.verifier == nil {
		s.logger.Error("Server starting without CA - stopping server from starting",
			"buffer_size", cap(s.requestChan))
		return
	}
	s.logger.Info("Server starting with CA",
		"buffer_size", cap(s.requestChan),
		"signature_verification", "enabled")
	s.wg.Add(1)
	go s.run()
}

// Stop gracefully shuts down the Server and all VirtualApps
func (s *Server) Stop() {
	s.logger.Info("Server stopping", "virtual_apps", s.registry.Count())
	// Stop all virtual apps
	vapps := s.registry.List()
	for _, vapp := range vapps {
		s.logger.Debug("Stopping virtual app", "id", vapp.ID())
		vapp.Stop()
	}
	s.cancel()
	s.wg.Wait()
	s.logger.Info("Server stopped")
}

// RequestChannel returns the channel for sending requests to the server
func (s *Server) RequestChannel() chan<- messages.Request {
	return s.requestChan
}

// Registry returns the VirtualApp registry
func (s *Server) Registry() *Registry {
	return s.registry
}

// CreateVirtualApp creates and registers a new VirtualApp (without transports).
// Caller must add transports and call Start() on the returned VirtualApp.
func (s *Server) CreateVirtualApp() (*VirtualApp, error) {
	vapp, err := newVirtualApp(s.requestChan, s.logger.With("component", "virtual-app"))
	if err != nil {
		s.logger.Error("Failed to create virtual app", "error", err)
		return nil, err
	}

	if err := s.registry.Register(vapp); err != nil {
		s.logger.Error("Failed to register virtual app", "error", err, "id", vapp.ID())
		return nil, err
	}

	s.logger.Info("Virtual app created and registered", "id", vapp.ID())
	return vapp, nil
}

// run is the main processing loop that handles incoming requests
func (s *Server) run() {
	defer s.wg.Done()

	for {
		select {
		case req, ok := <-s.requestChan:
			if !ok {
				return
			}
			resp := s.handleRequest(req)
			// Send response back to client. The client may have timed out
			// and closed ResponseChan, so we recover from the panic.
			func() {
				defer func() { recover() }()
				select {
				case req.ResponseChan <- resp:
				default:
				}
			}()
		case <-s.ctx.Done():
			// Drain remaining requests so clients don't hang
			s.drainRequests()
			return
		}
	}
}

// drainRequests sends error responses to any requests still in the channel
// so that waiting clients are unblocked during shutdown.
func (s *Server) drainRequests() {
	for {
		select {
		case req, ok := <-s.requestChan:
			if !ok {
				return
			}
			resp := messages.Response{
				ID:    req.ID,
				Error: fmt.Errorf("server shutting down"),
			}
			func() {
				defer func() { recover() }()
				select {
				case req.ResponseChan <- resp:
				default:
				}
			}()
		default:
			return
		}
	}
}

// handleRequest processes a single request and returns the response
func (s *Server) handleRequest(req messages.Request) messages.Response {
	traceID := trace.GetTraceID(req.Context)
	logger := s.logger
	if traceID != "" {
		logger = logger.With("trace_id", traceID)
	}

	handler, ok := s.handlers[reflect.TypeOf(req.Payload)]
	if !ok {
		logger.Warn("Unknown request type", "type", fmt.Sprintf("%T", req.Payload))
		return messages.Response{ID: req.ID, Error: fmt.Errorf("unknown request type: %T", req.Payload)}
	}

	// Auto-verify signature if payload implements crypto.Verifiable
	if v, ok := req.Payload.(crypto.Verifiable); ok {
		if v.RequiresSignature() {
			if s.verifier == nil {
				return messages.Response{ID: req.ID, Error: crypto.ErrNoVerifier}
			}
			if err := v.Verify(s.verifier, s.ca.PublicKey()); err != nil {
				logger.Warn("Signature verification failed", "error", err, "type", fmt.Sprintf("%T", req.Payload))
				return messages.Response{ID: req.ID, Error: err}
			}
		}
	}

	result, err := handler(req.Context, req.Payload)
	if err != nil {
		return messages.Response{ID: req.ID, Error: err}
	}

	// Publish event to all VirtualApp event buses after successful operation
	s.publishEvent(req.Payload, result)

	return messages.Response{ID: req.ID, Payload: result}
}

// publishEvent constructs an event from the request/response and publishes it
// to all VirtualApp GoChannels, one topic per involved account.
func (s *Server) publishEvent(requestPayload any, responsePayload any) {
	info := extractEventInfo(requestPayload, responsePayload)
	if info == nil {
		return
	}

	event := BuildEvent(info)
	if event == nil {
		return
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		s.logger.Error("failed to marshal event", "error", err)
		return
	}

	accountIDs := eventInvolvedAccounts(event)
	vapps := s.registry.List()
	for _, vapp := range vapps {
		pub := vapp.Publisher()
		for _, accountID := range accountIDs {
			msg := message.NewMessage(watermill.NewUUID(), payload)
			msg.Metadata.Set("event_type", event.GetType().String())
			if err := pub.Publish("events."+accountID, msg); err != nil {
				s.logger.Warn("failed to publish event", "topic", "events."+accountID, "error", err)
			}
		}
	}
}
