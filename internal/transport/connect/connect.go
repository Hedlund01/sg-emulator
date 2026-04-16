package grpc

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	protovalidate "buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"github.com/ThreeDotsLabs/watermill/message"
	"google.golang.org/protobuf/proto"

	accountv1 "sg-emulator/gen/account/v1"
	"sg-emulator/gen/account/v1/accountv1connect"
	adminv1 "sg-emulator/gen/admin/v1"
	"sg-emulator/gen/admin/v1/adminv1connect"
	currencyv1 "sg-emulator/gen/currency/v1"
	"sg-emulator/gen/currency/v1/currencyv1connect"
	eventv1 "sg-emulator/gen/event/v1"
	"sg-emulator/gen/event/v1/eventv1connect"
	tokenv1 "sg-emulator/gen/token/v1"
	"sg-emulator/gen/token/v1/tokenv1connect"
	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server"
	sgverifier "sg-emulator/internal/verifier"
)

// Transport implements the ConnectRPC transport for VirtualApps.
type Transport struct {
	address     string
	client      *server.Client
	logger      *slog.Logger
	httpServer  *http.Server
	exposeAdmin bool

	// Event support
	wmSubscriber        message.Subscriber
	activeSubscriptions map[string]struct{}
	activeSubMu         sync.Mutex
	verifier            *sgverifier.Verifier
	caPublicKey         ed25519.PublicKey
}

// New creates a new ConnectRPC transport with the given address and client.
func New(address string, client *server.Client, verifier *sgverifier.Verifier, caPublicKey ed25519.PublicKey, exposeAdmin bool, wmSubscriber message.Subscriber, logger *slog.Logger) *Transport {
	logger.Info("gRPC transport created", "address", address)
	return &Transport{
		address:             address,
		client:              client,
		logger:              logger,
		exposeAdmin:         exposeAdmin,
		wmSubscriber:        wmSubscriber,
		activeSubscriptions: make(map[string]struct{}),
		verifier:            verifier,
		caPublicKey:         caPublicKey,
	}
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

// FreezeToken handles a freeze token request.
func (h *tokenHandler) FreezeToken(ctx context.Context, req *tokenv1.FreezeTokenRequest) (*tokenv1.FreezeTokenResponse, error) {
	envelope, err := convertFreezeTokenEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.client.FreezeTokenSigned(ctx, envelope); err != nil {
		h.logger.Error("FreezeToken failed", "error", err)
		return &tokenv1.FreezeTokenResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &tokenv1.FreezeTokenResponse{Success: true}, nil
}

// UnfreezeToken handles an unfreeze token request.
func (h *tokenHandler) UnfreezeToken(ctx context.Context, req *tokenv1.UnfreezeTokenRequest) (*tokenv1.UnfreezeTokenResponse, error) {
	envelope, err := convertUnfreezeTokenEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := h.client.UnfreezeTokenSigned(ctx, envelope); err != nil {
		h.logger.Error("UnfreezeToken failed", "error", err)
		return &tokenv1.UnfreezeTokenResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &tokenv1.UnfreezeTokenResponse{Success: true}, nil
}

// accountHandler implements accountv1connect.AccountServiceHandler.
type accountHandler struct {
	accountv1connect.UnimplementedAccountServiceHandler
	client *server.Client
	logger *slog.Logger
}

func (h *accountHandler) GetAccount(ctx context.Context, req *accountv1.GetAccountRequest) (*accountv1.GetAccountResponse, error) {
	envelope, err := convertGetAccountEnvelope(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	accountID, err := scalegraph.ScalegraphIdFromString(envelope.Payload.AccountID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	acc, err := h.client.GetAccount(ctx, accountID, envelope)
	if err != nil {
		h.logger.Error("GetAccount failed", "account_id", envelope.Payload.AccountID, "error", err)
		return &accountv1.GetAccountResponse{Success: false, ErrorMessage: err.Error()}, nil
	}

	return &accountv1.GetAccountResponse{
		Success:         true,
		Balance:         acc.Balance(),
		Mbr:             acc.MBR(),
		OutgoingTxCount: acc.GetNonce(),
	}, nil
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

	subscriberID := envelope.Payload.AccountID

	// Guard: one active subscription per account.
	h.transport.activeSubMu.Lock()
	if _, exists := h.transport.activeSubscriptions[subscriberID]; exists {
		h.transport.activeSubMu.Unlock()
		return connect.NewError(connect.CodeAlreadyExists, errors.New("subscription already active"))
	}
	h.transport.activeSubscriptions[subscriberID] = struct{}{}
	h.transport.activeSubMu.Unlock()
	defer func() {
		h.transport.activeSubMu.Lock()
		delete(h.transport.activeSubscriptions, subscriberID)
		h.transport.activeSubMu.Unlock()
	}()

	// Build event type filter set (empty = accept all).
	filterTypes := map[string]struct{}{}
	for _, et := range req.GetSignedEnvelope().GetPayload().GetEventTypes() {
		filterTypes[et.String()] = struct{}{}
	}

	// Subscribe via Watermill to the per-account topic.
	topic := "events." + subscriberID
	msgs, err := h.transport.wmSubscriber.Subscribe(ctx, topic)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	h.logger.Info("Client subscribed to events", "account_id", subscriberID)

	// Send an empty initial response to flush HTTP/2 response headers.
	// Without this, the client's Subscribe call blocks until the first event
	// arrives, causing a deadlock when the test calls MintToken only after
	// openSubscription returns.
	if err := stream.Send(&eventv1.SubscribeResponse{}); err != nil {
		return err
	}

	// Stream events until client disconnects or broker shuts down.
	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return nil // subscriber channel closed
			}
			// Apply event type filter.
			if len(filterTypes) > 0 {
				et := msg.Metadata.Get("event_type")
				if _, pass := filterTypes[et]; !pass {
					msg.Ack()
					continue
				}
			}
			event := &eventv1.Event{}
			if err := proto.Unmarshal(msg.Payload, event); err != nil {
				h.logger.Error("failed to unmarshal event", "error", err)
				msg.Ack()
				continue
			}
			if err := stream.Send(&eventv1.SubscribeResponse{Event: event}); err != nil {
				h.logger.Debug("Failed to send event to subscriber", "account_id", subscriberID, "error", err)
				msg.Nack()
				return err
			}
			msg.Ack()
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

// Start begins listening for ConnectRPC requests.
func (t *Transport) Start(ctx context.Context) error {
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

	accountPath, accountH := accountv1connect.NewAccountServiceHandler(
		&accountHandler{client: t.client, logger: t.logger},
		connect.WithInterceptors(validationInterceptor(validator)),
	)
	mux.Handle(accountPath, accountH)

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
