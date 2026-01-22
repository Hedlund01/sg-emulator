package server

import (
	"context"
	"sync"

	"sg-emulator/internal/scalegraph"
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
}

// New creates a new Server with a fresh App and Registry
func New() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		app:         scalegraph.New(),
		registry:    NewRegistry(),
		requestChan: make(chan Request, 1000),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins processing requests in a separate goroutine
func (s *Server) Start() {
	s.wg.Add(1)
	go s.run()
}

// Stop gracefully shuts down the Server and all VirtualApps
func (s *Server) Stop() {
	// Stop all virtual apps
	for _, vapp := range s.registry.List() {
		vapp.Stop()
	}
	s.cancel()
	s.wg.Wait()
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
	vapp, err := newVirtualApp(s.requestChan)
	if err != nil {
		return nil, err
	}

	if err := s.registry.Register(vapp); err != nil {
		return nil, err
	}

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
	resp := Response{
		ID:      req.ID,
		Success: true,
	}

	switch req.Type {
	case ReqCreateAccount:
		payload := req.Payload.(CreateAccountPayload)
		acc, err := s.app.CreateAccount(payload.InitialBalance)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = CreateAccountResponse{Account: acc}
		}

	case ReqGetAccount:
		payload := req.Payload.(GetAccountPayload)
		acc, err := s.app.GetAccount(payload.ID)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = GetAccountResponse{Account: acc}
		}

	case ReqGetAccounts:
		accounts := s.app.GetAccounts()
		resp.Payload = GetAccountsResponse{Accounts: accounts}

	case ReqTransfer:
		payload := req.Payload.(TransferPayload)
		err := s.app.Transfer(payload.From, payload.To, payload.Amount)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = TransferResponse{}
		}

	case ReqMint:
		payload := req.Payload.(MintPayload)
		err := s.app.Mint(payload.To, payload.Amount)
		if err != nil {
			resp.Success = false
			resp.Error = err.Error()
		} else {
			resp.Payload = MintResponse{}
		}

	case ReqAccountCount:
		count := s.app.AccountCount()
		resp.Payload = AccountCountResponse{Count: count}

	default:
		resp.Success = false
		resp.Error = "unknown request type"
	}

	return resp
}
