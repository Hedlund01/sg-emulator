package mcp

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"sg-emulator/internal/crypto"
	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateAccountArgs represents arguments for create_account tool
type CreateAccountArgs struct {
	Balance float64 `json:"balance" jsonschema:"Initial balance for the account (default: 0)"`
}

// GetAccountArgs represents arguments for get_account tool
type GetAccountArgs struct {
	ID string `json:"id" jsonschema:"The account ID (hex string)"`
}

// TransferArgs represents arguments for transfer tool
type TransferArgs struct {
	From   string  `json:"from" jsonschema:"Source account ID (hex string)"`
	To     string  `json:"to" jsonschema:"Destination account ID (hex string)"`
	Amount float64 `json:"amount" jsonschema:"Amount to transfer"`
}

// MintArgs represents arguments for mint tool
type MintArgs struct {
	To     string  `json:"to" jsonschema:"Account ID to mint tokens to (hex string)"`
	Amount float64 `json:"amount" jsonschema:"Amount to mint"`
}

// MintTokenArgs represents arguments for mint_token tool
type MintTokenArgs struct {
	AccountID       string `json:"account_id" jsonschema:"Account ID of the token minter (hex string)"`
	TokenValue      string `json:"token_value" jsonschema:"The value/name of the token to mint"`
	ClawbackAddress string `json:"clawback_address,omitempty" jsonschema:"Optional clawback address (hex string)"`
	FreezeAddress   string `json:"freeze_address,omitempty" jsonschema:"Optional freeze address (hex string)"`
}

// AuthorizeTokenTransferArgs represents arguments for authorize_token_transfer tool
type AuthorizeTokenTransferArgs struct {
	AccountID    string `json:"account_id" jsonschema:"Account ID of the future token receiver that is authorizing the transfer (hex string)"`
	TokenOwnerID string `json:"token_owner_id" jsonschema:"Account ID of the current token owner (hex string)"`
	TokenID      string `json:"token_id" jsonschema:"The token ID to authorize for transfer"`
}

// UnauthorizeTokenTransferArgs represents arguments for unauthorize_token_transfer tool
type UnauthorizeTokenTransferArgs struct {
	AccountID    string `json:"account_id" jsonschema:"Account ID of the account revoking the authorization (hex string)"`
	TokenOwnerID string `json:"token_owner_id" jsonschema:"Account ID of the current token owner (hex string)"`
	TokenID      string `json:"token_id" jsonschema:"The token ID to unauthorize for transfer"`
}

// TransferTokenArgs represents arguments for transfer_token tool
type TransferTokenArgs struct {
	FromAccountID string `json:"from_account_id" jsonschema:"Source account ID (hex string)"`
	ToAccountID   string `json:"to_account_id" jsonschema:"Destination account ID (hex string)"`
	TokenID       string `json:"token_id" jsonschema:"The token ID to transfer"`
}

// BurnTokenArgs represents arguments for burn_token tool
type BurnTokenArgs struct {
	AccountID string `json:"account_id" jsonschema:"Account ID of the token owner (hex string)"`
	TokenID   string `json:"token_id" jsonschema:"The token ID to burn"`
}

// ClawbackTokenArgs represents arguments for clawback_token tool
type ClawbackTokenArgs struct {
	FromAccountID string `json:"from_account_id" jsonschema:"Account ID holding the token (hex string)"`
	ToAccountID   string `json:"to_account_id" jsonschema:"Clawback authority account ID that signs and receives the token (hex string)"`
	TokenID       string `json:"token_id" jsonschema:"The token ID to clawback"`
}

// FreezeTokenArgs represents arguments for freeze_token tool
type FreezeTokenArgs struct {
	FreezeAuthorityID string `json:"freeze_authority_id" jsonschema:"Freeze authority account ID that signs the request (hex string)"`
	TokenHolderID     string `json:"token_holder_id" jsonschema:"Account ID of the token holder (hex string)"`
	TokenID           string `json:"token_id" jsonschema:"The token ID to freeze"`
}

// UnfreezeTokenArgs represents arguments for unfreeze_token tool
type UnfreezeTokenArgs struct {
	FreezeAuthorityID string `json:"freeze_authority_id" jsonschema:"Freeze authority account ID that signs the request (hex string)"`
	TokenHolderID     string `json:"token_holder_id" jsonschema:"Account ID of the token holder (hex string)"`
	TokenID           string `json:"token_id" jsonschema:"The token ID to unfreeze"`
}

// LookupTokenArgs represents arguments for lookup_token tool
type LookupTokenArgs struct {
	AccountID string `json:"account_id" jsonschema:"Account ID to look up the token in (hex string)"`
	TokenID   string `json:"token_id" jsonschema:"The token ID to look up"`
}

// AdminCreateAccountArgs represents arguments for admin_create_account tool
type AdminCreateAccountArgs struct {
	Balance float64 `json:"balance" jsonschema:"Initial balance for the account (default: 0)"`
}

// AdminMintArgs represents arguments for admin_mint tool
type AdminMintArgs struct {
	To     string  `json:"to" jsonschema:"Account ID to mint tokens to (hex string)"`
	Amount float64 `json:"amount" jsonschema:"Amount to mint"`
}

// CreateSignedRequestArgs represents arguments for create_signed_request tool
type CreateSignedRequestArgs struct {
	Type            string  `json:"type" jsonschema:"Type of request: 'transfer', 'get_account', 'mint_token', 'authorize_token_transfer', 'unauthorize_token_transfer', 'transfer_token', 'burn_token', 'clawback_token', 'freeze_token', 'unfreeze_token', or 'lookup_token'"`
	AccountID       string  `json:"account_id" jsonschema:"Account ID that will sign the request (must have credentials)"`
	ToID            string  `json:"to_id,omitempty" jsonschema:"Destination account ID (for transfer only)"`
	Amount          float64 `json:"amount,omitempty" jsonschema:"Amount to transfer (for transfer only)"`
	TokenValue      string  `json:"token_value,omitempty" jsonschema:"Token value/name (for mint_token only)"`
	ClawbackAddress string  `json:"clawback_address,omitempty" jsonschema:"Optional clawback address hex string (for mint_token only)"`
	FreezeAddress   string  `json:"freeze_address,omitempty" jsonschema:"Optional freeze address hex string (for mint_token only)"`
	TokenID         string  `json:"token_id,omitempty" jsonschema:"Token ID (for authorize_token_transfer, transfer_token, burn_token, clawback_token, freeze_token, and unfreeze_token)"`
	ToAccountID     string  `json:"to_account_id,omitempty" jsonschema:"Destination account ID (for transfer_token and clawback_token)"`
	FromAccountID   string  `json:"from_account_id,omitempty" jsonschema:"Source account holding the token (for clawback_token only)"`
	TokenOwnerID    string  `json:"token_owner_id,omitempty" jsonschema:"Current token owner account ID (for authorize_token_transfer and unauthorize_token_transfer)"`
	TokenHolderID   string  `json:"token_holder_id,omitempty" jsonschema:"Token holder account ID (for freeze_token and unfreeze_token)"`
}

// RunHTTPServer starts an MCP server over HTTP with SSE transport.
func RunHTTPServer(ctx context.Context, addr string, client *server.Client, srv *server.Server, logger *slog.Logger) error {
	logger.Info("MCP server starting", "address", addr)

	// Create MCP server factory - returns the same server for all requests
	getServer := func(req *http.Request) *mcp.Server {
		return createServer(client, srv, logger)
	}

	handler := mcp.NewSSEHandler(getServer, nil)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		logger.Info("MCP server shutting down")
		httpServer.Shutdown(context.Background())
	}()

	baseURL := "http://" + addr
	logger.Info("MCP server listening",
		"address", addr,
		"transport", "SSE",
		"endpoint", baseURL,
		"info", "Configure MCP clients to use: "+baseURL)

	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		logger.Error("MCP server error", "error", err)
	}
	return err
}

func createServer(client *server.Client, srv *server.Server, logger *slog.Logger) *mcp.Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "scalegraph-mcp",
		Version: "0.1.0",
	}, nil)

	registerTools(mcpServer, client, srv)
	return mcpServer
}

// createSignedEnvelope is a generic helper that retrieves credentials and creates a signed envelope.
func createSignedEnvelope[T crypto.SignableData](srv *server.Server, accountID string, payload T) (*crypto.SignedEnvelope[T], error) {
	// Get the CA from server
	ca := srv.CA()
	if ca == nil {
		return nil, fmt.Errorf("Certificate Authority not available")
	}

	var privKey ed25519.PrivateKey
	var certPEM string

	// Check if signing as the CA's system account
	systemAccountID := scalegraph.ScalegraphIdFromPublicKey(ca.PublicKey())
	if accountID == systemAccountID.String() {
		// Use CA's own credentials directly
		privKey = ca.PrivateKey()
		certPEM = ca.CertificatePEM()
	} else {
		// Retrieve account credentials from the store
		privKeyPEM, err := ca.GetAccountPrivateKeyPEM(accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get private key for account %s: %v (account may not have credentials)", accountID, err)
		}

		certPEM, err = ca.GetAccountCertificatePEM(accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get certificate for account %s: %v", accountID, err)
		}

		privKey, err = crypto.DecodePrivateKeyPEM([]byte(privKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("failed to decode private key: %v", err)
		}
	}

	// Create signed envelope
	signedEnvelope, err := crypto.CreateSignedEnvelope(payload, privKey, accountID, certPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create signed envelope: %v", err)
	}

	return signedEnvelope, nil
}

func registerTools(mcpServer *mcp.Server, client *server.Client, srv *server.Server) {
	// create_account tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "create_account",
		Description: "Create a new account with an optional initial balance. Returns account ID, balance, certificate, and private key location.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateAccountArgs) (*mcp.CallToolResult, any, error) {
		createReq := &crypto.CreateAccountPayload{
			InitialBalance: args.Balance,
		}

		systemAccountID := scalegraph.ScalegraphIdFromPublicKey(srv.CA().PublicKey())

		signedEnvelope, err := createSignedEnvelope(srv, systemAccountID.String(), createReq)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		resp, err := client.CreateAccountWithCredentials(context.Background(), args.Balance, signedEnvelope)
		if err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Created account %s with balance %.2f\n", resp.Account.ID().String(), resp.Account.Balance())
		if resp.Certificate != "" {
			text += fmt.Sprintf("\nCertificate (PEM):\n%s\n", resp.Certificate)
		}
		if resp.PrivateKey != "" {
			text += fmt.Sprintf("\nPrivate Key (PEM):\n%s", resp.PrivateKey)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// get_accounts tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "get_accounts",
		Description: "List all accounts in the scalegraph with their IDs and balances",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
		accounts, err := client.GetAccounts(context.Background())
		if err != nil {
			return nil, nil, err
		}

		if len(accounts) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No accounts found"}},
			}, nil, nil
		}

		text := fmt.Sprintf("Total accounts: %d\n\n", len(accounts))
		for i, acc := range accounts {
			text += fmt.Sprintf("%d. ID: %s\n   Balance: %.2f\n\n", i+1, acc.ID().String(), acc.Balance())
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// get_account tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "get_account",
		Description: "Get details of a specific account by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetAccountArgs) (*mcp.CallToolResult, any, error) {
		id, err := scalegraph.ScalegraphIdFromString(args.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid account ID: %v", err)
		}

		signedReq, err := createSignedEnvelope(srv, id.String(), &crypto.GetAccountPayload{AccountID: id.String()})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		acc, err := client.GetAccount(context.Background(), id, signedReq)
		if err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Account: %s\nBalance: %.2f", acc.ID().String(), acc.Balance())
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// transfer tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "transfer",
		Description: "Transfer funds from one account to another",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args TransferArgs) (*mcp.CallToolResult, any, error) {
		fromID, err := scalegraph.ScalegraphIdFromString(args.From)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid 'from' account ID: %v", err)
		}

		signedReq, err := createSignedEnvelope(srv, fromID.String(), &crypto.GetAccountPayload{AccountID: fromID.String()})

		// Get account to calculate nonce
		fromAccount, err := client.GetAccount(context.Background(), fromID, signedReq)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get account for nonce: %v", err)
		}
		nonce := fromAccount.GetNonce()

		// Create transfer request
		transferReq := &crypto.TransferPayload{
			To:        args.To,
			Amount:    args.Amount,
			Nonce:     nonce,
			Timestamp: time.Now().Unix(),
		}

		// Create signed envelope using generic helper
		signedEnvelope, err := createSignedEnvelope(srv, args.From, transferReq)
		if err != nil {
			return nil, nil, err
		}

		// Execute signed transfer
		if _, err := client.TransferSigned(context.Background(), signedEnvelope); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Transferred %.2f from %s to %s", args.Amount, args.From[:16]+"...", args.To[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// mint tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "mint",
		Description: "Mint new tokens to an account",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args MintArgs) (*mcp.CallToolResult, any, error) {
		toID, err := scalegraph.ScalegraphIdFromString(args.To)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid account ID: %v", err)
		}

		if err := client.Mint(context.Background(), toID, args.Amount); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Minted %.2f tokens to account %s", args.Amount, args.To[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// mint_token tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "mint_token",
		Description: "Mint a new token for an account. The signing account becomes the token owner. Optionally specify a clawback address.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args MintTokenArgs) (*mcp.CallToolResult, any, error) {
		if _, err := scalegraph.ScalegraphIdFromString(args.AccountID); err != nil {
			return nil, nil, fmt.Errorf("invalid account_id: %v", err)
		}
		if args.TokenValue == "" {
			return nil, nil, fmt.Errorf("token_value is required")
		}

		var clawbackAddr *string
		if args.ClawbackAddress != "" {
			if _, err := scalegraph.ScalegraphIdFromString(args.ClawbackAddress); err != nil {
				return nil, nil, fmt.Errorf("invalid clawback_address: %v", err)
			}
			clawbackAddr = &args.ClawbackAddress
		}

		var freezeAddr *string
		if args.FreezeAddress != "" {
			if _, err := scalegraph.ScalegraphIdFromString(args.FreezeAddress); err != nil {
				return nil, nil, fmt.Errorf("invalid freeze_address: %v", err)
			}
			freezeAddr = &args.FreezeAddress
		}

		nonce, err := getAccountNonce(client, srv, args.AccountID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get account nonce: %v", err)
		}
		payload := &crypto.MintTokenPayload{
			TokenValue:      args.TokenValue,
			ClawbackAddress: clawbackAddr,
			FreezeAddress:   freezeAddr,
			Nonce:           int64(nonce),
		}
		signedEnvelope, err := createSignedEnvelope(srv, args.AccountID, payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		resp, err := client.MintTokenSigned(context.Background(), signedEnvelope)
		if err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Token minted for account %s\nToken value: %s\nToken ID: %s", args.AccountID[:16]+"...", args.TokenValue, resp.TokenID)
		if args.ClawbackAddress != "" {
			text += fmt.Sprintf("\nClawback address: %s", args.ClawbackAddress[:16]+"...")
		}
		if args.FreezeAddress != "" {
			text += fmt.Sprintf("\nFreeze address: %s", args.FreezeAddress[:16]+"...")
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// authorize_token_transfer tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "authorize_token_transfer",
		Description: "Authorize receiving a token transfer. Called by the future token receiver (account_id), signed by them, and directed at the current token owner (token_owner_id).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args AuthorizeTokenTransferArgs) (*mcp.CallToolResult, any, error) {
		if _, err := scalegraph.ScalegraphIdFromString(args.AccountID); err != nil {
			return nil, nil, fmt.Errorf("invalid account_id: %v", err)
		}
		if _, err := scalegraph.ScalegraphIdFromString(args.TokenOwnerID); err != nil {
			return nil, nil, fmt.Errorf("invalid token_owner_id: %v", err)
		}
		if args.TokenID == "" {
			return nil, nil, fmt.Errorf("token_id is required")
		}

		payload := &crypto.AuthorizeTokenTransferPayload{
			AccountID:    args.AccountID,
			TokenID:      args.TokenID,
			TokenOwnerID: args.TokenOwnerID,
		}
		signedEnvelope, err := createSignedEnvelope(srv, args.AccountID, payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		if _, err := client.AuthorizeTokenTransferSigned(context.Background(), signedEnvelope); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Token %s authorized for transfer to account %s from owner %s", args.TokenID, args.AccountID[:16]+"...", args.TokenOwnerID[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// unauthorize_token_transfer tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "unauthorize_token_transfer",
		Description: "Revoke a previously authorized token transfer. Called by the account that authorized (account_id), directed at the token owner (token_owner_id).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UnauthorizeTokenTransferArgs) (*mcp.CallToolResult, any, error) {
		if _, err := scalegraph.ScalegraphIdFromString(args.AccountID); err != nil {
			return nil, nil, fmt.Errorf("invalid account_id: %v", err)
		}
		if _, err := scalegraph.ScalegraphIdFromString(args.TokenOwnerID); err != nil {
			return nil, nil, fmt.Errorf("invalid token_owner_id: %v", err)
		}
		if args.TokenID == "" {
			return nil, nil, fmt.Errorf("token_id is required")
		}

		payload := &crypto.UnauthorizeTokenTransferPayload{
			AccountID:    args.AccountID,
			TokenID:      args.TokenID,
			TokenOwnerID: args.TokenOwnerID,
		}
		signedEnvelope, err := createSignedEnvelope(srv, args.AccountID, payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		if _, err := client.UnauthorizeTokenTransferSigned(context.Background(), signedEnvelope); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Token %s authorization revoked by account %s", args.TokenID, args.AccountID[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// transfer_token tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "transfer_token",
		Description: "Transfer a token from one account to another. The token must first be authorized for transfer using authorize_token_transfer.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args TransferTokenArgs) (*mcp.CallToolResult, any, error) {
		if _, err := scalegraph.ScalegraphIdFromString(args.FromAccountID); err != nil {
			return nil, nil, fmt.Errorf("invalid from_account_id: %v", err)
		}
		if _, err := scalegraph.ScalegraphIdFromString(args.ToAccountID); err != nil {
			return nil, nil, fmt.Errorf("invalid to_account_id: %v", err)
		}
		if args.TokenID == "" {
			return nil, nil, fmt.Errorf("token_id is required")
		}

		payload := &crypto.TransferTokenPayload{
			From:    args.FromAccountID,
			To:      args.ToAccountID,
			TokenID: args.TokenID,
		}
		signedEnvelope, err := createSignedEnvelope(srv, args.FromAccountID, payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		if _, err := client.TransferTokenSigned(context.Background(), signedEnvelope); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Token %s transferred from %s to %s", args.TokenID, args.FromAccountID[:16]+"...", args.ToAccountID[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// burn_token tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "burn_token",
		Description: "Permanently destroy a token. Must be signed by the token owner.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args BurnTokenArgs) (*mcp.CallToolResult, any, error) {
		if _, err := scalegraph.ScalegraphIdFromString(args.AccountID); err != nil {
			return nil, nil, fmt.Errorf("invalid account_id: %v", err)
		}
		if args.TokenID == "" {
			return nil, nil, fmt.Errorf("token_id is required")
		}

		payload := &crypto.BurnTokenPayload{
			AccountID: args.AccountID,
			TokenID:   args.TokenID,
		}
		signedEnvelope, err := createSignedEnvelope(srv, args.AccountID, payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		if _, err := client.BurnTokenSigned(context.Background(), signedEnvelope); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Token %s burned by account %s", args.TokenID, args.AccountID[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// clawback_token tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "clawback_token",
		Description: "Reclaim a token from a holder back to the issuer/authority. Must be signed by the clawback authority (to_account_id).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ClawbackTokenArgs) (*mcp.CallToolResult, any, error) {
		if _, err := scalegraph.ScalegraphIdFromString(args.FromAccountID); err != nil {
			return nil, nil, fmt.Errorf("invalid from_account_id: %v", err)
		}
		if _, err := scalegraph.ScalegraphIdFromString(args.ToAccountID); err != nil {
			return nil, nil, fmt.Errorf("invalid to_account_id: %v", err)
		}
		if args.TokenID == "" {
			return nil, nil, fmt.Errorf("token_id is required")
		}

		payload := &crypto.ClawbackTokenPayload{
			From:    args.FromAccountID,
			To:      args.ToAccountID,
			TokenID: args.TokenID,
		}
		// Signed by the clawback authority (to_account_id), not the holder
		signedEnvelope, err := createSignedEnvelope(srv, args.ToAccountID, payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		if _, err := client.ClawbackTokenSigned(context.Background(), signedEnvelope); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Token %s clawed back from %s to %s", args.TokenID, args.FromAccountID[:16]+"...", args.ToAccountID[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// freeze_token tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "freeze_token",
		Description: "Freeze a token, preventing it from being transferred. Must be signed by the freeze authority.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args FreezeTokenArgs) (*mcp.CallToolResult, any, error) {
		if _, err := scalegraph.ScalegraphIdFromString(args.FreezeAuthorityID); err != nil {
			return nil, nil, fmt.Errorf("invalid freeze_authority_id: %v", err)
		}
		if _, err := scalegraph.ScalegraphIdFromString(args.TokenHolderID); err != nil {
			return nil, nil, fmt.Errorf("invalid token_holder_id: %v", err)
		}
		if args.TokenID == "" {
			return nil, nil, fmt.Errorf("token_id is required")
		}

		payload := &crypto.FreezeTokenPayload{
			FreezeAuthority: args.FreezeAuthorityID,
			TokenHolder:     args.TokenHolderID,
			TokenID:         args.TokenID,
		}
		signedEnvelope, err := createSignedEnvelope(srv, args.FreezeAuthorityID, payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		if _, err := client.FreezeTokenSigned(context.Background(), signedEnvelope); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Token %s frozen by %s (holder: %s)", args.TokenID, args.FreezeAuthorityID[:16]+"...", args.TokenHolderID[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// unfreeze_token tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "unfreeze_token",
		Description: "Unfreeze a previously frozen token, allowing it to be transferred again. Must be signed by the freeze authority.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UnfreezeTokenArgs) (*mcp.CallToolResult, any, error) {
		if _, err := scalegraph.ScalegraphIdFromString(args.FreezeAuthorityID); err != nil {
			return nil, nil, fmt.Errorf("invalid freeze_authority_id: %v", err)
		}
		if _, err := scalegraph.ScalegraphIdFromString(args.TokenHolderID); err != nil {
			return nil, nil, fmt.Errorf("invalid token_holder_id: %v", err)
		}
		if args.TokenID == "" {
			return nil, nil, fmt.Errorf("token_id is required")
		}

		payload := &crypto.UnfreezeTokenPayload{
			FreezeAuthority: args.FreezeAuthorityID,
			TokenHolder:     args.TokenHolderID,
			TokenID:         args.TokenID,
		}
		signedEnvelope, err := createSignedEnvelope(srv, args.FreezeAuthorityID, payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		if _, err := client.UnfreezeTokenSigned(context.Background(), signedEnvelope); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Token %s unfrozen by %s (holder: %s)", args.TokenID, args.FreezeAuthorityID[:16]+"...", args.TokenHolderID[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// lookup_token tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "lookup_token",
		Description: "Look up a token held by an account. Must be signed by the requesting account.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args LookupTokenArgs) (*mcp.CallToolResult, any, error) {
		if _, err := scalegraph.ScalegraphIdFromString(args.AccountID); err != nil {
			return nil, nil, fmt.Errorf("invalid account_id: %v", err)
		}
		if args.TokenID == "" {
			return nil, nil, fmt.Errorf("token_id is required")
		}

		payload := &crypto.LookupTokenPayload{
			TokenID:   args.TokenID,
			AccountID: args.AccountID,
		}
		signedEnvelope, err := createSignedEnvelope(srv, args.AccountID, payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create signed request: %v", err)
		}

		resp, err := client.LookupTokenSigned(context.Background(), signedEnvelope)
		if err != nil {
			return nil, nil, err
		}

		if resp.Token == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Token %s not found in account %s", args.TokenID, args.AccountID[:16]+"...")}},
			}, nil, nil
		}

		t := resp.Token
		text := fmt.Sprintf("Token found:\n  ID:    %s\n  Value: %s", t.ID(), t.Value())
		if t.ClawbackAddress() != nil {
			text += fmt.Sprintf("\n  Clawback: %s", t.ClawbackAddress().String())
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// get_account_count tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "get_account_count",
		Description: "Get the total number of accounts in the scalegraph",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
		count, err := client.AccountCount(context.Background())
		if err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Total accounts: %d", count)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// get_virtual_nodes tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "get_virtual_nodes",
		Description: "List all virtual nodes/apps in the system with their IDs and transports",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
		registry := srv.Registry()
		vapps := registry.List()

		if len(vapps) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No virtual nodes found"}},
			}, nil, nil
		}

		text := fmt.Sprintf("Total virtual nodes: %d\n\n", len(vapps))
		for i, vapp := range vapps {
			text += fmt.Sprintf("%d. ID: %s\n", i+1, vapp.ID().String())
			addresses := vapp.Addresses()
			if len(addresses) > 0 {
				text += "   Transports:\n"
				for tType, addr := range addresses {
					text += fmt.Sprintf("   - %s: %s\n", tType, addr)
				}
			}
			text += "\n"
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// admin_create_account tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "admin_create_account",
		Description: "Create a new account without requiring a signed request (admin/bypass auth). Returns account ID and balance.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args AdminCreateAccountArgs) (*mcp.CallToolResult, any, error) {
		resp, err := client.AdminCreateAccount(context.Background(), args.Balance)
		if err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Created account %s with balance %.2f\n", resp.Account.ID().String(), resp.Account.Balance())
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// admin_mint tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "admin_mint",
		Description: "Mint tokens to an account without requiring a signed request (admin/bypass auth).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args AdminMintArgs) (*mcp.CallToolResult, any, error) {
		toID, err := scalegraph.ScalegraphIdFromString(args.To)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid account ID: %v", err)
		}

		if err := client.AdminMint(context.Background(), toID, args.Amount); err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Minted %.2f tokens to account %s", args.Amount, args.To[:16]+"...")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})

	// create_signed_request tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "create_signed_request",
		Description: "Create a cryptographically signed request for REST API endpoints. Generates a complete SignedEnvelope with Ed25519 signature and certificate. Supports request types: 'transfer', 'get_account', 'mint_token', 'authorize_token_transfer', 'unauthorize_token_transfer', 'transfer_token', 'burn_token', 'clawback_token', 'lookup_token'.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateSignedRequestArgs) (*mcp.CallToolResult, any, error) {
		// Validate account ID
		_, err := scalegraph.ScalegraphIdFromString(args.AccountID)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid account_id: %v", err)
		}

		// Get the CA from server
		ca := srv.CA()
		if ca == nil {
			return nil, nil, fmt.Errorf("Certificate Authority not available")
		}

		// Retrieve private key
		privKeyPEM, err := ca.GetAccountPrivateKeyPEM(args.AccountID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get private key for account %s: %v (account may not have credentials)", args.AccountID, err)
		}

		// Retrieve certificate
		certPEM, err := ca.GetAccountCertificatePEM(args.AccountID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get certificate for account %s: %v", args.AccountID, err)
		}

		// Decode private key
		privKey, err := crypto.DecodePrivateKeyPEM([]byte(privKeyPEM))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode private key: %v", err)
		}

		var signedEnvelopeJSON []byte

		switch args.Type {
		case "transfer":
			// Validate transfer-specific fields
			if args.ToID == "" {
				return nil, nil, fmt.Errorf("to_id is required for transfer requests")
			}
			if args.Amount <= 0 {
				return nil, nil, fmt.Errorf("amount must be positive for transfer requests")
			}

			// Validate to account ID
			_, err := scalegraph.ScalegraphIdFromString(args.ToID)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid to_id: %v", err)
			}

			nonce, err := getAccountNonce(client, srv, args.AccountID)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get account nonce: %v", err)
			}
			// Create transfer request
			transferReq := &crypto.TransferPayload{
				From:      args.AccountID,
				To:        args.ToID,
				Amount:    args.Amount,
				Nonce:     nonce,
				Timestamp: time.Now().Unix(),
			}

			// Create signed envelope
			envelope, err := crypto.CreateSignedEnvelope(transferReq, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}

			// Marshal to JSON
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "get_account":
			account, _ := scalegraph.ScalegraphIdFromString(args.AccountID)

			// Create signable account request
			accountReq := &crypto.GetAccountPayload{
				AccountID: account.String(),
			}

			signedReq, err := createSignedEnvelope(srv, args.AccountID, accountReq)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}

			// Marshal to JSON
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": signedReq,
				"account_id":      account.String(),
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "mint_token":
			if args.TokenValue == "" {
				return nil, nil, fmt.Errorf("token_value is required for mint_token requests")
			}
			var clawbackAddr *string
			if args.ClawbackAddress != "" {
				if _, err := scalegraph.ScalegraphIdFromString(args.ClawbackAddress); err != nil {
					return nil, nil, fmt.Errorf("invalid clawback_address: %v", err)
				}
				clawbackAddr = &args.ClawbackAddress
			}
			var freezeAddr *string
			if args.FreezeAddress != "" {
				if _, err := scalegraph.ScalegraphIdFromString(args.FreezeAddress); err != nil {
					return nil, nil, fmt.Errorf("invalid freeze_address: %v", err)
				}
				freezeAddr = &args.FreezeAddress
			}
			nonce, err := getAccountNonce(client, srv, args.AccountID)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get account nonce: %v", err)
			}
			payload := &crypto.MintTokenPayload{
				TokenValue:      args.TokenValue,
				ClawbackAddress: clawbackAddr,
				FreezeAddress:   freezeAddr,
				Nonce:           int64(nonce),
			}
			envelope, err := crypto.CreateSignedEnvelope(payload, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "lookup_token":
			if args.TokenID == "" {
				return nil, nil, fmt.Errorf("token_id is required for lookup_token requests")
			}
			payload := &crypto.LookupTokenPayload{
				TokenID:   args.TokenID,
				AccountID: args.AccountID,
			}
			envelope, err := crypto.CreateSignedEnvelope(payload, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
				"account_id":      args.AccountID,
				"token_id":        args.TokenID,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "authorize_token_transfer":
			if args.TokenID == "" {
				return nil, nil, fmt.Errorf("token_id is required for authorize_token_transfer requests")
			}
			if args.TokenOwnerID == "" {
				return nil, nil, fmt.Errorf("token_owner_id is required for authorize_token_transfer requests")
			}
			payload := &crypto.AuthorizeTokenTransferPayload{
				AccountID:    args.AccountID,
				TokenID:      args.TokenID,
				TokenOwnerID: args.TokenOwnerID,
			}
			envelope, err := crypto.CreateSignedEnvelope(payload, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "unauthorize_token_transfer":
			if args.TokenID == "" {
				return nil, nil, fmt.Errorf("token_id is required for unauthorize_token_transfer requests")
			}
			if args.TokenOwnerID == "" {
				return nil, nil, fmt.Errorf("token_owner_id is required for unauthorize_token_transfer requests")
			}
			payload := &crypto.UnauthorizeTokenTransferPayload{
				AccountID:    args.AccountID,
				TokenID:      args.TokenID,
				TokenOwnerID: args.TokenOwnerID,
			}
			envelope, err := crypto.CreateSignedEnvelope(payload, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "transfer_token":
			if args.TokenID == "" {
				return nil, nil, fmt.Errorf("token_id is required for transfer_token requests")
			}
			if args.ToAccountID == "" {
				return nil, nil, fmt.Errorf("to_account_id is required for transfer_token requests")
			}
			if _, err := scalegraph.ScalegraphIdFromString(args.ToAccountID); err != nil {
				return nil, nil, fmt.Errorf("invalid to_account_id: %v", err)
			}
			payload := &crypto.TransferTokenPayload{
				From:    args.AccountID,
				To:      args.ToAccountID,
				TokenID: args.TokenID,
			}
			envelope, err := crypto.CreateSignedEnvelope(payload, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "burn_token":
			if args.TokenID == "" {
				return nil, nil, fmt.Errorf("token_id is required for burn_token requests")
			}
			payload := &crypto.BurnTokenPayload{
				AccountID: args.AccountID,
				TokenID:   args.TokenID,
			}
			envelope, err := crypto.CreateSignedEnvelope(payload, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "clawback_token":
			if args.TokenID == "" {
				return nil, nil, fmt.Errorf("token_id is required for clawback_token requests")
			}
			if args.FromAccountID == "" {
				return nil, nil, fmt.Errorf("from_account_id is required for clawback_token requests")
			}
			if _, err := scalegraph.ScalegraphIdFromString(args.FromAccountID); err != nil {
				return nil, nil, fmt.Errorf("invalid from_account_id: %v", err)
			}
			// account_id is the clawback authority (signer/recipient), from_account_id is the holder
			payload := &crypto.ClawbackTokenPayload{
				From:    args.FromAccountID,
				To:      args.AccountID,
				TokenID: args.TokenID,
			}
			envelope, err := crypto.CreateSignedEnvelope(payload, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "freeze_token":
			if args.TokenID == "" {
				return nil, nil, fmt.Errorf("token_id is required for freeze_token requests")
			}
			if args.TokenHolderID == "" {
				return nil, nil, fmt.Errorf("token_holder_id is required for freeze_token requests")
			}
			if _, err := scalegraph.ScalegraphIdFromString(args.TokenHolderID); err != nil {
				return nil, nil, fmt.Errorf("invalid token_holder_id: %v", err)
			}
			// account_id is the freeze authority (signer)
			payload := &crypto.FreezeTokenPayload{
				FreezeAuthority: args.AccountID,
				TokenHolder:     args.TokenHolderID,
				TokenID:         args.TokenID,
			}
			envelope, err := crypto.CreateSignedEnvelope(payload, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		case "unfreeze_token":
			if args.TokenID == "" {
				return nil, nil, fmt.Errorf("token_id is required for unfreeze_token requests")
			}
			if args.TokenHolderID == "" {
				return nil, nil, fmt.Errorf("token_holder_id is required for unfreeze_token requests")
			}
			if _, err := scalegraph.ScalegraphIdFromString(args.TokenHolderID); err != nil {
				return nil, nil, fmt.Errorf("invalid token_holder_id: %v", err)
			}
			// account_id is the freeze authority (signer)
			payload := &crypto.UnfreezeTokenPayload{
				FreezeAuthority: args.AccountID,
				TokenHolder:     args.TokenHolderID,
				TokenID:         args.TokenID,
			}
			envelope, err := crypto.CreateSignedEnvelope(payload, privKey, args.AccountID, certPEM)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create signed envelope: %v", err)
			}
			signedEnvelopeJSON, err = json.MarshalIndent(map[string]interface{}{
				"signed_envelope": envelope,
			}, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal signed envelope: %v", err)
			}

		default:
			return nil, nil, fmt.Errorf("unsupported request type: %s (must be 'transfer', 'get_account', 'mint_token', 'authorize_token_transfer', 'unauthorize_token_transfer', 'transfer_token', 'burn_token', 'clawback_token', 'freeze_token', 'unfreeze_token', or 'lookup_token')", args.Type)
		}

		text := fmt.Sprintf("Signed %s request created for account %s\n\n", args.Type, args.AccountID[:16]+"...")
		text += "Copy this JSON body and paste it into the Swagger UI for the REST API:\n\n"
		text += string(signedEnvelopeJSON)
		text += "\n\n"
		text += "Instructions:\n"
		text += "1. Open the Swagger UI (usually at http://localhost:8080/swagger/index.html)\n"
		switch args.Type {
		case "transfer":
			text += "2. Find the POST /transfer endpoint\n"
		case "get_account":
			text += "2. Find the POST /accounts/me endpoint\n"
		case "mint_token":
			text += "2. Find the POST /tokens/mint endpoint\n"
		case "authorize_token_transfer":
			text += "2. Find the POST /tokens/authorize endpoint\n"
		case "unauthorize_token_transfer":
			text += "2. Find the POST /tokens/unauthorize endpoint\n"
		case "transfer_token":
			text += "2. Find the POST /tokens/transfer endpoint\n"
		case "burn_token":
			text += "2. Find the POST /tokens/burn endpoint\n"
		case "clawback_token":
			text += "2. Find the POST /tokens/clawback endpoint\n"
		case "freeze_token":
			text += "2. Find the POST /tokens/freeze endpoint\n"
		case "unfreeze_token":
			text += "2. Find the POST /tokens/unfreeze endpoint\n"
		}
		text += "3. Click 'Try it out'\n"
		text += "4. Paste the entire JSON above into the request body\n"
		text += "5. Click 'Execute'\n"

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

func getAccountNonce(client *server.Client, srv *server.Server, accountID string) (uint64, error) {
	id, err := scalegraph.ScalegraphIdFromString(accountID)
	if err != nil {
		return 0, fmt.Errorf("invalid account ID: %v", err)
	}

	signedReq, err := createSignedEnvelope(srv, id.String(), &crypto.GetAccountPayload{AccountID: id.String()})
	if err != nil {
		return 0, fmt.Errorf("failed to create signed request: %v", err)
	}

	acc, err := client.GetAccount(context.Background(), id, signedReq)
	if err != nil {
		return 0, fmt.Errorf("failed to get account: %v", err)
	}

	return acc.GetNonce(), nil
}
