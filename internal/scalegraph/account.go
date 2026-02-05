package scalegraph

import (
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"sync"
)

type Account struct {
	mu               sync.RWMutex
	id               ScalegraphId
	balance          float64
	blockchain       IBlockchain
	valuestore       map[string]string
	publicKey        ed25519.PublicKey
	certificate      *x509.Certificate
	TransactionCount uint64 // Number of transactions sent from this account
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
		id:               id,
		balance:          0,
		blockchain:       blockchain,
		valuestore:       make(map[string]string),
		TransactionCount: 0,
	}

	return acc, nil
}

// newAccountWithPublicKey creates a new account with a public key and certificate
// The account ID is derived from the public key hash
func newAccountWithPublicKey(pubKey ed25519.PublicKey, cert *x509.Certificate) (*Account, error) {
	id := ScalegraphIdFromPublicKey(pubKey)

	acc := &Account{
		id:               id,
		balance:          0,
		blockchain:       newBlockchain(),
		valuestore:       make(map[string]string),
		publicKey:        pubKey,
		certificate:      cert,
		TransactionCount: 0,
	}

	return acc, nil
}

// PublicKey returns the account's public key (may be nil for legacy accounts)
func (a *Account) PublicKey() ed25519.PublicKey {
	return a.publicKey
}

// Certificate returns the account's X.509 certificate (may be nil for legacy accounts)
func (a *Account) Certificate() *x509.Certificate {
	return a.certificate
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
