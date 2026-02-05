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

// New creates a new App instance
func New(logger *slog.Logger) *App {
	return &App{
		accounts: make(map[ScalegraphId]*Account),
		logger:   logger,
	}
}

// CreateAccount creates a new account with an optional initial balance
func (a *App) CreateAccount(ctx context.Context, initialBalance float64) (*Account, error) {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Creating account", "initial_balance", initialBalance)
	acc, err := newAccount()
	if err != nil {
		logger.Error("Failed to create account", "error", err)
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	if initialBalance > 0 {
		if err := acc.mint(initialBalance); err != nil {
			logger.Error("Failed to mint initial balance", "error", err, "account_id", acc.ID(), "amount", initialBalance)
			return nil, fmt.Errorf("failed to mint initial balance: %w", err)
		}
	}

	a.mu.Lock()
	a.accounts[acc.ID()] = acc
	totalAccounts := len(a.accounts)
	a.mu.Unlock()

	logger.Info("Account created", "account_id", acc.ID(), "balance", acc.Balance(), "total_accounts", totalAccounts)
	return acc, nil
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

	if initialBalance > 0 {
		if err := acc.mint(initialBalance); err != nil {
			logger.Error("Failed to mint initial balance", "error", err, "account_id", acc.ID(), "amount", initialBalance)
			return nil, fmt.Errorf("failed to mint initial balance: %w", err)
		}
	}

	a.mu.Lock()
	a.accounts[acc.ID()] = acc
	totalAccounts := len(a.accounts)
	a.mu.Unlock()

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

	// Lock both accounts to ensure atomicity
	// Lock from account first to validate nonce atomically
	fromAcc.mu.Lock()
	defer fromAcc.mu.Unlock()

	toAcc.mu.Lock()
	defer toAcc.mu.Unlock()

	// Validate preconditions
	if amount > fromAcc.balance {
		logger.Warn("Insufficient funds", "from", from, "balance", fromAcc.balance, "amount", amount)
		return fmt.Errorf("insufficient funds: attempted to transfer %.2f but balance is %.2f", amount, fromAcc.balance)
	}

	fromBalanceBefore := fromAcc.balance
	toBalanceBefore := toAcc.balance

	// Create both transactions before applying any changes
	fromTx, err := newTransaction(fromAcc, toAcc, amount, "")
	if err != nil {
		logger.Error("Failed to create from transaction", "error", err, "from", from, "to", to, "amount", amount)
		return err
	}
	toTx, err := newTransaction(fromAcc, toAcc, amount, "")
	if err != nil {
		logger.Error("Failed to create to transaction", "error", err, "from", from, "to", to, "amount", amount)
		return err
	}

	// Apply changes atomically
	fromAcc.blockchain.append(fromTx)
	fromAcc.balance -= amount

	toAcc.blockchain.append(toTx)
	toAcc.balance += amount

	logger.Info("Transfer completed",
		"from", from,
		"to", to,
		"amount", amount,
		"from_balance_before", fromBalanceBefore,
		"from_balance_after", fromAcc.balance,
		"to_balance_before", toBalanceBefore,
		"to_balance_after", toAcc.balance)

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

	if err := toAcc.mint(amount); err != nil {
		logger.Error("Mint failed", "error", err, "account_id", to, "amount", amount)
		return err
	}

	logger.Info("Mint completed", "account_id", to, "amount", amount, "new_balance", toAcc.Balance())
	return nil
}

// AccountCount returns the number of accounts
func (a *App) AccountCount(ctx context.Context) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.accounts)
}
