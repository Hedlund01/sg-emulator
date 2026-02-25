package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"

	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server"
	_ "sg-emulator/internal/transport/rest/docs"
)

// @title SG Emulator REST API
// @version 1.0
// @description REST API for SG Emulator with cryptographically signed requests
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /

// Transport implements the REST transport for VirtualApps.
type Transport struct {
	address string
	client  *server.Client
	logger  *slog.Logger
	server  *http.Server
}

// New creates a new REST transport with the given address and client
func New(address string, client *server.Client, logger *slog.Logger) *Transport {
	logger.Info("REST transport created", "address", address)
	return &Transport{
		address: address,
		client:  client,
		logger:  logger,
	}
}

// Start begins listening for REST requests
func (t *Transport) Start(ctx context.Context) error {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(t.loggingMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Swagger documentation
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// Routes
	r.Get("/health", t.handleHealth)
	r.Post("/accounts/me", t.handleGetMyAccount) // Get own account (signed, authenticated)
	r.Post("/transfer", t.handleTransfer)        // Transfer (signed, authenticated)

	// Token routes
	r.Route("/tokens", func(r chi.Router) {
		r.Post("/mint", t.handleMintToken)                       // Mint a token (signed)
		r.Post("/authorize", t.handleAuthorizeTokenTransfer)     // Authorize token transfer (signed)
		r.Post("/unauthorize", t.handleUnauthorizeTokenTransfer) // Unauthorize token transfer (signed)
		r.Post("/transfer", t.handleTransferToken)               // Transfer a token (signed)
		r.Post("/burn", t.handleBurnToken)                       // Burn a token (signed)
		r.Post("/clawback", t.handleClawbackToken)               // Clawback a token (signed)
	})

	t.server = &http.Server{
		Addr:    t.address,
		Handler: r,
	}

	t.logger.Info("Starting REST server", "address", t.address, "swagger", "http://"+t.address+"/swagger/index.html")

	// Start server in goroutine
	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.logger.Error("REST server error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	return t.Stop()
}

// Stop gracefully shuts down the REST transport
func (t *Transport) Stop() error {
	if t.server != nil {
		t.logger.Info("Stopping REST server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return t.server.Shutdown(ctx)
	}
	return nil
}

// Address returns the listening address
func (t *Transport) Address() string {
	return t.address
}

// Type returns the transport type
func (t *Transport) Type() string {
	return "rest"
}

// loggingMiddleware logs all HTTP requests
func (t *Transport) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			t.logger.Debug("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"remote", r.RemoteAddr,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration", time.Since(start).String(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		}()

		next.ServeHTTP(ww, r)
	})
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
	Type   string `json:"type" example:"rest"`
}

// handleHealth godoc
// @Summary Health check
// @Description Get the health status of the REST API
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (t *Transport) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, HealthResponse{
		Status: "ok",
		Type:   "rest",
	})
}

// GetMyAccountRequest is the incoming request body for the /accounts/me endpoint
type GetMyAccountRequest struct {
	AccountID     scalegraph.ScalegraphId                           `json:"account_id"`
	SignedRequest *crypto.SignedEnvelope[*crypto.GetAccountPayload] `json:"signed_request"`
}

// AccountResponse represents account details with transaction history
type AccountResponse struct {
	ID           string                   `json:"id" example:"6c439a07c32f7fb09c29403d8d2e4e47b8c5e8a9"`
	Balance      float64                  `json:"balance" example:"100.50"`
	Transactions []map[string]interface{} `json:"transactions"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error" example:"Invalid request"`
}

// handleGetMyAccount godoc
// @Summary Get your own account details
// @Description Get account details including balance and transaction history. Requires cryptographically signed request with Ed25519 signature and X.509 certificate.
// @Success 200 {object} AccountResponse
// @Failure 400 {object} ErrorResponse "Invalid request body or missing signature"
// @Failure 403 {object} ErrorResponse "Attempting to access another account"
// @Failure 404 {object} ErrorResponse "Account not found"
// @Router /accounts/me [post]
func (t *Transport) handleGetMyAccount(w http.ResponseWriter, r *http.Request) {
	var req GetMyAccountRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.SignedRequest == nil {
		respondError(w, http.StatusBadRequest, "Signed envelope required")
		return
	}

	// Extract account ID from signed payload
	accountIDStr := req.SignedRequest.Payload.AccountID
	accountID, err := scalegraph.ScalegraphIdFromString(accountIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	// Verify the signer matches the requested account
	if req.SignedRequest.Signature.SignerID != accountIDStr {
		t.logger.Warn("Unauthorized access attempt",
			"requested_account", accountIDStr,
			"signer", req.SignedRequest.Signature.SignerID,
		)
		respondError(w, http.StatusForbidden, "Can only access your own account")
		return
	}

	acc, err := t.client.GetAccount(r.Context(), accountID, req.SignedRequest)
	if err != nil {
		t.logger.Error("Failed to get account", "error", err, "id", accountIDStr)
		respondError(w, http.StatusNotFound, "Account not found")
		return
	}

	// Get transaction history
	blocks := acc.Blockchain().GetBlocks()
	transactions := make([]map[string]interface{}, 0)
	for _, block := range blocks {
		if tx := block.Transaction(); tx != nil {
			fromID := "genesis"
			if sender := tx.Sender(); sender != nil {
				fromID = sender.ID().String()
			}
			toID := "unknown"
			if receiver := tx.Receiver(); receiver != nil {
				toID = receiver.ID().String()
			}

			var amount float64
			switch tx.Type() {
			case scalegraph.Mint:
				mintTx := tx.(*scalegraph.MintTransaction)
				amount = mintTx.Amount()
			case scalegraph.Transfer:
				transferTx := tx.(*scalegraph.TransferTransaction)
				amount = transferTx.Amount()
			case scalegraph.Burn:
				burnTx := tx.(*scalegraph.BurnTransaction)
				amount = burnTx.Amount()
			}

			transactions = append(transactions, map[string]interface{}{
				"type":   tx.Type().String(),
				"from":   fromID,
				"to":     toID,
				"amount": amount,
			})
		}
	}

	respondJSON(w, http.StatusOK, AccountResponse{
		ID:           acc.ID().String(),
		Balance:      acc.Balance(),
		Transactions: transactions,
	})
}

// SignedTransferRequest represents the incoming signed transfer request
type SignedTransferRequest struct {
	SignedEnvelope *crypto.SignedEnvelope[*crypto.TransferPayload] `json:"signed_envelope"`
}

// TransferResponse represents a successful transfer response
type TransferResponse struct {
	Status string `json:"status" example:"success"`
}

// handleTransfer godoc
// @Summary Transfer funds between accounts
// @Description Transfer funds from one account to another. Requires cryptographically signed request with Ed25519 signature.
// @Tags transfer
// @Accept json
// @Produce json
// @Param request body SignedTransferRequest true "Signed transfer request with Ed25519 signature"
// @Success 200 {object} TransferResponse
// @Failure 400 {object} ErrorResponse "Invalid request, missing fields, or transfer failed"
// @Router /transfer [post]
func (t *Transport) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req SignedTransferRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.SignedEnvelope == nil {
		respondError(w, http.StatusBadRequest, "Signed envelope required")
		return
	}

	// Extract IDs from the signed payload
	fromID, err := scalegraph.ScalegraphIdFromString(req.SignedEnvelope.Payload.From)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid from account ID")
		return
	}
	toID, err := scalegraph.ScalegraphIdFromString(req.SignedEnvelope.Payload.To)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid to account ID")
		return
	}

	// Server will verify signature and that signer matches from account
	if err := t.client.TransferSigned(r.Context(), fromID, toID, req.SignedEnvelope.Payload.Amount, req.SignedEnvelope); err != nil {
		t.logger.Error("Transfer failed", "error", err,
			"from", req.SignedEnvelope.Payload.From,
			"to", req.SignedEnvelope.Payload.To,
			"amount", req.SignedEnvelope.Payload.Amount)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, TransferResponse{
		Status: "success",
	})
}

// SignedMintTokenRequest represents the incoming signed mint token request
type SignedMintTokenRequest struct {
	SignedEnvelope *crypto.SignedEnvelope[*crypto.MintTokenPayload] `json:"signed_envelope"`
}

// handleMintToken godoc
// @Summary Mint a new token
// @Description Mint a new token for an account. Requires a cryptographically signed request with Ed25519 signature.
// @Tags tokens
// @Accept json
// @Produce json
// @Param request body SignedMintTokenRequest true "Signed mint token request"
// @Success 200 {object} TransferResponse
// @Failure 400 {object} ErrorResponse "Invalid request or mint failed"
// @Router /tokens/mint [post]
func (t *Transport) handleMintToken(w http.ResponseWriter, r *http.Request) {
	var req SignedMintTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.SignedEnvelope == nil {
		respondError(w, http.StatusBadRequest, "Signed envelope required")
		return
	}

	if _, err := t.client.MintTokenSigned(r.Context(), req.SignedEnvelope); err != nil {
		t.logger.Error("Mint token failed", "error", err,
			"signer", req.SignedEnvelope.Signature.SignerID,
			"token_value", req.SignedEnvelope.Payload.TokenValue)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, TransferResponse{Status: "success"})
}

// SignedAuthorizeTokenTransferRequest represents the incoming signed authorize token transfer request
type SignedAuthorizeTokenTransferRequest struct {
	SignedEnvelope *crypto.SignedEnvelope[*crypto.AuthorizeTokenTransferPayload] `json:"signed_envelope"`
}

// handleAuthorizeTokenTransfer godoc
// @Summary Authorize a token transfer
// @Description Authorize a token to be transferred from an account. Must be called before /tokens/transfer. Requires a cryptographically signed request.
// @Tags tokens
// @Accept json
// @Produce json
// @Param request body SignedAuthorizeTokenTransferRequest true "Signed authorize token transfer request"
// @Success 200 {object} TransferResponse
// @Failure 400 {object} ErrorResponse "Invalid request or authorization failed"
// @Router /tokens/authorize [post]
func (t *Transport) handleAuthorizeTokenTransfer(w http.ResponseWriter, r *http.Request) {
	var req SignedAuthorizeTokenTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.SignedEnvelope == nil {
		respondError(w, http.StatusBadRequest, "Signed envelope required")
		return
	}

	if _, err := t.client.AuthorizeTokenTransferSigned(r.Context(), req.SignedEnvelope); err != nil {
		t.logger.Error("Authorize token transfer failed", "error", err,
			"account_id", req.SignedEnvelope.Payload.AccountID,
			"token_id", req.SignedEnvelope.Payload.TokenID)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, TransferResponse{Status: "success"})
}

// SignedUnauthorizeTokenTransferRequest represents the incoming signed unauthorize token transfer request
type SignedUnauthorizeTokenTransferRequest struct {
	SignedEnvelope *crypto.SignedEnvelope[*crypto.UnauthorizeTokenTransferPayload] `json:"signed_envelope"`
}

// handleUnauthorizeTokenTransfer godoc
// @Summary Unauthorize a token transfer
// @Description Revoke a previously authorized token transfer from an account. Requires a cryptographically signed request.
// @Tags tokens
// @Accept json
// @Produce json
// @Param request body SignedUnauthorizeTokenTransferRequest true "Signed unauthorize token transfer request"
// @Success 200 {object} TransferResponse
// @Failure 400 {object} ErrorResponse "Invalid request or unauthorization failed"
// @Router /tokens/unauthorize [post]
func (t *Transport) handleUnauthorizeTokenTransfer(w http.ResponseWriter, r *http.Request) {
	var req SignedUnauthorizeTokenTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.SignedEnvelope == nil {
		respondError(w, http.StatusBadRequest, "Signed envelope required")
		return
	}

	if _, err := t.client.UnauthorizeTokenTransferSigned(r.Context(), req.SignedEnvelope); err != nil {
		t.logger.Error("Unauthorize token transfer failed", "error", err,
			"account_id", req.SignedEnvelope.Payload.AccountID,
			"token_id", req.SignedEnvelope.Payload.TokenID)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, TransferResponse{Status: "success"})
}

// SignedTransferTokenRequest represents the incoming signed transfer token request
type SignedTransferTokenRequest struct {
	SignedEnvelope *crypto.SignedEnvelope[*crypto.TransferTokenPayload] `json:"signed_envelope"`
}

// handleTransferToken godoc
// @Summary Transfer a token between accounts
// @Description Transfer a token from one account to another. The token must first be authorized for transfer using /tokens/authorize. Requires a cryptographically signed request.
// @Tags tokens
// @Accept json
// @Produce json
// @Param request body SignedTransferTokenRequest true "Signed transfer token request"
// @Success 200 {object} TransferResponse
// @Failure 400 {object} ErrorResponse "Invalid request or transfer failed"
// @Router /tokens/transfer [post]
func (t *Transport) handleTransferToken(w http.ResponseWriter, r *http.Request) {
	var req SignedTransferTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.SignedEnvelope == nil {
		respondError(w, http.StatusBadRequest, "Signed envelope required")
		return
	}

	if _, err := t.client.TransferTokenSigned(r.Context(), req.SignedEnvelope); err != nil {
		t.logger.Error("Transfer token failed", "error", err,
			"from", req.SignedEnvelope.Payload.From,
			"to", req.SignedEnvelope.Payload.To,
			"token_id", req.SignedEnvelope.Payload.TokenID)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, TransferResponse{Status: "success"})
}

// SignedBurnTokenRequest represents the incoming signed burn token request
type SignedBurnTokenRequest struct {
	SignedEnvelope *crypto.SignedEnvelope[*crypto.BurnTokenPayload] `json:"signed_envelope"`
}

// handleBurnToken godoc
// @Summary Burn a token
// @Description Permanently destroy a token owned by an account. Requires a cryptographically signed request.
// @Tags tokens
// @Accept json
// @Produce json
// @Param request body SignedBurnTokenRequest true "Signed burn token request"
// @Success 200 {object} TransferResponse
// @Failure 400 {object} ErrorResponse "Invalid request or burn failed"
// @Router /tokens/burn [post]
func (t *Transport) handleBurnToken(w http.ResponseWriter, r *http.Request) {
	var req SignedBurnTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.SignedEnvelope == nil {
		respondError(w, http.StatusBadRequest, "Signed envelope required")
		return
	}

	if _, err := t.client.BurnTokenSigned(r.Context(), req.SignedEnvelope); err != nil {
		t.logger.Error("Burn token failed", "error", err,
			"account_id", req.SignedEnvelope.Payload.AccountID,
			"token_id", req.SignedEnvelope.Payload.TokenID)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, TransferResponse{Status: "success"})
}

// SignedClawbackTokenRequest represents the incoming signed clawback token request
type SignedClawbackTokenRequest struct {
	SignedEnvelope *crypto.SignedEnvelope[*crypto.ClawbackTokenPayload] `json:"signed_envelope"`
}

// handleClawbackToken godoc
// @Summary Clawback a token
// @Description Reclaim a token from an account using clawback authority. Must be signed by the clawback authority account (To). Requires a cryptographically signed request.
// @Tags tokens
// @Accept json
// @Produce json
// @Param request body SignedClawbackTokenRequest true "Signed clawback token request"
// @Success 200 {object} TransferResponse
// @Failure 400 {object} ErrorResponse "Invalid request or clawback failed"
// @Router /tokens/clawback [post]
func (t *Transport) handleClawbackToken(w http.ResponseWriter, r *http.Request) {
	var req SignedClawbackTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.SignedEnvelope == nil {
		respondError(w, http.StatusBadRequest, "Signed envelope required")
		return
	}

	if _, err := t.client.ClawbackTokenSigned(r.Context(), req.SignedEnvelope); err != nil {
		t.logger.Error("Clawback token failed", "error", err,
			"from", req.SignedEnvelope.Payload.From,
			"to", req.SignedEnvelope.Payload.To,
			"token_id", req.SignedEnvelope.Payload.TokenID)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, TransferResponse{Status: "success"})
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes a JSON error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{
		Error: message,
	})
}
