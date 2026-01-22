package scalegraph

import (
	"fmt"
	"sync"
)

// App manages accounts and provides the core business logic.
// This can be used by TUI, gRPC, REST, or any other interface.
type App struct {
	mu       sync.RWMutex
	accounts map[ScalegraphId]*Account
}

// New creates a new App instance
func New() *App {
	return &App{
		accounts: make(map[ScalegraphId]*Account),
	}
}

// CreateAccount creates a new account with an optional initial balance
func (a *App) CreateAccount(initialBalance float64) (*Account, error) {
	acc, err := newAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	if initialBalance > 0 {
		if err := acc.mint(initialBalance); err != nil {
			return nil, fmt.Errorf("failed to mint initial balance: %w", err)
		}
	}

	a.mu.Lock()
	a.accounts[acc.ID()] = acc
	a.mu.Unlock()

	return acc, nil
}

// GetAccounts returns all accounts
func (a *App) GetAccounts() []*Account {
	a.mu.RLock()
	defer a.mu.RUnlock()

	accounts := make([]*Account, 0, len(a.accounts))
	for _, acc := range a.accounts {
		accounts = append(accounts, acc)
	}
	return accounts
}

// GetAccount returns an account by index
func (a *App) GetAccount(id ScalegraphId) (*Account, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	index, exists := a.accounts[id]
	if !exists {
		return nil, fmt.Errorf("account not found: %s", id)
	}

	return index, nil
}

// Transfer transfers funds between two accounts atomically
func (a *App) Transfer(from, to ScalegraphId, amount float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	fromAcc, exists := a.accounts[from]
	if !exists {
		return fmt.Errorf("source account not found: %s", from)
	}

	toAcc, exists := a.accounts[to]
	if !exists {
		return fmt.Errorf("destination account not found: %s", to)
	}

	// Lock both accounts to ensure atomicity
	fromAcc.mu.Lock()
	defer fromAcc.mu.Unlock()
	toAcc.mu.Lock()
	defer toAcc.mu.Unlock()

	// Validate preconditions
	if amount > fromAcc.balance {
		return fmt.Errorf("insufficient funds: attempted to transfer %.2f but balance is %.2f", amount, fromAcc.balance)
	}

	// Create both transactions before applying any changes
	fromTx, err := newTransaction(fromAcc, toAcc, amount, "")
	if err != nil {
		return err
	}
	toTx, err := newTransaction(fromAcc, toAcc, amount, "")
	if err != nil {
		return err
	}

	// Apply changes atomically
	fromAcc.blockchain.append(fromTx)
	fromAcc.balance -= amount

	toAcc.blockchain.append(toTx)
	toAcc.balance += amount

	return nil
}

func (a *App) Mint(to ScalegraphId, amount float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	toAcc, exists := a.accounts[to]
	if !exists {
		return fmt.Errorf("destination account not found: %s", to)
	}

	return toAcc.mint(amount)
}

// AccountCount returns the number of accounts
func (a *App) AccountCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.accounts)
}
