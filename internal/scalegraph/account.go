package scalegraph

import (
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"sync"
)

// txHandlerFunc is a function that handles a specific transaction type for an account.
// After a successful handler call, the transaction is appended to the account's blockchain.
// Handlers that manage their own blockchain appending (e.g. AuthorizeTokenTransfer) should return errSkipAppend.
type txHandlerFunc func(trx ITransaction) error

// txRollbackFunc is a function that reverses the state changes made by the corresponding txHandlerFunc.
// It is called by rollbacklatestTransaction after the block has been removed from the blockchain.
// Rollback handlers must not acquire a.mu (the caller already holds it).
type txRollbackFunc func(trx ITransaction) error

// errSkipAppend is a sentinel error returned by a txHandlerFunc to signal that
// the handler has already managed blockchain state and the caller should skip the
// default a.blockchain.append(trx) step.
type errSkipAppend struct{}

func (errSkipAppend) Error() string { return "skip blockchain append" }

// registerTxHandler registers a typed transaction handler for a specific transaction type.
// The handler receives the concrete transaction type T.
func registerTxHandler[T ITransaction](handlers map[TransactionType]txHandlerFunc, txType TransactionType, handler func(trx T) error) {
	handlers[txType] = func(trx ITransaction) error {
		typed, ok := trx.(T)
		if !ok {
			return fmt.Errorf("invalid transaction type: got %T, want %T", trx, *new(T))
		}
		return handler(typed)
	}
}

// registerTxRollbackHandler registers a typed rollback handler for a specific transaction type.
// The handler receives the concrete transaction type T and reverses the state changes of the apply handler.
func registerTxRollbackHandler[T ITransaction](handlers map[TransactionType]txRollbackFunc, txType TransactionType, handler func(trx T) error) {
	handlers[txType] = func(trx ITransaction) error {
		typed, ok := trx.(T)
		if !ok {
			return fmt.Errorf("invalid transaction type for rollback: got %T, want %T", trx, *new(T))
		}
		return handler(typed)
	}
}

const (
	// MBR (Minimum Balance Requirement) = MBR_SLOT_COST * number of authorized token transfers (i.e. number of &Token{} entries in the token store) + MBR_TOKEN_COST * number of tokens created by the account. Each token creation or authorization of token transfer will require the account to have at least MBR_SLOT_COST balance as MBR, which will be unfrozen when the burn token transaction is executed or the unauthorize token transfer transaction is executed. This is to prevent accounts to create DoS token transactions by authorizing unlimited token transfers or token creation transactions, which can cause the blockchain to grow indefinitely and consume all memory.

	MBR_SLOT_COST = 0.5 // Each token transfer transaction will require the sender to have at least this balance as MBR, which will unfreeze when the unauthorize token transfer transaction is executed or the token is received.

	MBR_TOKEN_COST = 1.0 // Each token creation transaction will require the sender to have at least this balance as MBR, which will unfreeze when the burn token transaction is executed.

)

type Account struct {
	mu                 sync.RWMutex
	id                 ScalegraphId
	balance            float64
	mbr                float64
	blockchain         IBlockchain
	valuestore         map[string]string
	publicKey          ed25519.PublicKey
	certificate        *x509.Certificate
	tokenStore         map[string]*Token // &Token{} means authorized but not owned, nil means not authorized, non-nil means owned
	txHandlers         map[TransactionType]txHandlerFunc
	txRollbackHandlers map[TransactionType]txRollbackFunc
	outgoingTxCount    uint64 // number of Transfer transactions sent by this account
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
		tokenStore:  make(map[string]*Token),
		publicKey:   pubKey,
		certificate: cert,
	}
	acc.registerTxHandlers()
	acc.registerTxRollbackHandlers()

	return acc, nil
}

func (a *Account) registerTxHandlers() {
	a.txHandlers = make(map[TransactionType]txHandlerFunc)
	registerTxHandler(a.txHandlers, Mint, a.handleMintTx)
	registerTxHandler(a.txHandlers, Transfer, a.handleTransferTx)
	registerTxHandler(a.txHandlers, Burn, a.handleBurnTx)
	registerTxHandler(a.txHandlers, MintToken, a.handleMintTokenTx)
	registerTxHandler(a.txHandlers, TransferToken, a.handleTransferTokenTx)
	registerTxHandler(a.txHandlers, AuthorizeTokenTransfer, a.handleAuthorizeTokenTransferTx)
	registerTxHandler(a.txHandlers, UnauthorizeTokenTransfer, a.handleUnauthorizeTokenTransferTx)
	registerTxHandler(a.txHandlers, BurnToken, a.handleBurnTokenTx)
	registerTxHandler(a.txHandlers, ClawbackTokenTransfer, a.handleClawbackTokenTx)
}

func (a *Account) registerTxRollbackHandlers() {
	a.txRollbackHandlers = make(map[TransactionType]txRollbackFunc)
	registerTxRollbackHandler(a.txRollbackHandlers, Mint, a.rollbackMintTx)
	registerTxRollbackHandler(a.txRollbackHandlers, Transfer, a.rollbackTransferTx)
	registerTxRollbackHandler(a.txRollbackHandlers, Burn, a.rollbackBurnTx)
	registerTxRollbackHandler(a.txRollbackHandlers, MintToken, a.rollbackMintTokenTx)
	registerTxRollbackHandler(a.txRollbackHandlers, TransferToken, a.rollbackTransferTokenTx)
	registerTxRollbackHandler(a.txRollbackHandlers, AuthorizeTokenTransfer, a.rollbackAuthorizeTokenTransferTx)
	registerTxRollbackHandler(a.txRollbackHandlers, UnauthorizeTokenTransfer, a.rollbackUnauthorizeTokenTransferTx)
	// BurnToken does not need a rollback handler because a burn token transaction only touches one account.
	registerTxRollbackHandler(a.txRollbackHandlers, ClawbackTokenTransfer, a.rollbackClawbackTokenTx)
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

// MBR returns the account's current Minimum Balance Requirement
func (a *Account) MBR() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mbr
}

// Blockchain returns the account's blockchain
func (a *Account) Blockchain() IBlockchain {
	return a.blockchain
}

// GetNonce returns the number of outgoing Transfer transactions sent by this account.
// The next Transfer from this account must use nonce = GetNonce() + 1.
func (a *Account) GetNonce() uint64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.outgoingTxCount
}

// GetToken returns the token with the given ID if it exists in the account's token store.
// Returns the token and true if found, or nil and false if not found.
// Returns nil and false if the token is not authorized for this account (i.e. token store entry is &Token{}), or if the token does not exist (i.e. token store entry is nil).
func (a *Account) GetToken(tokenId string) (*Token, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	token := a.tokenStore[tokenId]
	if token == nil || token.Equal(&Token{}) {
		return nil, false
	}
	return token, true
}

// GetTokens returns all fully owned tokens in this account's token store.
// It excludes tokens that are only authorized-but-not-owned (represented by &Token{} entries).
func (a *Account) GetTokens() []*Token {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var tokens []*Token
	for _, t := range a.tokenStore {
		if t != nil && !t.Equal(&Token{}) {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

// mintToken adds a newly minted token directly into the account.
// Unlike addTokenFromTransfer, no pre-authorized &Token{} slot is required.
func (a *Account) mintToken(token *Token) error {
	if a.tokenStore[token.ID()] != nil {
		return fmt.Errorf("token with ID %s already exists in account %s", token.ID(), a.ID())
	}

	signerID, err := ScalegraphIdFromString(token.Signature().SignerID)
	if err != nil {
		return fmt.Errorf("invalid signer ID in token signature: %w", err)
	}
	if signerID != a.ID() {
		return fmt.Errorf("token with ID %s is signed by account %s, cannot be minted to account %s", token.ID(), token.Signature().SignerID, a.ID())
	}

	if (a.balance - a.mbr) < MBR_TOKEN_COST {
		return fmt.Errorf("insufficient balance to mint token: current balance %.2f, mbr %.2f, required %.2f", a.balance, a.mbr, MBR_TOKEN_COST)
	}

	a.mbr += MBR_TOKEN_COST
	a.tokenStore[token.ID()] = token
	return nil
}

// addTokenFromTransfer adds a token received via a transfer.
// The receiving account must have previously authorized the transfer by setting
// a &Token{} placeholder for this token ID.
func (a *Account) addTokenFromTransfer(token *Token) error {
	slot := a.tokenStore[token.ID()]
	if slot == nil || !slot.Equal(&Token{}) {
		return fmt.Errorf("token with ID %s is not authorized to transfer to account %s", token.ID(), a.ID())
	}

	a.tokenStore[token.ID()] = token
	a.mbr -= MBR_SLOT_COST // Unfreeze the MBR for the authorized slot, which is now occupied by the received token.
	return nil
}

func (a *Account) removeToken(tokenId string) error {

	if a.tokenStore[tokenId] == nil {
		return fmt.Errorf("token with ID %s does not exist in account %s", tokenId, a.ID())
	}
	delete(a.tokenStore, tokenId)
	return nil
}

func (a *Account) addToken(token *Token) error {

	if a.tokenStore[token.ID()] != nil {
		return fmt.Errorf("token with ID %s already exists in account %s", token.ID(), a.ID())
	}

	a.tokenStore[token.ID()] = token
	return nil
}

func (a *Account) burnToken(tokenId string) error {

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
	if latest.Transaction() == nil || latest.Transaction().ID() != trx.ID() {
		return fmt.Errorf("transaction to remove is not the latest transaction in the blockchain")
	}

	rollback, ok := a.txRollbackHandlers[trx.Type()]
	if !ok {
		return fmt.Errorf("no rollback handler registered for transaction type: %s", trx.Type())
	}

	a.blockchain.removeLatestBlock()

	if err := rollback(trx); err != nil {
		return fmt.Errorf("rollback handler failed for transaction type %s, latest block removed: %w", trx.Type(), err)
	}
	return nil
}

func (a *Account) appendTransaction(trx ITransaction) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	handler, ok := a.txHandlers[trx.Type()]
	if !ok {
		return fmt.Errorf("unsupported transaction type: %s", trx.Type())
	}

	if err := handler(trx); err != nil {
		if _, skip := err.(errSkipAppend); skip {
			return nil
		}
		return err
	}

	a.blockchain.append(trx)
	return nil
}

func (a *Account) handleMintTx(trx *MintTransaction) error {
	a.balance += trx.Amount()
	return nil
}

func (a *Account) handleTransferTx(trx *TransferTransaction) error {
	if trx.Sender() != nil && trx.Sender().ID() == a.ID() {
		if (a.balance - a.mbr) < trx.Amount() {
			return fmt.Errorf("insufficient balance for transfer: current balance %.2f, mbr %.2f, transfer amount %.2f", a.balance, a.mbr, trx.Amount())
		}
		a.balance -= trx.Amount()
		a.outgoingTxCount++
	}
	if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		a.balance += trx.Amount()
	}
	return nil
}

func (a *Account) handleBurnTx(trx *BurnTransaction) error {
	if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		if (a.balance - a.mbr) < trx.Amount() {
			return fmt.Errorf("insufficient balance for burn, can not burn more than balance: current balance %.2f, mbr %.2f, burn amount %.2f", a.balance, a.mbr, trx.Amount())
		}
		a.balance -= trx.Amount()
	}
	return nil
}

func (a *Account) handleMintTokenTx(trx *MintTokenTransaction) error {
	if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		if err := a.mintToken(trx.Token()); err != nil {
			return fmt.Errorf("failed to mint token: %w", err)
		}
	} else if trx.Sender() != nil && trx.Sender().ID() == a.ID() {
		return fmt.Errorf("sender should be nil for mint token transaction")
	}
	return nil
}

func (a *Account) handleTransferTokenTx(trx *TransferTokenTransaction) error {
	if trx.Sender() != nil && trx.Sender().ID() == a.ID() {
		_, ok := a.tokenStore[trx.Token().ID()]
		if !ok {
			return fmt.Errorf("token with ID %s does not exist in account %s", trx.Token().ID(), a.ID())
		}
		if err := a.removeToken(trx.Token().ID()); err != nil {
			return fmt.Errorf("failed to remove token: %w", err)
		}
	} else if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		if err := a.addTokenFromTransfer(trx.Token()); err != nil {
			return fmt.Errorf("failed to add token: %w", err)
		}
	} else {
		return fmt.Errorf("either sender or receiver must be this account for transfer token transaction")
	}
	return nil
}

func (a *Account) handleAuthorizeTokenTransferTx(trx *AuthorizeTokenTransferTransaction) error {
	if trx.Sender() == nil || trx.Receiver() == nil || trx.Sender().ID() != a.ID() || trx.Receiver().ID() != a.ID() {
		return fmt.Errorf("both sender and receiver must be this account for authorize token transfer transaction")
	}

	tokenId := *trx.TokenId()
	if a.tokenStore[tokenId] == nil {
		if (a.balance - a.mbr) < MBR_SLOT_COST {
			return fmt.Errorf("insufficient balance to authorize token transfer: current balance %.2f, mbr %.2f, required mbr for authorizing token transfer %.2f", a.balance, a.mbr, MBR_SLOT_COST)
		}
		a.mbr += MBR_SLOT_COST
		a.tokenStore[tokenId] = &Token{}
	} else if !a.tokenStore[tokenId].Equal(&Token{}) {
		return fmt.Errorf("token with ID %s is already owned by account %s, cannot authorize transfer", tokenId, a.ID())
	}
	return nil
}

func (a *Account) handleUnauthorizeTokenTransferTx(trx *UnauthorizeTokenTransferTransaction) error {
	if trx.Sender() == nil || trx.Receiver() == nil || trx.Sender().ID() != a.ID() || trx.Receiver().ID() != a.ID() {
		return fmt.Errorf("both sender and receiver must be this account for unauthorize token transfer transaction")
	}

	tokenId := *trx.TokenId()
	slot, ok := a.tokenStore[tokenId]
	if !ok {
		return fmt.Errorf("token with ID %s is not authorized for transfer to account %s, cannot unauthorize", tokenId, a.ID())
	}
	if !slot.Equal(&Token{}) {
		return fmt.Errorf("token with ID %s is already owned by account %s, cannot unauthorize transfer", tokenId, a.ID())
	}

	delete(a.tokenStore, tokenId)
	a.mbr -= MBR_SLOT_COST
	return nil
}

func (a *Account) handleBurnTokenTx(trx *BurnTokenTransaction) error {
	if trx.Sender() != nil && trx.Sender().ID() == a.ID() {
		if err := a.burnToken(trx.TokenID()); err != nil {
			return fmt.Errorf("failed to burn token: %w", err)
		}
	} else if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		return fmt.Errorf("receiver should be nil for burn token transaction")
	} else {
		return fmt.Errorf("sender must be this account for burn token transaction")
	}
	return nil
}

func (a *Account) handleClawbackTokenTx(trx *ClawbackTokenTransaction) error {
	token := trx.Token()
	if trx.Sender() != nil && trx.Sender().ID() == a.ID() {
		_, ok := a.tokenStore[token.ID()]
		if !ok {
			return fmt.Errorf("token with ID %s does not exist in account %s", token.ID(), a.ID())
		}
		if err := a.removeToken(token.ID()); err != nil {
			return fmt.Errorf("failed to remove token during clawback: %w", err)
		}
	} else if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		if err := a.addToken(&token); err != nil {
			return fmt.Errorf("failed to add token during clawback: %w", err)
		}
	} else {
		return fmt.Errorf("either from or to must be this account for clawback token transaction")
	}
	return nil
}

// --- Rollback handlers ---
// These mirror their handleXxx counterparts and must be called with a.mu already held.

func (a *Account) rollbackMintTx(trx *MintTransaction) error {
	a.balance -= trx.Amount()
	return nil
}

func (a *Account) rollbackTransferTx(trx *TransferTransaction) error {
	if trx.Sender() != nil && trx.Sender().ID() == a.ID() {
		a.balance += trx.Amount()
		a.outgoingTxCount--
	}
	if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		a.balance -= trx.Amount()
	}
	return nil
}

func (a *Account) rollbackBurnTx(trx *BurnTransaction) error {
	if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		a.balance += trx.Amount()
	}
	return nil
}

// rollbackMintTokenTx reverses a MintToken: removes the token and decrements mbr.
// MBR is decremented here because mintToken incremented it; no burn transaction is created.
func (a *Account) rollbackMintTokenTx(trx *MintTokenTransaction) error {
	if trx.Receiver() == nil || trx.Receiver().ID() != a.ID() {
		return nil
	}
	tokenId := trx.Token().ID()
	if a.tokenStore[tokenId] == nil {
		return fmt.Errorf("cannot rollback mint token: token %s not found in account %s", tokenId, a.ID())
	}
	delete(a.tokenStore, tokenId)
	a.mbr -= MBR_TOKEN_COST
	return nil
}

// rollbackTransferTokenTx reverses a TransferToken.
// Sender: restore the token pointer back into tokenStore (no MBR change; removeToken did not touch MBR).
// Receiver: revert the slot back to the &Token{} placeholder and decrement mbr
//
//	(addTokenFromTransfer replaced the placeholder with the real token and incremented mbr).
func (a *Account) rollbackTransferTokenTx(trx *TransferTokenTransaction) error {
	if trx.Sender() != nil && trx.Sender().ID() == a.ID() {
		a.tokenStore[trx.Token().ID()] = trx.Token()
	} else if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		a.tokenStore[trx.Token().ID()] = &Token{}
		a.mbr -= MBR_TOKEN_COST
	}
	return nil
}

// rollbackAuthorizeTokenTransferTx reverses an AuthorizeTokenTransfer:
// removes the &Token{} placeholder and decrements mbr by MBR_SLOT_COST.
func (a *Account) rollbackAuthorizeTokenTransferTx(trx *AuthorizeTokenTransferTransaction) error {
	tokenId := *trx.TokenId()
	delete(a.tokenStore, tokenId)
	a.mbr -= MBR_SLOT_COST
	return nil
}

func (a *Account) rollbackUnauthorizeTokenTransferTx(trx *UnauthorizeTokenTransferTransaction) error {
	tokenId := *trx.TokenId()
	a.tokenStore[tokenId] = &Token{}
	a.mbr += MBR_SLOT_COST
	return nil
}

func (a *Account) rollbackClawbackTokenTx(trx *ClawbackTokenTransaction) error {
	token := trx.Token()
	if trx.Sender() != nil && trx.Sender().ID() == a.ID() {
		if err := a.addToken(&token); err != nil {
			return fmt.Errorf("failed to add token during clawback rollback: %w", err)
		}
	} else if trx.Receiver() != nil && trx.Receiver().ID() == a.ID() {
		if err := a.removeToken(token.ID()); err != nil {
			return fmt.Errorf("failed to remove token during clawback rollback: %w", err)
		}
	}
	return nil
}

func (a *Account) String() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return fmt.Sprintf("Account(ID: %s, Balance: %.2f, TokenStore: %v)", a.id, a.balance, a.tokenStore)
}
