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
	"sg-emulator/internal/trace"
	"sg-emulator/internal/server/messages"
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

// Transfer transfers funds with a cryptographically signed request
func (c *Client) Transfer(ctx context.Context, req *scalegraph.TransferRequest) (*scalegraph.TransferResponse, error) {
	return Send[scalegraph.TransferRequest, scalegraph.TransferResponse](c, ctx, req)
}

// TransferSigned transfers funds with a cryptographically signed request (convenience wrapper)
func (c *Client) TransferSigned(ctx context.Context, from, to scalegraph.ScalegraphId, amount float64, signedRequest *crypto.SignedEnvelope[*crypto.TransferPayload]) error {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"from", from, "to", to, "amount", amount, "signed", true}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Signed transfer requested", logAttrs...)

	_, err := Send[scalegraph.TransferRequest, scalegraph.TransferResponse](c, ctx, &scalegraph.TransferRequest{
		From:           from,
		To:             to,
		Amount:         amount,
		Nonce:          signedRequest.Payload.Nonce,
		SignedEnvelope: signedRequest,
	})
	return err
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
