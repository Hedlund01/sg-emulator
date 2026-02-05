package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"sg-emulator/internal/ca"
	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/trace"
)

// Server wraps a scalegraph.App and processes requests in its own goroutine.
// It also manages VirtualApps through its Registry.
type Server struct {
	app         *scalegraph.App
	registry    *Registry
	requestChan chan Request
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	logger      *slog.Logger
	ca          *ca.CA
	verifier    *crypto.Verifier
}

// New creates a new Server with a fresh App and Registry
func New(logger *slog.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		app:         scalegraph.New(logger.With("component", "app")),
		registry:    NewRegistry(logger.With("component", "registry")),
		requestChan: make(chan Request, 1000),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
	}
}

// NewWithCA creates a new Server with a Certificate Authority
func NewWithCA(logger *slog.Logger, certAuth *ca.CA) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		app:         scalegraph.New(logger.With("component", "app")),
		registry:    NewRegistry(logger.With("component", "registry")),
		requestChan: make(chan Request, 1000),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		ca:          certAuth,
		verifier:    certAuth.NewVerifier(),
	}
}

// CA returns the server's Certificate Authority (may be nil)
func (s *Server) CA() *ca.CA {
	return s.ca
}

// Start begins processing requests in a separate goroutine
func (s *Server) Start() {
	s.logger.Info("Server starting", "buffer_size", cap(s.requestChan))
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
func (s *Server) RequestChannel() chan<- Request {
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
			// Send response back to client (non-blocking to avoid deadlock)
			select {
			case req.ResponseChan <- resp:
			default:
				// Response channel full or closed, skip
			}
		case <-s.ctx.Done():
			return
		}
	}
}

// handleRequest processes a single request and returns the response
func (s *Server) handleRequest(req Request) Response {
	traceID := trace.GetTraceID(req.Context)
	logger := s.logger
	if traceID != "" {
		logger = logger.With("trace_id", traceID)
	}

	resp := Response{
		ID:      req.ID,
		Success: true,
	}

	switch req.Type {
	case ReqCreateAccount:
		payload := req.Payload.(CreateAccountPayload)
		// If CA is available, create account with cryptographic credentials
		if s.ca != nil {
			keyPair, cert, accountID, err := s.ca.CreateAccountCredentials()
			if err != nil {
				logger.Error("CreateAccount with keys failed", "error", err)
				resp.Success = false
				resp.Error = err.Error()
				break
			}

			acc, err := s.app.CreateAccountWithKeys(req.Context, keyPair.PublicKey, cert, payload.InitialBalance)
			if err != nil {
				logger.Error("CreateAccountWithKeys request failed", "error", err, "initial_balance", payload.InitialBalance)
				resp.Success = false
				resp.Error = err.Error()
			} else {
				certPEM := crypto.EncodeCertificatePEM(cert)
				privKeyPEM, _ := crypto.EncodePrivateKeyPEM(keyPair.PrivateKey)
				resp.Payload = CreateAccountResponse{
					Account:     acc,
					Certificate: certPEM,
					PrivateKey:  string(privKeyPEM),
				}
				logger.Info("Account created with cryptographic credentials", "account_id", accountID)
			}
		} else {
			// Fallback to legacy account creation without crypto
			acc, err := s.app.CreateAccount(req.Context, payload.InitialBalance)
			if err != nil {
				logger.Error("CreateAccount request failed", "error", err, "initial_balance", payload.InitialBalance)
				resp.Success = false
				resp.Error = err.Error()
			} else {
				resp.Payload = CreateAccountResponse{Account: acc}
			}
		}

	case ReqGetAccount:
		payload := req.Payload.(GetAccountPayload)
		acc, err := s.app.GetAccount(req.Context, payload.ID)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = GetAccountResponse{Account: acc}
		}

	case ReqGetAccounts:
		accounts := s.app.GetAccounts(req.Context)
		resp.Payload = GetAccountsResponse{Accounts: accounts}

	case ReqTransfer:
		payload := req.Payload.(TransferPayload)

		// If verifier is available and a signed request is provided, verify it
		if s.verifier != nil && payload.SignedRequest != nil {
			// Verify the signed envelope
			_, err := crypto.VerifyEnvelope(s.verifier, payload.SignedRequest)
			if err != nil {
				logger.Warn("Transfer signature verification failed", "error", err, "from", payload.From)
				resp.Success = false
				resp.Error = fmt.Sprintf("signature verification failed: %v", err)
				break
			}

			// Verify the signer ID matches the From account
			if payload.SignedRequest.Signature.SignerID != payload.From.String() {
				logger.Warn("Transfer signer ID mismatch", "signer_id", payload.SignedRequest.Signature.SignerID, "from", payload.From)
				resp.Success = false
				resp.Error = "signer ID does not match source account"
				break
			}

			logger.Debug("Transfer signature verified", "from", payload.From, "to", payload.To, "amount", payload.Amount)
		}

		err := s.app.Transfer(req.Context, payload.From, payload.To, payload.Amount, payload.Nonce)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = TransferResponse{}
		}

	case ReqMint:
		payload := req.Payload.(MintPayload)
		err := s.app.Mint(req.Context, payload.To, payload.Amount)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = MintResponse{}
		}

	case ReqAccountCount:
		count := s.app.AccountCount(req.Context)
		resp.Payload = AccountCountResponse{Count: count}

	default:
		logger.Warn("Unknown request type", "type", req.Type)
		resp.Success = false
		resp.Error = "unknown request type"
	}

	return resp
}
