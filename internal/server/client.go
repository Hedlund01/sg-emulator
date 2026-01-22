package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"sg-emulator/internal/scalegraph"
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
	requestChan  chan<- Request
	responseChan chan Response
	timeout      time.Duration
	logger       *slog.Logger
}

// NewClient creates a new Client that sends requests to the given channel
func NewClient(requestChan chan<- Request, logger *slog.Logger) *Client {
	return &Client{
		requestChan:  requestChan,
		responseChan: make(chan Response, 10),
		timeout:      30 * time.Second,
		logger:       logger,
	}
}

// SetTimeout sets the timeout for request/response operations
func (c *Client) SetTimeout(d time.Duration) {
	c.timeout = d
}

// sendRequest sends a request and waits for the response
func (c *Client) sendRequest(ctx context.Context, reqType RequestType, payload any) (Response, error) {
	traceID := trace.GetTraceID(ctx)
	req := Request{
		ID:           generateRequestID(),
		Type:         reqType,
		ResponseChan: c.responseChan,
		Payload:      payload,
		Context:      ctx,
	}

	// Send request
	select {
	case c.requestChan <- req:
		// Request sent successfully
	case <-time.After(c.timeout):
		return Response{}, errors.New("request send timeout")
	}

	// Wait for response
	select {
	case resp := <-c.responseChan:
		return resp, nil
	case <-time.After(c.timeout):
		if traceID != "" {
			c.logger.Error("Response timeout", "trace_id", traceID)
		}
		return Response{}, errors.New("response timeout")
	}
}

// CreateAccount creates a new account with an optional initial balance
func (c *Client) CreateAccount(ctx context.Context, initialBalance float64) (*scalegraph.Account, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"initial_balance", initialBalance}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Creating account", logAttrs...)
	resp, err := c.sendRequest(ctx, ReqCreateAccount, CreateAccountPayload{
		InitialBalance: initialBalance,
	})
	if err != nil {
		logAttrs = append(logAttrs, "error", err)
		c.logger.Error("Failed to create account", logAttrs...)
		return nil, err
	}
	if !resp.Success {
		logAttrs = append(logAttrs, "error", resp.Error)
		c.logger.Error("Account creation failed", logAttrs...)
		return nil, errors.New(resp.Error)
	}
	account := resp.Payload.(CreateAccountResponse).Account
	logAttrs = append([]any{"account_id", account.ID(), "balance", account.Balance()}, logAttrs...)
	c.logger.Info("Account created", logAttrs...)
	return account, nil
}

// GetAccount retrieves an account by ID
func (c *Client) GetAccount(ctx context.Context, id scalegraph.ScalegraphId) (*scalegraph.Account, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"account_id", id}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Getting account", logAttrs...)
	resp, err := c.sendRequest(ctx, ReqGetAccount, GetAccountPayload{ID: id})
	if err != nil {
		logAttrs = append(logAttrs, "error", err)
		c.logger.Error("Failed to get account", logAttrs...)
		return nil, err
	}
	if !resp.Success {
		logAttrs = append(logAttrs, "error", resp.Error)
		c.logger.Warn("Account not found", logAttrs...)
		return nil, errors.New(resp.Error)
	}
	return resp.Payload.(GetAccountResponse).Account, nil
}

// GetAccounts retrieves all accounts
func (c *Client) GetAccounts(ctx context.Context) ([]*scalegraph.Account, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Getting all accounts", logAttrs...)
	resp, err := c.sendRequest(ctx, ReqGetAccounts, GetAccountsPayload{})
	if err != nil {
		logAttrs = append(logAttrs, "error", err)
		c.logger.Error("Failed to get accounts", logAttrs...)
		return nil, err
	}
	if !resp.Success {
		logAttrs = append(logAttrs, "error", resp.Error)
		c.logger.Error("Get accounts failed", logAttrs...)
		return nil, errors.New(resp.Error)
	}
	accounts := resp.Payload.(GetAccountsResponse).Accounts
	logAttrs = append([]any{"count", len(accounts)}, logAttrs...)
	c.logger.Debug("Retrieved accounts", logAttrs...)
	return accounts, nil
}

// Transfer transfers funds between two accounts
func (c *Client) Transfer(ctx context.Context, from, to scalegraph.ScalegraphId, amount float64) error {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"from", from, "to", to, "amount", amount}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Transfer requested", logAttrs...)
	resp, err := c.sendRequest(ctx, ReqTransfer, TransferPayload{
		From:   from,
		To:     to,
		Amount: amount,
	})
	if err != nil {
		logAttrs = append(logAttrs, "error", err)
		c.logger.Error("Transfer failed", logAttrs...)
		return err
	}
	if !resp.Success {
		logAttrs = append(logAttrs, "error", resp.Error)
		c.logger.Warn("Transfer rejected", logAttrs...)
		return errors.New(resp.Error)
	}
	c.logger.Info("Transfer completed", logAttrs...)
	return nil
}

// Mint creates new funds in an account
func (c *Client) Mint(ctx context.Context, to scalegraph.ScalegraphId, amount float64) error {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{"account_id", to, "amount", amount}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Mint requested", logAttrs...)
	resp, err := c.sendRequest(ctx, ReqMint, MintPayload{
		To:     to,
		Amount: amount,
	})
	if err != nil {
		logAttrs = append(logAttrs, "error", err)
		c.logger.Error("Mint failed", logAttrs...)
		return err
	}
	if !resp.Success {
		logAttrs = append(logAttrs, "error", resp.Error)
		c.logger.Warn("Mint rejected", logAttrs...)
		return errors.New(resp.Error)
	}
	c.logger.Info("Mint completed", logAttrs...)
	return nil
}

// AccountCount returns the total number of accounts
func (c *Client) AccountCount(ctx context.Context) (int, error) {
	traceID := trace.GetTraceID(ctx)
	logAttrs := []any{}
	if traceID != "" {
		logAttrs = append(logAttrs, "trace_id", traceID)
	}
	c.logger.Debug("Getting account count", logAttrs...)
	resp, err := c.sendRequest(ctx, ReqAccountCount, AccountCountPayload{})
	if err != nil {
		logAttrs = append(logAttrs, "error", err)
		c.logger.Error("Failed to get account count", logAttrs...)
		return 0, err
	}
	if !resp.Success {
		logAttrs = append(logAttrs, "error", resp.Error)
		c.logger.Error("Account count failed", logAttrs...)
		return 0, errors.New(resp.Error)
	}
	count := resp.Payload.(AccountCountResponse).Count
	logAttrs = append([]any{"count", count}, logAttrs...)
	c.logger.Debug("Account count retrieved", logAttrs...)
	return count, nil
}
