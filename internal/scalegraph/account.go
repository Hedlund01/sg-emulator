package scalegraph

import (
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"sync"
)

type Account struct {
	mu          sync.RWMutex
	id          ScalegraphId
	balance     float64
	blockchain  IBlockchain
	valuestore  map[string]string
	publicKey   ed25519.PublicKey
	certificate *x509.Certificate
}

// newAccountWithPublicKey creates a new account with a public key and certificate
// The account ID is derived from the public key hash
func newAccountWithPublicKey(pubKey ed25519.PublicKey, cert *x509.Certificate) (*Account, error) {
	id := ScalegraphIdFromPublicKey(pubKey)

	acc := &Account{
		id:          id,
		balance:     0,
		blockchain:  newBlockchain(),
		valuestore:  make(map[string]string),
		publicKey:   pubKey,
		certificate: cert,
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

// GetNonce returns the current nonce for this account (blockchain length)
// The next transaction from this account should use nonce = GetNonce() + 1
func (a *Account) GetNonce() uint64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return uint64(a.blockchain.Len())
}

func (a *Account) appendTransaction(trx ITransaction) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch *trx.Type() {
	case Mint:
		tx := trx.(*MintTransaction)
		a.balance += tx.Amount()
	case Transfer:
		tx := trx.(*TransferTransaction)
		if tx.Sender() != nil && tx.Sender().ID() == a.ID() {
			if a.balance < tx.Amount() {
				return fmt.Errorf("insufficient balance for transfer: current balance %.2f, transfer amount %.2f", a.balance, tx.Amount())
			}
			a.balance -= tx.Amount()
		}
		if tx.Receiver() != nil && tx.Receiver().ID() == a.ID() {
			a.balance += tx.Amount()
		}
	case Burn:
		tx := trx.(*BurnTransaction)
		if tx.Receiver() != nil && tx.Receiver().ID() == a.ID() {
			if a.balance < tx.Amount() {
				return fmt.Errorf("insufficient balance for burn, can not burn more than balance: current balance %.2f, burn amount %.2f", a.balance, tx.Amount())
			}
			a.balance -= tx.Amount()
		}
	}

	a.blockchain.append(trx)
	return nil

}

func (a *Account) String() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return fmt.Sprintf("Account(ID: %s, Balance: %.2f, Valutestore: %v)", a.id, a.balance, a.valuestore)
}
