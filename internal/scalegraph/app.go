package scalegraph

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"log/slog"
	"sync"

	"sg-emulator/internal/trace"
)

// App manages accounts and provides the core business logic.
// This can be used by TUI, gRPC, REST, or any other interface.
type App struct {
	mu       sync.RWMutex
	accounts map[ScalegraphId]*Account
	logger   *slog.Logger
}

// NewApp creates a new App instance
func NewApp(logger *slog.Logger) *App {
	return &App{
		accounts: make(map[ScalegraphId]*Account),
		logger:   logger,
	}
}

// CreateAccountWithKeys creates a new account with a public key and certificate
// The account ID is derived from the public key hash
func (a *App) CreateAccountWithKeys(ctx context.Context, pubKey ed25519.PublicKey, cert *x509.Certificate, initialBalance float64) (*Account, error) {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}

	// Derive account ID from public key
	id := ScalegraphIdFromPublicKey(pubKey)
	logger.Debug("Creating account with keys", "account_id", id, "initial_balance", initialBalance)

	// Check if account already exists
	a.mu.RLock()
	_, exists := a.accounts[id]
	a.mu.RUnlock()
	if exists {
		logger.Warn("Account already exists", "account_id", id)
		return nil, fmt.Errorf("account already exists: %s", id)
	}

	acc, err := newAccountWithPublicKey(pubKey, cert)
	if err != nil {
		logger.Error("Failed to create account with keys", "error", err)
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	// Add account to map before minting so Mint() can find it
	a.mu.Lock()
	a.accounts[acc.ID()] = acc
	a.mu.Unlock()

	if initialBalance > 0 {
		if err := a.Mint(ctx, acc.ID(), initialBalance); err != nil {
			// Rollback: remove account from map if mint fails
			a.mu.Lock()
			delete(a.accounts, acc.ID())
			a.mu.Unlock()
			logger.Error("Failed to mint initial balance", "error", err, "account_id", acc.ID(), "amount", initialBalance)
			return nil, fmt.Errorf("failed to mint initial balance: %w", err)
		}
	}

	a.mu.RLock()
	totalAccounts := len(a.accounts)
	a.mu.RUnlock()

	logger.Info("Account created with keys", "account_id", acc.ID(), "balance", acc.Balance(), "total_accounts", totalAccounts)
	return acc, nil
}

// GetAccounts returns all accounts
func (a *App) GetAccounts(ctx context.Context) []*Account {
	a.mu.RLock()
	defer a.mu.RUnlock()

	accounts := make([]*Account, 0, len(a.accounts))
	for _, acc := range a.accounts {
		accounts = append(accounts, acc)
	}
	return accounts
}

// GetAccount returns an account by index
func (a *App) GetAccount(ctx context.Context, id ScalegraphId) (*Account, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	index, exists := a.accounts[id]
	if !exists {
		return nil, fmt.Errorf("account not found: %s", id)
	}

	return index, nil
}

// Transfer transfers funds between two accounts atomically
func (a *App) Transfer(ctx context.Context, from, to ScalegraphId, amount float64, nonce uint64) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Transfer initiated", "from", from, "to", to, "amount", amount, "nonce", nonce)
	a.mu.RLock()
	defer a.mu.RUnlock()

	fromAcc, exists := a.accounts[from]
	if !exists {
		logger.Warn("Source account not found", "from", from)
		return fmt.Errorf("source account not found: %s", from)
	}

	toAcc, exists := a.accounts[to]
	if !exists {
		logger.Warn("Destination account not found", "to", to)
		return fmt.Errorf("destination account not found: %s", to)
	}

	// Reject self-transfers
	if from == to {
		logger.Warn("Self-transfer not allowed", "account", from)
		return fmt.Errorf("self-transfer not allowed")
	}

	// Validate nonce before proceeding
	expectedNonce := fromAcc.GetNonce() + 1
	if nonce != expectedNonce {
		logger.Warn("Nonce mismatch", "from", from, "expected", expectedNonce, "got", nonce)
		return fmt.Errorf("nonce mismatch: expected %d, got %d", expectedNonce, nonce)
	}

	transferTx := newTransferTransaction(fromAcc, toAcc, amount)

	if err := fromAcc.appendTransaction(transferTx); err != nil {
		logger.Error("Failed to append transaction", "error", err)
		return err
	}

	if err := toAcc.appendTransaction(transferTx); err != nil {
		logger.Error("Failed to append transaction", "error", err)
		return err
	}

	return nil
}

func (a *App) Mint(ctx context.Context, to ScalegraphId, amount float64) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Mint operation initiated", "account_id", to, "amount", amount)
	a.mu.RLock()
	defer a.mu.RUnlock()

	toAcc, exists := a.accounts[to]
	if !exists {
		logger.Warn("Account not found for mint", "account_id", to)
		return fmt.Errorf("destination account not found: %s", to)
	}

	mintTx := newMintTransaction(toAcc, amount)

	toAcc.appendTransaction(mintTx)

	logger.Info("Mint completed", "account_id", to, "amount", amount, "new_balance", toAcc.Balance())
	return nil
}

func (a *App) MintToken(ctx context.Context, to ScalegraphId, tokenValue string, signature []byte, clawbackAddress *ScalegraphId) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Mint token operation initiated", "account_id", to, "token_value", tokenValue)
	a.mu.RLock()
	defer a.mu.RUnlock()

	toAcc, exists := a.accounts[to]
	if !exists {
		logger.Warn("Account not found for mint token", "account_id", to)
		return fmt.Errorf("destination account not found: %s", to)
	}

	token := newToken(tokenValue, signature, clawbackAddress)
	mintTokenTx := newMintTokenTransaction(toAcc, token)

	if err := toAcc.appendTransaction(mintTokenTx); err != nil {
		logger.Error("Failed to append mint token transaction", "error", err)
		return err
	}

	logger.Info("Mint token completed", "account_id", to, "token_value", tokenValue)
	return nil
}

func (a *App) TransferToken(ctx context.Context, from, to ScalegraphId, tokenId ScalegraphId) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Transfer token operation initiated", "from", from, "to", to, "token_id", tokenId)
	a.mu.RLock()
	defer a.mu.RUnlock()

	fromAcc, exists := a.accounts[from]
	if !exists {
		logger.Warn("Source account not found for token transfer", "from", from)
		return fmt.Errorf("source account not found: %s", from)
	}
	
	toAcc, exists := a.accounts[to]
	if !exists {
		logger.Warn("Destination account not found for token transfer", "to", to)
		return fmt.Errorf("destination account not found: %s", to)
	}

	token, exists := fromAcc.GetToken(tokenId)
	if !exists {
		logger.Warn("Token not found in source account", "token_id", tokenId, "from_account", from)
		return fmt.Errorf("token not found in source account: %s", tokenId)
	}
	
	transferTokenTx := newTransferTokenTransaction(fromAcc, toAcc, token)

	if err := fromAcc.appendTransaction(transferTokenTx); err != nil {
		logger.Error("Failed to append transfer token transaction", "error", err)
		return err
	}

	if err := toAcc.appendTransaction(transferTokenTx); err != nil {
		logger.Error("Failed to append transfer token transaction", "error", err)
		// Rollback: remove transaction from sender if receiver append fails
		fromAcc.rollbacklatestTransaction(transferTokenTx)
		return err
	}

	logger.Info("Transfer token completed", "from", from, "to", to, "token_id", tokenId)
	return nil
}

// AccountCount returns the number of accounts
func (a *App) AccountCount(ctx context.Context) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.accounts)
}
