package scalegraph

import (
	"fmt"
	"sync"
)

type Account struct {
	mu         sync.RWMutex
	id         ScalegraphId
	balance    float64
	blockchain IBlockchain
	valuestore map[string]string
}

// newAccount creates a new account with a unique ID and initial balance
func newAccount() (*Account, error) {
	return newAccountWithBlockchain(newBlockchain())
}

// newAccountWithBlockchain creates a new account with a provided blockchain (for testing)
func newAccountWithBlockchain(blockchain IBlockchain) (*Account, error) {
	id, err := NewScalegraphId()
	if err != nil {
		return nil, err
	}

	// Create account with provided blockchain
	acc := &Account{
		id:         id,
		balance:    0,
		blockchain: blockchain,
		valuestore: make(map[string]string),
	}

	return acc, nil
}

// ID returns the account's unique identifier
func (a *Account) ID() ScalegraphId {
	return a.id
}

// Balance returns the account's current balance
func (a *Account) Balance() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.balance
}

// Blockchain returns the account's blockchain
func (a *Account) Blockchain() IBlockchain {
	return a.blockchain
}

func (a *Account) updateValue(key, value string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tx, err := newTransaction(a, a, 0, value)
	if err != nil {
		return err
	}

	a.blockchain.append(tx)
	a.valuestore[key] = value
	return nil
}

// Mint creates new funds in the account and records the transaction in the blockchain
func (a *Account) mint(amount float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tx, err := newTransaction(nil, a, amount, "")
	if err != nil {
		return err
	}
	a.blockchain.append(tx)
	a.balance += amount
	return nil
}

func (a *Account) String() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return fmt.Sprintf("Account(ID: %s, Balance: %.2f, Valutestore: %v)", a.id, a.balance, a.valuestore)
}
