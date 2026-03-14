package grpc

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	protovalidate "buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	adminv1 "sg-emulator/gen/admin/v1"
	"sg-emulator/gen/admin/v1/adminv1connect"
	currencyv1 "sg-emulator/gen/currency/v1"
	"sg-emulator/gen/currency/v1/currencyv1connect"
	eventv1 "sg-emulator/gen/event/v1"
	"sg-emulator/gen/event/v1/eventv1connect"
	tokenv1 "sg-emulator/gen/token/v1"
	"sg-emulator/gen/token/v1/tokenv1connect"
	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server"
)

// Transport implements the ConnectRPC transport for VirtualApps.
// It also implements server.EventTransport for event delivery.
type Transport struct {
	address     string
	client      *server.Client
	logger      *slog.Logger
	httpServer  *http.Server
	exposeAdmin bool

	// Event support
	eventCh     chan server.EventDelivery
	subscribers map[string]chan server.EventDelivery
	subMu       sync.RWMutex
	verifier    *crypto.Verifier
	caPublicKey ed25519.PublicKey
	eventBus    *server.EventBus
}

// New creates a new ConnectRPC transport with the given address and client.
func New(address string, client *server.Client, verifier *crypto.Verifier, caPublicKey ed25519.PublicKey, exposeAdmin bool, eventBus *server.EventBus, logger *slog.Logger) *Transport {
	logger.Info("gRPC transport created", "address", address)
	return &Transport{
		address:     address,
		client:      client,
		logger:      logger,
		exposeAdmin: exposeAdmin,
		eventCh:     make(chan server.EventDelivery, 256),
		subscribers: make(map[string]chan server.EventDelivery),
		verifier:    verifier,
		caPublicKey: caPublicKey,
		eventBus:    eventBus,
	}
}

// EventChannel returns the channel the EventBus delivers events to.
func (t *Transport) EventChannel() chan<- server.EventDelivery {
	return t.eventCh 
}

// currencyHandler implements currencyv1connect.CurrencyServiceHandler.
type currencyHandler struct {
	currencyv1connect.UnimplementedCurrencyServiceHandler
	client *server.Client
	logger *slog.Logger
}

// Transfer handles a currency transfer request.
func (h *currencyHandler) Transfer(ctx context.Context, req *currencyv1.TransferRequest) (*currencyv1.TransferResponse, error) {
	envelope, err := convertTransferEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.client.TransferSigned(ctx, envelope); err != nil {
		h.logger.Error("Transfer failed",
			"from", req.GetSignedEnvelope().GetPayload().GetFrom(),
			"to", req.GetSignedEnvelope().GetPayload().GetTo(),
			"amount", req.GetSignedEnvelope().GetPayload().GetAmount(),
			"error", err,
		)
		return &currencyv1.TransferResponse{Success: false, ErrorMessage: err.Error()}, nil
	}

	return &currencyv1.TransferResponse{Success: true}, nil
}

// tokenHandler implements tokenv1connect.TokenServiceHandler.
type tokenHandler struct {
	tokenv1connect.UnimplementedTokenServiceHandler
	client *server.Client
	logger *slog.Logger
}

// MintToken handles a mint token request.
func (h *tokenHandler) MintToken(ctx context.Context, req *tokenv1.MintTokenRequest) (*tokenv1.MintTokenResponse, error) {
	envelope, err := convertMintTokenEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.client.MintTokenSigned(ctx, envelope); err != nil {
		h.logger.Error("MintToken failed", "error", err)
		return &tokenv1.MintTokenResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &tokenv1.MintTokenResponse{Success: true}, nil
}

// TransferToken handles a transfer token request.
func (h *tokenHandler) TransferToken(ctx context.Context, req *tokenv1.TransferTokenRequest) (*tokenv1.TransferTokenResponse, error) {
	envelope, err := convertTransferTokenEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.client.TransferTokenSigned(ctx, envelope); err != nil {
		h.logger.Error("TransferToken failed", "error", err)
		return &tokenv1.TransferTokenResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &tokenv1.TransferTokenResponse{Success: true}, nil
}

// AuthorizeTokenTransfer handles an authorize token transfer request.
func (h *tokenHandler) AuthorizeTokenTransfer(ctx context.Context, req *tokenv1.AuthorizeTokenTransferRequest) (*tokenv1.AuthorizeTokenTransferResponse, error) {
	envelope, err := convertAuthorizeTokenTransferEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.client.AuthorizeTokenTransferSigned(ctx, envelope); err != nil {
		h.logger.Error("AuthorizeTokenTransfer failed", "error", err)
		return &tokenv1.AuthorizeTokenTransferResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &tokenv1.AuthorizeTokenTransferResponse{Success: true}, nil
}

// UnauthorizeTokenTransfer handles an unauthorize token transfer request.
func (h *tokenHandler) UnauthorizeTokenTransfer(ctx context.Context, req *tokenv1.UnauthorizeTokenTransferRequest) (*tokenv1.UnauthorizeTokenTransferResponse, error) {
	envelope, err := convertUnauthorizeTokenTransferEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.client.UnauthorizeTokenTransferSigned(ctx, envelope); err != nil {
		h.logger.Error("UnauthorizeTokenTransfer failed", "error", err)
		return &tokenv1.UnauthorizeTokenTransferResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &tokenv1.UnauthorizeTokenTransferResponse{Success: true}, nil
}

// BurnToken handles a burn token request.
func (h *tokenHandler) BurnToken(ctx context.Context, req *tokenv1.BurnTokenRequest) (*tokenv1.BurnTokenResponse, error) {
	envelope, err := convertBurnTokenEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.client.BurnTokenSigned(ctx, envelope); err != nil {
		h.logger.Error("BurnToken failed", "error", err)
		return &tokenv1.BurnTokenResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &tokenv1.BurnTokenResponse{Success: true}, nil
}

// LookupToken handles a lookup token request.
func (h *tokenHandler) LookupToken(ctx context.Context, req *tokenv1.LookupTokenRequest) (*tokenv1.LookupTokenResponse, error) {
	envelope, err := convertLookupTokenEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	resp, err := h.client.LookupTokenSigned(ctx, envelope)
	if err != nil {
		h.logger.Error("LookupToken failed", "error", err)
		return &tokenv1.LookupTokenResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	var protoToken *tokenv1.Token
	if resp.Token != nil {
		t := resp.Token
		var cb string
		if t.ClawbackAddress() != nil {
			cb = t.ClawbackAddress().String()
		}
		protoToken = &tokenv1.Token{
			TokenId:         t.ID(),
			TokenValue:      t.Value(),
			ClawbackAddress: cb,
			Owner:           envelope.Payload.AccountID,
		}
	}
	return &tokenv1.LookupTokenResponse{Success: true, Token: protoToken}, nil
}

// ClawbackToken handles a clawback token request.
func (h *tokenHandler) ClawbackToken(ctx context.Context, req *tokenv1.ClawbackTokenRequest) (*tokenv1.ClawbackTokenResponse, error) {
	envelope, err := convertClawbackTokenEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.client.ClawbackTokenSigned(ctx, envelope); err != nil {
		h.logger.Error("ClawbackToken failed", "error", err)
		return &tokenv1.ClawbackTokenResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &tokenv1.ClawbackTokenResponse{Success: true}, nil
}

// adminHandler implements adminv1connect.AdminServiceHandler.
type adminHandler struct {
	adminv1connect.UnimplementedAdminServiceHandler
	client *server.Client
	logger *slog.Logger
}

func (h *adminHandler) CreateAccount(ctx context.Context, req *adminv1.CreateAccountRequest) (*adminv1.CreateAccountResponse, error) {
	resp, err := h.client.AdminCreateAccount(ctx, req.GetInitialBalance())
	if err != nil {
		return &adminv1.CreateAccountResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &adminv1.CreateAccountResponse{
		Success:        true,
		AccountId:      resp.Account.ID().String(),
		PrivateKeyPem:  resp.PrivateKey,
		PublicKeyPem:   resp.PublicKey,
		CertificatePem: resp.Certificate,
	}, nil
}

func (h *adminHandler) Mint(ctx context.Context, req *adminv1.MintRequest) (*adminv1.MintResponse, error) {
	toID, err := scalegraph.ScalegraphIdFromString(req.GetAccountId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.client.AdminMint(ctx, toID, req.GetAmount()); err != nil {
		return &adminv1.MintResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &adminv1.MintResponse{Success: true}, nil
}

// eventHandler implements eventv1connect.EventServiceHandler.
type eventHandler struct {
	eventv1connect.UnimplementedEventServiceHandler
	transport *Transport
	logger    *slog.Logger
}

// Subscribe handles a server-streaming event subscription request.
// The request must be signed by the subscribing account's private key.
// Signature verification and subscriber management are owned by this transport.
func (h *eventHandler) Subscribe(ctx context.Context, req *eventv1.SubscribeRequest, stream *connect.ServerStream[eventv1.SubscribeResponse]) error {
	// Validate the proto message manually (streaming RPCs don't use unary interceptors)
	validator, err := protovalidate.New()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if err := validator.Validate(req); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	envelope, err := convertSubscribeEnvelope(req)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Build domain request for signature verification
	accountID, err := scalegraph.ScalegraphIdFromString(envelope.Payload.AccountID)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid account ID: %w", err))
	}
	domainReq := &scalegraph.SubscribeEventsRequest{
		AccountID:      accountID,
		EventTypes:     envelope.Payload.EventTypes,
		SignedEnvelope: envelope,
	}
	if err := domainReq.Verify(h.transport.verifier, h.transport.caPublicKey); err != nil {
		h.logger.Warn("Subscribe signature verification failed",
			"account_id", envelope.Payload.AccountID,
			"error", err,
		)
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("signature verification failed: %w", err))
	}

	// Register a per-subscriber channel
	subscriberID := envelope.Payload.AccountID
	subCh := make(chan server.EventDelivery, 256)

	h.transport.subMu.Lock()
	if _, exists := h.transport.subscribers[subscriberID]; exists {
		h.transport.subMu.Unlock()
		return connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("account %s already has an active subscription", subscriberID))
	}
	h.transport.subscribers[subscriberID] = subCh
	h.transport.subMu.Unlock()

	// Build EventBus filter and register so the EventBus dispatches events for this account.
	eventFilter := &server.EventFilter{
		AccountIDs: map[string]struct{}{subscriberID: {}},
		EventTypes: make(map[eventv1.EventType]struct{}),
	}
	for _, et := range req.GetSignedEnvelope().GetPayload().GetEventTypes() {
		eventFilter.EventTypes[et] = struct{}{}
	}
	if _, err := h.transport.eventBus.Subscribe(subscriberID, eventFilter); err != nil {
		// Undo the transport registration and surface as AlreadyExists.
		h.transport.subMu.Lock()
		delete(h.transport.subscribers, subscriberID)
		h.transport.subMu.Unlock()
		return connect.NewError(connect.CodeAlreadyExists, err)
	}

	defer func() {
		h.transport.subMu.Lock()
		delete(h.transport.subscribers, subscriberID)
		h.transport.subMu.Unlock()
		h.transport.eventBus.Unsubscribe(subscriberID)
	}()

	h.logger.Info("Client subscribed to events", "account_id", subscriberID)

	// Send an empty initial response to flush HTTP/2 response headers.
	// Without this, the client's Subscribe call blocks until the first event
	// arrives, causing a deadlock when the test calls MintToken only after
	// openSubscription returns.
	if err := stream.Send(&eventv1.SubscribeResponse{}); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("send ready signal: %w", err))
	}

	// Stream events until client disconnects
	for {
		select {
		case delivery, ok := <-subCh:
			if !ok {
				h.logger.Info("Subscriber channel closed", "account_id", subscriberID)
				return nil
			}
			if err := stream.Send(&eventv1.SubscribeResponse{Event: delivery.Event}); err != nil {
				h.logger.Debug("Failed to send event to subscriber", "account_id", subscriberID, "error", err)
				return err
			}
		case <-ctx.Done():
			h.logger.Info("Client disconnected", "account_id", subscriberID)
			return nil
		}
	}
}

// validationInterceptor returns a Connect interceptor that validates incoming
// requests using buf protovalidate rules defined in the proto schema.
func validationInterceptor(validator protovalidate.Validator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if msg, ok := req.Any().(proto.Message); ok {
				if err := validator.Validate(msg); err != nil {
					return nil, connect.NewError(connect.CodeInvalidArgument, err)
				}
			}
			return next(ctx, req)
		}
	}
}

// startEventRouter reads events from the transport's eventCh and routes them
// to the appropriate per-subscriber channel based on AccountID.
func (t *Transport) startEventRouter(ctx context.Context) {
	for {
		select {
		case delivery, ok := <-t.eventCh:
			if !ok {
				return
			}
			t.subMu.RLock()
			ch, exists := t.subscribers[delivery.AccountID]
			t.subMu.RUnlock()
			if !exists {
				continue
			}
			select {
			case ch <- delivery:
			default:
				t.logger.Warn("Subscriber channel full, dropping event",
					"account_id", delivery.AccountID,
					"event_type", delivery.Event.GetType(),
				)
			}
		case <-ctx.Done():
			return
		}
	}
}

// Start begins listening for ConnectRPC requests.
func (t *Transport) Start(ctx context.Context) error {
	go t.startEventRouter(ctx)

	validator, err := protovalidate.New()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	path, handler := currencyv1connect.NewCurrencyServiceHandler(
		&currencyHandler{client: t.client, logger: t.logger},
		connect.WithInterceptors(validationInterceptor(validator)),
	)
	mux.Handle(path, handler)

	tokenPath, tokenHandler := tokenv1connect.NewTokenServiceHandler(
		&tokenHandler{client: t.client, logger: t.logger},
		connect.WithInterceptors(validationInterceptor(validator)),
	)
	mux.Handle(tokenPath, tokenHandler)

	eventPath, eventHandler := eventv1connect.NewEventServiceHandler(
		&eventHandler{
			transport: t,
			logger:    t.logger,
		},
	)
	mux.Handle(eventPath, eventHandler)

	if t.exposeAdmin {
		adminPath, adminH := adminv1connect.NewAdminServiceHandler(
			&adminHandler{client: t.client, logger: t.logger},
			connect.WithInterceptors(validationInterceptor(validator)),
		)
		mux.Handle(adminPath, adminH)
		t.logger.Warn("Admin interface exposed — do not use in production", "path", adminPath)
	}

	p := new(http.Protocols)
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)

	t.httpServer = &http.Server{
		Addr:      t.address,
		Handler:   mux,
		Protocols: p,
	}

	go func() {
		t.logger.Info("gRPC transport listening", "address", t.address)
		if err := t.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.logger.Error("gRPC transport error", "error", err)
		}
	}()

	<-ctx.Done()
	return t.Stop()
}

// Stop gracefully shuts down the ConnectRPC transport.
func (t *Transport) Stop() error {
	if t.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return t.httpServer.Shutdown(ctx)
}

// Address returns the listening address.
func (t *Transport) Address() string {
	return t.address
}

// Type returns the transport type.
func (t *Transport) Type() string {
	return "ConnectRPC"
}
