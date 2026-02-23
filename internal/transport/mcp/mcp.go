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

// CreateSignedRequestArgs represents arguments for create_signed_request tool
type CreateSignedRequestArgs struct {
	Type      string  `json:"type" jsonschema:"Type of request: 'transfer' or 'get_account'"`
	AccountID string  `json:"account_id" jsonschema:"Account ID that will sign the request (must have credentials)"`
	ToID      string  `json:"to_id,omitempty" jsonschema:"Destination account ID (for transfer only)"`
	Amount    float64 `json:"amount,omitempty" jsonschema:"Amount to transfer (for transfer only)"`
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

		toID, err := scalegraph.ScalegraphIdFromString(args.To)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid 'to' account ID: %v", err)
		}

		signedReq, err := createSignedEnvelope(srv, fromID.String(), &crypto.GetAccountPayload{AccountID: fromID.String()})

		// Get account to calculate nonce
		fromAccount, err := client.GetAccount(context.Background(), fromID, signedReq)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get account for nonce: %v", err)
		}
		nonce := fromAccount.GetNonce() + 1

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
		if err := client.TransferSigned(context.Background(), fromID, toID, args.Amount, signedEnvelope); err != nil {
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

	// create_signed_request tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "create_signed_request",
		Description: "Create a cryptographically signed request for REST API endpoints. Generates a complete SignedEnvelope with Ed25519 signature and certificate. Supports 'transfer' and 'get_account' request types.",
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
			nonce++

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

		default:
			return nil, nil, fmt.Errorf("unsupported request type: %s (must be 'transfer' or 'get_account')", args.Type)
		}

		text := fmt.Sprintf("Signed %s request created for account %s\n\n", args.Type, args.AccountID[:16]+"...")
		text += "Copy this JSON body and paste it into the Swagger UI for the REST API:\n\n"
		text += string(signedEnvelopeJSON)
		text += "\n\n"
		text += "Instructions:\n"
		text += "1. Open the Swagger UI (usually at http://localhost:8080/swagger/index.html)\n"
		if args.Type == "transfer" {
			text += "2. Find the POST /transfer endpoint\n"
		} else {
			text += "2. Find the POST /accounts/me endpoint\n"
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
