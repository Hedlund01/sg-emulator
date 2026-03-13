package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server/messages"
	"sg-emulator/internal/trace"
)

// requestIDCounter for generating unique request IDs
var requestIDCounter uint64

// generateRequestID creates a unique request ID
func generateRequestID() string {
	id := atomic.AddUint64(&requestIDCounter, 1)
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), id)
}

// Client communicates with a Server via channels.
// It provides the same API as scalegraph.App but sends requests through channels.
type Client struct {
	requestChan     chan<- messages.Request
	pendingRequests map[string]chan messages.Response
	mu              sync.Mutex
	timeout         time.Duration
	logger          *slog.Logger
}

// NewClient creates a new Client that sends requests to the given channel
func NewClient(requestChan chan<- messages.Request, logger *slog.Logger) *Client {
	return &Client{
		requestChan:     requestChan,
		pendingRequests: make(map[string]chan messages.Response),
		timeout:         30 * time.Second,
		logger:          logger,
	}
}

// SetTimeout sets the timeout for request/response operations
func (c *Client) SetTimeout(d time.Duration) {
	c.timeout = d
}

// Send is the generic, type-safe way to send requests.
func Send[Req, Resp any](c *Client, ctx context.Context, req *Req) (*Resp, error) {
	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	result, ok := resp.Payload.(*Resp)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", resp.Payload)
	}
	return result, nil
}

// sendRequest sends a request and waits for the response
func (c *Client) sendRequest(ctx context.Context, payload any) (messages.Response, error) {
	traceID := trace.GetTraceID(ctx)
	reqID := generateRequestID()

	// Create a unique response channel for this request
	respChan := make(chan messages.Response, 1)

	// Register the response channel
	c.mu.Lock()
	c.pendingRequests[reqID] = respChan
	c.mu.Unlock()

	// Ensure cleanup of pending request on exit
	defer func() {
		c.mu.Lock()
		delete(c.pendingRequests, reqID)
		c.mu.Unlock()
		close(respChan)
	}()

	req := messages.Request{
		ID:           reqID,
		ResponseChan: respChan,
		Payload:      payload,
		Context:      ctx,
	}

	// Send request
	select {
	case c.requestChan <- req:
		// Request sent successfully
	case <-ctx.Done():
		return messages.Response{}, ctx.Err()
	case <-time.After(c.timeout):
		return messages.Response{}, errors.New("request send timeout")
	}

	// Wait for response
	select {
	case resp := <-respChan:
		return resp, nil
	case <-ctx.Done():
		if traceID != "" {
			c.logger.Error("Request cancelled", "trace_id", traceID)
		}
		return messages.Response{}, ctx.Err()
	case <-time.After(c.timeout):
		if traceID != "" {
			c.logger.Error("Response timeout", "trace_id", traceID)
		}
		return messages.Response{}, errors.New("response timeout")
	}
}

// CreateAccountWithCredentials creates a new account and returns the full response
// including the certificate and private key
func (c *Client) CreateAccountWithCredentials(ctx context.Context, initialBalance float64, signedReq *crypto.SignedEnvelope[*crypto.CreateAccountPayload]) (*scalegraph.CreateAccountResponse, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"initial_balance", initialBalance}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Creating account with credentials", logAttrs...)

	return Send[scalegraph.CreateAccountRequest, scalegraph.CreateAccountResponse](c, ctx, &scalegraph.CreateAccountRequest{
		InitialBalance: initialBalance,
		SignedEnvelope: signedReq,
	})
}

// GetAccount retrieves an account by ID
func (c *Client) GetAccount(ctx context.Context, id scalegraph.ScalegraphId, signedReq *crypto.SignedEnvelope[*crypto.GetAccountPayload]) (*scalegraph.Account, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"account_id", id}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Getting account", logAttrs...)

	resp, err := Send[scalegraph.GetAccountRequest, scalegraph.GetAccountResponse](c, ctx, &scalegraph.GetAccountRequest{
		AccountID:      id,
		SignedEnvelope: signedReq,
	})
	if err != nil {
		return nil, err
	}
	return resp.Account, nil
}

// GetAccounts retrieves all accounts
func (c *Client) GetAccounts(ctx context.Context) ([]*scalegraph.Account, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Getting all accounts", logAttrs...)

	resp, err := Send[scalegraph.GetAccountsRequest, scalegraph.GetAccountsResponse](c, ctx, &scalegraph.GetAccountsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Accounts, nil
}

// TransferSigned transfers funds with a cryptographically signed request
func (c *Client) TransferSigned(ctx context.Context, signedRequest *crypto.SignedEnvelope[*crypto.TransferPayload]) (*scalegraph.TransferResponse, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"from", signedRequest.Payload.From, "to", signedRequest.Payload.To, "amount", signedRequest.Payload.Amount, "signed", true}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Signed transfer requested", logAttrs...)

	from, err := scalegraph.ScalegraphIdFromString(signedRequest.Payload.From)
	if err != nil {
		c.logger.Error("Invalid from account ID in signed request", "error", err, "from_account_id", signedRequest.Payload.From)
		return nil, err
	}
	to, err := scalegraph.ScalegraphIdFromString(signedRequest.Payload.To)
	if err != nil {
		c.logger.Error("Invalid to account ID in signed request", "error", err, "to_account_id", signedRequest.Payload.To)
		return nil, err
	}

	return Send[scalegraph.TransferRequest, scalegraph.TransferResponse](c, ctx, &scalegraph.TransferRequest{
		From:           from,
		To:             to,
		Amount:         signedRequest.Payload.Amount,
		Nonce:          signedRequest.Payload.Nonce,
		SignedEnvelope: signedRequest,
	})
}

// Mint creates new funds in an account
func (c *Client) Mint(ctx context.Context, to scalegraph.ScalegraphId, amount float64) error {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"account_id", to, "amount", amount}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Mint requested", logAttrs...)

	_, err := Send[scalegraph.MintRequest, scalegraph.MintResponse](c, ctx, &scalegraph.MintRequest{
		To:     to,
		Amount: amount,
	})
	return err
}

// AccountCount returns the total number of accounts
func (c *Client) AccountCount(ctx context.Context) (int, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Getting account count", logAttrs...)

	resp, err := Send[scalegraph.AccountCountRequest, scalegraph.AccountCountResponse](c, ctx, &scalegraph.AccountCountRequest{})
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (c *Client) MintTokenSigned(ctx context.Context, signedReq *crypto.SignedEnvelope[*crypto.MintTokenPayload]) (*scalegraph.MintTokenResponse, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"account_id", signedReq.Signature.SignerID, "token_id", string(signedReq.Signature.Value), "signed", true}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Mint token requested", logAttrs...)
	var clawBackAddr *scalegraph.ScalegraphId = nil
	if signedReq.Payload.ClawbackAddress != nil {
		addr, err := scalegraph.ScalegraphIdFromString(*signedReq.Payload.ClawbackAddress)
		if err != nil {
			c.logger.Error("Invalid clawback address", "error", err, "clawback_address", *signedReq.Payload.ClawbackAddress)
			return nil, err
		}
		clawBackAddr = &addr
	}
	logAttrs = append(logAttrs, "clawback_address", clawBackAddr)

	return Send[scalegraph.MintTokenRequest, scalegraph.MintTokenResponse](c, ctx, &scalegraph.MintTokenRequest{
		TokenValue:      signedReq.Payload.TokenValue,
		ClawbackAddress: clawBackAddr,
		SignedEnvelope:  signedReq,
	})
}

func (c *Client) AuthorizeTokenTransferSigned(ctx context.Context, signedReq *crypto.SignedEnvelope[*crypto.AuthorizeTokenTransferPayload]) (*scalegraph.AuthorizeTokenTransferResponse, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"account_id", signedReq.Payload.AccountID, "token_id", signedReq.Payload.TokenID, "signed", true}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Authorize token transfer requested", logAttrs...)

	acc, err := scalegraph.ScalegraphIdFromString(signedReq.Payload.AccountID)
	if err != nil {
		c.logger.Error("Invalid account ID in signed request", "error", err, "account_id", signedReq.Signature.SignerID)
		return nil, err
	}

	return Send[scalegraph.AuthorizeTokenTransferRequest, scalegraph.AuthorizeTokenTransferResponse](c, ctx, &scalegraph.AuthorizeTokenTransferRequest{
		AccountID:      acc,
		TokenId:        signedReq.Payload.TokenID,
		SignedEnvelope: signedReq,
	})
}

func (c *Client) UnauthorizeTokenTransferSigned(ctx context.Context, signedReq *crypto.SignedEnvelope[*crypto.UnauthorizeTokenTransferPayload]) (*scalegraph.UnauthorizeTokenTransferResponse, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"account_id", signedReq.Payload.AccountID, "token_id", signedReq.Payload.TokenID, "signed", true}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Unauthorize token transfer requested", logAttrs...)

	acc, err := scalegraph.ScalegraphIdFromString(signedReq.Payload.AccountID)
	if err != nil {
		c.logger.Error("Invalid account ID in signed request", "error", err, "account_id", signedReq.Signature.SignerID)
		return nil, err
	}

	return Send[scalegraph.UnauthorizeTokenTransferRequest, scalegraph.UnauthorizeTokenTransferResponse](c, ctx, &scalegraph.UnauthorizeTokenTransferRequest{
		AccountID:      acc,
		TokenId:        signedReq.Payload.TokenID,
		SignedEnvelope: signedReq,
	})
}

func (c *Client) TransferTokenSigned(ctx context.Context, signedReq *crypto.SignedEnvelope[*crypto.TransferTokenPayload]) (*scalegraph.TransferTokenResponse, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"from", signedReq.Payload.From, "to", signedReq.Payload.To, "token_id", signedReq.Payload.TokenID, "signed", true}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Transfer token requested", logAttrs...)

	fromAcc, err := scalegraph.ScalegraphIdFromString(signedReq.Payload.From)
	if err != nil {
		c.logger.Error("Invalid from account ID in signed request", "error", err, "from_account_id", signedReq.Signature.SignerID)
		return nil, err
	}
	toAcc, err := scalegraph.ScalegraphIdFromString(signedReq.Payload.To)
	if err != nil {
		c.logger.Error("Invalid to account ID in signed request", "error", err, "to_account_id", signedReq.Signature.SignerID)
		return nil, err
	}

	return Send[scalegraph.TransferTokenRequest, scalegraph.TransferTokenResponse](c, ctx, &scalegraph.TransferTokenRequest{
		From:           fromAcc,
		To:             toAcc,
		TokenId:        signedReq.Payload.TokenID,
		SignedEnvelope: signedReq,
	})
}

func (c *Client) BurnTokenSigned(ctx context.Context, signedReq *crypto.SignedEnvelope[*crypto.BurnTokenPayload]) (*scalegraph.BurnTokenResponse, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"account_id", signedReq.Payload.AccountID, "token_id", signedReq.Payload.TokenID, "signed", true}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Burn token requested", logAttrs...)

	acc, err := scalegraph.ScalegraphIdFromString(signedReq.Payload.AccountID)
	if err != nil {
		c.logger.Error("Invalid account ID in signed request", "error", err, "account_id", signedReq.Signature.SignerID)
		return nil, err
	}

	return Send[scalegraph.BurnTokenRequest, scalegraph.BurnTokenResponse](c, ctx, &scalegraph.BurnTokenRequest{
		AccountID:      acc,
		TokenId:        signedReq.Payload.TokenID,
		SignedEnvelope: signedReq,
	})
}

// AdminCreateAccount creates an account without requiring a signed request.
// Access is controlled at the transport layer via a flag.
func (c *Client) AdminCreateAccount(ctx context.Context, initialBalance float64) (*scalegraph.CreateAccountResponse, error) {
	return Send[scalegraph.AdminCreateAccountRequest, scalegraph.CreateAccountResponse](c, ctx,
		&scalegraph.AdminCreateAccountRequest{InitialBalance: initialBalance})
}

// AdminMint mints funds into an account without requiring a signed request.
// Access is controlled at the transport layer via a flag.
func (c *Client) AdminMint(ctx context.Context, to scalegraph.ScalegraphId, amount float64) error {
	_, err := Send[scalegraph.AdminMintRequest, scalegraph.AdminMintResponse](c, ctx,
		&scalegraph.AdminMintRequest{To: to, Amount: amount})
	return err
}

func (c *Client) LookupTokenSigned(ctx context.Context, signedReq *crypto.SignedEnvelope[*crypto.LookupTokenPayload]) (*scalegraph.LookupTokenResponse, error) {
	acc, err := scalegraph.ScalegraphIdFromString(signedReq.Payload.AccountID)
	if err != nil {
		return nil, err
	}
	return Send[scalegraph.LookupTokenRequest, scalegraph.LookupTokenResponse](c, ctx, &scalegraph.LookupTokenRequest{
		TokenID:        signedReq.Payload.TokenID,
		AccountID:      acc,
		SignedEnvelope: signedReq,
	})
}

func (c *Client) ClawbackTokenSigned(ctx context.Context, signedReq *crypto.SignedEnvelope[*crypto.ClawbackTokenPayload]) (*scalegraph.ClawbackTokenResponse, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"from", signedReq.Payload.From, "to", signedReq.Payload.To, "token_id", signedReq.Payload.TokenID, "signed", true}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Clawback token requested", logAttrs...)

	fromAcc, err := scalegraph.ScalegraphIdFromString(signedReq.Payload.From)
	if err != nil {
		c.logger.Error("Invalid from account ID in signed request", "error", err, "from_account_id", signedReq.Signature.SignerID)
		return nil, err
	}
	toAcc, err := scalegraph.ScalegraphIdFromString(signedReq.Payload.To)
	if err != nil {
		c.logger.Error("Invalid to account ID in signed request", "error", err, "to_account_id", signedReq.Signature.SignerID)
		return nil, err
	}

	return Send[scalegraph.ClawbackTokenRequest, scalegraph.ClawbackTokenResponse](c, ctx, &scalegraph.ClawbackTokenRequest{
		From:           fromAcc,
		To:             toAcc,
		TokenId:        signedReq.Payload.TokenID,
		SignedEnvelope: signedReq,
	})
}
