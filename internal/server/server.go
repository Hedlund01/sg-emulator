package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"sg-emulator/internal/ca"
	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server/messages"
	"sg-emulator/internal/trace"
)

// Signature verification errors
var (
	// ErrNoVerifier indicates the server was not configured with a CA for signature verification
	ErrNoVerifier = errors.New("server not configured with certificate authority for signature verification")
	// ErrMissingSignature indicates a required signature was not provided
	ErrMissingSignature = errors.New("signature required but not provided")
	// ErrInvalidSignature indicates the signature verification failed
	ErrInvalidSignature = errors.New("signature verification failed")
	// ErrSignerMismatch indicates the signer ID does not match the expected account
	ErrSignerMismatch = errors.New("signer ID does not match expected account")
	// ErrPayloadMismatch indicates the payload data does not match the signed data
	ErrPayloadMismatch = errors.New("payload data does not match signed data")
)

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
	verifier    *crypto.Verifier
}

// New creates a new Server with a fresh App and Registry
func New(logger *slog.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		app:         scalegraph.NewApp(logger.With("component", "app")),
		registry:    NewRegistry(logger.With("component", "registry")),
		requestChan: make(chan messages.Request, 1000),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
	}
}

// NewWithCA creates a new Server with a Certificate Authority
func NewWithCA(logger *slog.Logger, certAuth *ca.CA) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		app:         scalegraph.NewApp(logger.With("component", "app")),
		registry:    NewRegistry(logger.With("component", "registry")),
		requestChan: make(chan messages.Request, 1000),
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

// verifySignedRequest verifies the cryptographic signature on a signed request payload.
// It is a generic function that works with any SignablePayload type.
//
// The function performs the following checks:
//  1. Verifies signature is provided when required
//  2. Validates certificate chain and cryptographic signature
//  3. Verifies signer ID matches expected account
//  4. Verifies payload data matches signed data
//
// Returns nil if verification succeeds or signature is optional and not provided.
// Returns wrapped error with context if verification fails.
func verifySignedRequest[T crypto.SignableData](s *Server, payload messages.SignablePayload[T]) error {
	signedReq := payload.GetSignedRequest()
	required := payload.RequiresSignature()

	if required {
		if s.verifier == nil {
			// Server misconfiguration - CA required for this operation
			return ErrNoVerifier
		}

		if signedReq == nil {
			// Signature is mandatory but not provided
			return ErrMissingSignature
		}
	} else {
		// Signature is optional - if not provided, skip verification
		if signedReq == nil {
			return nil
		}
	}

	// Verify the signed envelope (certificate chain, signature, timestamp)
	_, err := crypto.VerifyEnvelope(s.verifier, signedReq)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	// Verify the signer ID matches the expected account
	expectedSignerID := payload.GetSignerID()
	if expectedSignerID == (scalegraph.ScalegraphId{}) {
		// Zero value means this request must be signed by the CA's system account
		// (e.g., account creation requests)
		caSystemID := scalegraph.ScalegraphIdFromPublicKey(s.ca.PublicKey())
		if signedReq.Signature.SignerID != caSystemID.String() {
			return fmt.Errorf("%w: request requires CA signature, expected %s, got %s", ErrSignerMismatch, caSystemID, signedReq.Signature.SignerID)
		}
	} else if signedReq.Signature.SignerID != expectedSignerID.String() {
		return fmt.Errorf("%w: expected %s, got %s", ErrSignerMismatch, expectedSignerID, signedReq.Signature.SignerID)
	}

	// Verify the payload data matches the signed data
	if err := payload.VerifyPayloadData(); err != nil {
		return fmt.Errorf("%w: %v", ErrPayloadMismatch, err)
	}

	return nil
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
				ID:      req.ID,
				Success: false,
				Error:   "server shutting down",
			}
			select {
			case req.ResponseChan <- resp:
			default:
			}
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

	resp := messages.Response{
		ID:      req.ID,
		Success: true,
	}

	switch req.Type {
	case messages.ReqCreateAccount:
		payload := req.Payload.(*messages.CreateAccountWithKeysPayload)

		payloadCert, err := crypto.ParseCertificatePEM(payload.GetSignedRequest().Certificate)
		if err != nil {
			logger.Error("Failed to parse certificate from CreateAccountWithKeys payload", "error", err)
			resp.Success = false
			resp.Error = "Invalid certificate in request"
			break
		}

		if !s.ca.Certificate().Equal(payloadCert) {
			logger.Error("Certificate in CreateAccountWithKeys payload does not match server CA certificate")
			resp.Success = false
			resp.Error = "Certificate in request does not match server CA certificate"
			break
		}

		if err := verifySignedRequest(s, payload); err != nil {
			logger.Warn("CreateAccountWithKeys signature verification failed", "error", err)
			resp.Success = false
			resp.Error = err.Error()
			break
		}

		if s.ca == nil {
			logger.Error("No CA available to create a new account with credentials")
			resp.Error = "Internal server error: no CA available for account creation"
			resp.Success = false
			break
		}

		// If CA is available, create account with cryptographic credentials
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
		}
		certPEM := crypto.EncodeCertificatePEM(cert)
		privKeyPEM, _ := crypto.EncodePrivateKeyPEM(keyPair.PrivateKey)
		pubKeyPEM, _ := crypto.EncodePublicKeyPEM(keyPair.PublicKey)
		resp.Payload = messages.CreateAccountWithKeysResponse{
			Account:     acc,
			Certificate: certPEM,
			PrivateKey:  string(privKeyPEM),
			PublicKey:   string(pubKeyPEM),
		}
		logger.Info("Account created with cryptographic credentials", "account_id", accountID)

	case messages.ReqGetAccount:
		payload := req.Payload.(*messages.GetAccountPayload)

		if err := verifySignedRequest(s, payload); err != nil {
			logger.Warn("GetAccount signature verification failed", "error", err, "account_id", payload.AccountID)
			resp.Success = false
			resp.Error = err.Error()
			break
		}

		acc, err := s.app.GetAccount(req.Context, payload.AccountID)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = messages.GetAccountResponse{Account: acc}
		}

	case messages.ReqGetAccounts:
		accounts := s.app.GetAccounts(req.Context)
		resp.Payload = messages.GetAccountsResponse{Accounts: accounts}

	case messages.ReqTransfer:
		payload := req.Payload.(*messages.TransferPayload)

		// Verify cryptographic signature
		if err := verifySignedRequest(s, payload); err != nil {
			logger.Warn("Transfer signature verification failed", "error", err, "from", payload.From)
			resp.Success = false
			resp.Error = err.Error()
			break
		}

		logger.Debug("Transfer signature verified", "from", payload.From, "to", payload.To, "amount", payload.Amount)

		err := s.app.Transfer(req.Context, payload.From, payload.To, payload.Amount, payload.Nonce)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = messages.TransferResponse{}
		}

	case messages.ReqMint:
		payload := req.Payload.(messages.MintPayload)
		err := s.app.Mint(req.Context, payload.To, payload.Amount)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = messages.MintResponse{}
		}

	case messages.ReqAccountCount:
		count := s.app.AccountCount(req.Context)
		resp.Payload = messages.AccountCountResponse{Count: count}

	default:
		logger.Warn("Unknown request type", "type", req.Type)
		resp.Success = false
		resp.Error = "unknown request type"
	}

	return resp
}
