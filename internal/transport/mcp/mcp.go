package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

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

// RunHTTPServer starts an MCP server over HTTP with SSE transport.
// This allows the MCP server to run alongside TUI since it doesn't use stdio.
func RunHTTPServer(ctx context.Context, addr string, client *server.Client, srv *server.Server, logger *slog.Logger) error {
	logger.Info("MCP server starting", "address", addr)
	mcpServer := createServer(client, srv, logger)

	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

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

func registerTools(mcpServer *mcp.Server, client *server.Client, srv *server.Server) {
	// create_account tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "create_account",
		Description: "Create a new account with an optional initial balance",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateAccountArgs) (*mcp.CallToolResult, any, error) {
		acc, err := client.CreateAccount(context.Background(), args.Balance)
		if err != nil {
			return nil, nil, err
		}

		text := fmt.Sprintf("Created account %s with balance %.2f", acc.ID().String(), acc.Balance())
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

		acc, err := client.GetAccount(context.Background(), id)
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

		if err := client.Transfer(context.Background(), fromID, toID, args.Amount); err != nil {
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
}
