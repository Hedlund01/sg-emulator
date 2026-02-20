package scalegraph

import (
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"sync"
)

const (
	// MBR (Minimum Balance Requirement) = MBR_SLOT_COST * number of authorized token transfers (i.e. number of &Token{} entries in the token store) + MBR_TOKEN_COST * number of tokens created by the account. Each token creation or authorization of token transfer will require the account to have at least MBR_SLOT_COST balance as MBR, which will be unfrozen when the burn token transaction is executed or the unauthorize token transfer transaction is executed. This is to prevent accounts to create DoS token transactions by authorizing unlimited token transfers or token creation transactions, which can cause the blockchain to grow indefinitely and consume all memory.

	MBR_SLOT_COST = 0.5 // Each token transfer transaction will require the sender to have at least this balance as MBR, which will unfreeze when the unauthorize token transfer transaction is executed.

	MBR_TOKEN_COST = 1.0 // Each token creation transaction will require the sender to have at least this balance as MBR, which will unfreeze when the burn token transaction is executed.

)

type Account struct {
	mu          sync.RWMutex
	id          ScalegraphId
	balance     float64
	mbr         float64
	blockchain  IBlockchain
	valuestore  map[string]string
	publicKey   ed25519.PublicKey
	certificate *x509.Certificate
	tokenStore  map[ScalegraphId]*Token // &Token{} means authorized but not owned, nil means not authorized, non-nil means owned
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

// GetToken returns the token with the given ID if it exists in the account's token store.
// Returns the token and true if found, or nil and false if not found.
// Returns nil and false if the token is not authorized for this account (i.e. token store entry is &Token{}), or if the token does not exist (i.e. token store entry is nil).
func (a *Account) GetToken(tokenId ScalegraphId) (*Token, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.tokenStore[tokenId].Equal(&Token{}) {
		return nil, false
	}

	token, exists := a.tokenStore[tokenId]

	return token, exists
}

func (a *Account) addToken(token *Token) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.tokenStore[token.ID()].Equal(&Token{}) {
		return fmt.Errorf("token with ID %s is not authorized to transfer to account %s", token.ID(), a.ID())
	}

	if a.tokenStore[token.ID()] != nil {
		return fmt.Errorf("token with ID %s already exists in account %s", token.ID(), a.ID())
	}

	signerID, err := ScalegraphIdFromString(token.Signature().SignerID)
	if err != nil {
		return fmt.Errorf("invalid signer ID in token signature: %w", err)
	}
	if signerID != a.ID() {
		return fmt.Errorf("token with ID %s is signed by account %s, cannot be added to account %s", token.ID(), token.Signature().SignerID, a.ID())
	}


	a.mbr += MBR_TOKEN_COST
	a.tokenStore[token.ID()] = token
	return nil
}

func (a *Account) removeToken(tokenId ScalegraphId) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.tokenStore[tokenId] == nil {
		return fmt.Errorf("token with ID %s does not exist in account %s", tokenId, a.ID())
	}
	delete(a.tokenStore, tokenId)
	return nil
}

func (a *Account ) burnToken(tokenId ScalegraphId) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.tokenStore[tokenId] == nil {
		return fmt.Errorf("token with ID %s does not exist in account %s", tokenId, a.ID())
	}

	if a.tokenStore[tokenId].Equal(&Token{}) {
		return fmt.Errorf("token with ID %s is not owned by account %s, cannot be burned", tokenId, a.ID())
	}

	tokenSignerID, err := ScalegraphIdFromString(a.tokenStore[tokenId].Signature().SignerID)
	if err != nil {
		return fmt.Errorf("invalid signer ID in token signature: %w", err)
	}
	if tokenSignerID != a.ID() {
		return fmt.Errorf("token with ID %s is signed by account %s, cannot be burned by account %s", tokenId, a.tokenStore[tokenId].Signature().SignerID, a.ID())
	}

	delete(a.tokenStore, tokenId)
	a.mbr -= MBR_TOKEN_COST
	return nil
}



func (a *Account) rollbacklatestTransaction(trx ITransaction) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.blockchain.Len() == 0 {
		return nil // No transactions to remove
	}

	latest := a.blockchain.Tail()
	if latest.ID() != trx.ID() {
		return fmt.Errorf("transaction to remove is not the latest transaction in the blockchain")
	}

	a.blockchain.removeLatestBlock()
	return nil
}

func (a *Account) appendTransaction(trx ITransaction) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch trx.Type() {
	case Mint:
		tx := trx.(*MintTransaction)
		a.balance += tx.Amount()
	case Transfer:
		tx := trx.(*TransferTransaction)
		if tx.Sender() != nil && tx.Sender().ID() == a.ID() {
			if (a.balance - a.mbr) < tx.Amount() {
				return fmt.Errorf("insufficient balance for transfer: current balance %.2f, mbr %.2f, transfer amount %.2f", a.balance, a.mbr, tx.Amount())
			}

			a.balance -= tx.Amount()
		}
		if tx.Receiver() != nil && tx.Receiver().ID() == a.ID() {
			a.balance += tx.Amount()
		}
	case Burn:
		tx := trx.(*BurnTransaction)
		if tx.Receiver() != nil && tx.Receiver().ID() == a.ID() {
			if (a.balance - a.mbr) < tx.Amount() {
				return fmt.Errorf("insufficient balance for burn, can not burn more than balance: current balance %.2f, mbr %.2f, burn amount %.2f", a.balance, a.mbr, tx.Amount())
			}
			a.balance -= tx.Amount()
		}
	case MintToken:
		tx := trx.(*MintTokenTransaction)
		if tx.Receiver() != nil && tx.Receiver().ID() == a.ID() {
			if err := a.addToken(tx.Token()); err != nil {
				return fmt.Errorf("failed to mint token: %w", err)
			}
		} else if tx.Sender() != nil && tx.Sender().ID() == a.ID() {
			return fmt.Errorf("sender should be nil for mint token transaction")
		}
	case TransferToken:
		tx := trx.(*TransferTokenTransaction)
		if tx.Sender() != nil && tx.Sender().ID() == a.ID() {
			_, exists := a.GetToken(tx.Token().ID())
			if !exists {
				return fmt.Errorf("token with ID %s does not exist in account %s", tx.Token().ID(), a.ID())
			}
			if err := a.removeToken(tx.Token().ID()); err != nil {
				return fmt.Errorf("failed to transfer token: %w", err)
			}
		} else if tx.Receiver() != nil && tx.Receiver().ID() == a.ID() {
			if err := a.addToken(tx.Token()); err != nil {
				return fmt.Errorf("failed to transfer token: %w", err)
			}
		} else {
			return fmt.Errorf("either sender or receiver must be this account for transfer token transaction")
		}
	case AuthorizeTokenTransfer:
		tx := trx.(*AuthorizeTokenTransferTransaction)

		if tx.Sender() != nil && tx.Receiver() != nil && tx.Sender().ID() == a.ID() && tx.Receiver().ID() == a.ID() {
			tokenId := *tx.TokenId()
			if a.tokenStore[tokenId] == nil {
				if (a.balance - a.mbr) < MBR_SLOT_COST {
					return fmt.Errorf("insufficient balance to authorize token transfer: current balance %.2f, mbr %.2f, required mbr for authorizing token transfer %.2f", a.balance, a.mbr, MBR_SLOT_COST)
				}

				a.mbr += MBR_SLOT_COST
				a.tokenStore[tokenId] = &Token{}
			} else if a.tokenStore[tokenId] != nil && !a.tokenStore[tokenId].Equal(&Token{}) {
				return fmt.Errorf("token with ID %s is already owned by account %s, cannot authorize transfer", tokenId, a.ID())
			}

			// If the token store entry is &Token{}, it means it's already authorized but not owned, so we can just return nil without error as we are authorizing again only
			return nil
		}
		return fmt.Errorf("both sender and receiver must be this account for authorize token transfer transaction")

	default:
		return fmt.Errorf("unsupported transaction type: %s", trx.Type())

	}

	a.blockchain.append(trx)
	return nil

}

func (a *Account) String() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return fmt.Sprintf("Account(ID: %s, Balance: %.2f, TokenStore: %v)", a.id, a.balance, a.tokenStore)
}
