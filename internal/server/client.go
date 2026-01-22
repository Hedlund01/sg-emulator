package server

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"sg-emulator/internal/scalegraph"
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
}

// NewClient creates a new Client that sends requests to the given channel
func NewClient(requestChan chan<- Request) *Client {
	return &Client{
		requestChan:  requestChan,
		responseChan: make(chan Response, 10),
		timeout:      30 * time.Second,
	}
}

// SetTimeout sets the timeout for request/response operations
func (c *Client) SetTimeout(d time.Duration) {
	c.timeout = d
}

// sendRequest sends a request and waits for the response
func (c *Client) sendRequest(reqType RequestType, payload any) (Response, error) {
	req := Request{
		ID:           generateRequestID(),
		Type:         reqType,
		ResponseChan: c.responseChan,
		Payload:      payload,
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
		return Response{}, errors.New("response timeout")
	}
}

// CreateAccount creates a new account with an optional initial balance
func (c *Client) CreateAccount(initialBalance float64) (*scalegraph.Account, error) {
	resp, err := c.sendRequest(ReqCreateAccount, CreateAccountPayload{
		InitialBalance: initialBalance,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}
	return resp.Payload.(CreateAccountResponse).Account, nil
}

// GetAccount retrieves an account by ID
func (c *Client) GetAccount(id scalegraph.ScalegraphId) (*scalegraph.Account, error) {
	resp, err := c.sendRequest(ReqGetAccount, GetAccountPayload{ID: id})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}
	return resp.Payload.(GetAccountResponse).Account, nil
}

// GetAccounts retrieves all accounts
func (c *Client) GetAccounts() ([]*scalegraph.Account, error) {
	resp, err := c.sendRequest(ReqGetAccounts, GetAccountsPayload{})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}
	return resp.Payload.(GetAccountsResponse).Accounts, nil
}

// Transfer transfers funds between two accounts
func (c *Client) Transfer(from, to scalegraph.ScalegraphId, amount float64) error {
	resp, err := c.sendRequest(ReqTransfer, TransferPayload{
		From:   from,
		To:     to,
		Amount: amount,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}

// Mint creates new funds in an account
func (c *Client) Mint(to scalegraph.ScalegraphId, amount float64) error {
	resp, err := c.sendRequest(ReqMint, MintPayload{
		To:     to,
		Amount: amount,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}

// AccountCount returns the total number of accounts
func (c *Client) AccountCount() (int, error) {
	resp, err := c.sendRequest(ReqAccountCount, AccountCountPayload{})
	if err != nil {
		return 0, err
	}
	if !resp.Success {
		return 0, errors.New(resp.Error)
	}
	return resp.Payload.(AccountCountResponse).Count, nil
}
