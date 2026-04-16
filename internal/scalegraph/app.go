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
		if err := a.Mint(ctx, &MintRequest{To: acc.ID(), Amount: initialBalance}); err != nil {
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

func (a *App) GetAccountCertAndPublicKey(accountID ScalegraphId) (*x509.Certificate, ed25519.PublicKey, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	acc, exists := a.accounts[accountID]
	if !exists {
		return nil, nil, fmt.Errorf("account not found: %s", accountID)
	}

	return acc.Certificate(), acc.PublicKey(), nil
}

// GetAccounts returns all accounts
func (a *App) GetAccounts(ctx context.Context, req *GetAccountsRequest) (*GetAccountsResponse, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	accounts := make([]*Account, 0, len(a.accounts))
	for _, acc := range a.accounts {
		accounts = append(accounts, acc)
	}
	return &GetAccountsResponse{Accounts: accounts}, nil
}

// GetAccount returns an account by ID
func (a *App) GetAccount(ctx context.Context, req *GetAccountRequest) (*GetAccountResponse, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	acc, exists := a.accounts[req.AccountID]
	if !exists {
		return nil, fmt.Errorf("account not found: %s", req.AccountID)
	}

	return &GetAccountResponse{Account: acc}, nil
}

func (a *App) LookupToken(ctx context.Context, req *LookupTokenRequest) (*LookupTokenResponse, error) {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Lookup token operation initiated", "account_id", req.AccountID, "token_id", req.TokenID)
	a.mu.RLock()
	defer a.mu.RUnlock()

	acc, exist := a.accounts[req.AccountID]
	if !exist {
		logger.Warn("Account not found for token lookup", "account_id", req.AccountID)
		return &LookupTokenResponse{}, nil
	}

	token, exists := acc.GetToken(req.TokenID)
	if !exists {
		logger.Warn("Token not found in account", "token_id", req.TokenID, "account_id", req.AccountID)
		return nil, fmt.Errorf("token with ID %s, not found in account: %s", req.TokenID, req.AccountID)
	}

	return &LookupTokenResponse{Token: token}, nil
}

// Transfer transfers funds between two accounts atomically
func (a *App) Transfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error) {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Transfer initiated", "from", req.From, "to", req.To, "amount", req.Amount, "nonce", req.Nonce)
	a.mu.RLock()
	defer a.mu.RUnlock()

	fromAcc, exists := a.accounts[req.From]
	if !exists {
		logger.Warn("Source account not found", "from", req.From)
		return nil, fmt.Errorf("source account not found: %s", req.From)
	}

	toAcc, exists := a.accounts[req.To]
	if !exists {
		logger.Warn("Destination account not found", "to", req.To)
		return nil, fmt.Errorf("destination account not found: %s", req.To)
	}

	// Reject self-transfers
	if req.From == req.To {
		logger.Warn("Self-transfer not allowed", "account", req.From)
		return nil, fmt.Errorf("self-transfer not allowed")
	}

	// Validate nonce before proceeding
	expectedNonce := fromAcc.GetNonce()
	if req.Nonce != expectedNonce {
		logger.Warn("Nonce mismatch", "from", req.From, "expected", expectedNonce, "got", req.Nonce)
		return nil, fmt.Errorf("nonce mismatch: expected %d, got %d", expectedNonce, req.Nonce)
	}

	transferTx := newTransferTransaction(fromAcc, toAcc, req.Amount)

	if err := fromAcc.appendTransaction(transferTx); err != nil {
		logger.Error("Failed to append transaction", "error", err)
		return nil, err
	}

	if err := toAcc.appendTransaction(transferTx); err != nil {
		logger.Error("Failed to append transaction", "error", err)
		return nil, err
	}

	return &TransferResponse{}, nil
}

// Mint creates new funds in an account
func (a *App) Mint(ctx context.Context, req *MintRequest) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Mint operation initiated", "account_id", req.To, "amount", req.Amount)
	a.mu.RLock()
	defer a.mu.RUnlock()

	toAcc, exists := a.accounts[req.To]
	if !exists {
		logger.Warn("Account not found for mint", "account_id", req.To)
		return fmt.Errorf("destination account not found: %s", req.To)
	}

	mintTx := newMintTransaction(toAcc, req.Amount)

	toAcc.appendTransaction(mintTx)

	logger.Info("Mint completed", "account_id", req.To, "amount", req.Amount, "new_balance", toAcc.Balance())
	return nil
}

// MintToken mints a new token into an account
func (a *App) MintToken(ctx context.Context, req *MintTokenRequest) (*MintTokenResponse, error) {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}

	// Determine the target account from the signer
	signerID, err := ScalegraphIdFromString(req.SignedEnvelope.Signature.SignerID)
	if err != nil {
		return nil, fmt.Errorf("invalid signer ID: %w", err)
	}

	logger.Debug("Mint token operation initiated", "account_id", signerID, "token_value", req.TokenValue)
	a.mu.RLock()
	defer a.mu.RUnlock()

	toAcc, exists := a.accounts[signerID]
	if !exists {
		logger.Warn("Account not found for mint token", "account_id", signerID)
		return nil, fmt.Errorf("destination account not found: %s", signerID)
	}
	toAcc.GetNonce()

	token := newToken(req.TokenValue, req.SignedEnvelope.Signature, req.ClawbackAddress, req.FreezeAddress, req.Nonce)
	mintTokenTx := newMintTokenTransaction(toAcc, token)

	if err := toAcc.appendTransaction(mintTokenTx); err != nil {
		logger.Error("Failed to append mint token transaction", "error", err)
		return nil, err
	}

	logger.Info("Mint token completed", "account_id", signerID, "token_value", req.TokenValue)
	return &MintTokenResponse{TokenID: token.ID()}, nil
}

func (a *App) AuthorizeTokenTransfer(ctx context.Context, req *AuthorizeTokenTransferRequest) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Authorize token transfer operation initiated", "account_id", req.AccountID, "token_owner_id", req.TokenOwnerID, "token_id", req.TokenId)
	a.mu.RLock()
	defer a.mu.RUnlock()

	authorizerAcc, exists := a.accounts[req.AccountID]
	if !exists {
		logger.Warn("Authorizer account not found", "account_id", req.AccountID)
		return fmt.Errorf("account not found: %s", req.AccountID)
	}

	tokenOwnerAcc, exists := a.accounts[req.TokenOwnerID]
	if !exists {
		logger.Warn("Token owner account not found", "token_owner_id", req.TokenOwnerID)
		return fmt.Errorf("token owner account not found: %s", req.TokenOwnerID)
	}

	authorizeTx := newAuthorizeTokenTransferTransaction(authorizerAcc, tokenOwnerAcc, &req.TokenId)
	if err := authorizerAcc.appendTransaction(authorizeTx); err != nil {
		logger.Error("Failed to append authorize token transfer transaction (authorizer)", "error", err)
		return err
	}

	if err := tokenOwnerAcc.appendTransaction(authorizeTx); err != nil {
		logger.Error("Failed to append authorize token transfer transaction (token owner)", "error", err)
		authorizerAcc.rollbacklatestTransaction(authorizeTx)
		return err
	}

	logger.Info("Authorize token transfer completed", "account_id", req.AccountID, "token_owner_id", req.TokenOwnerID, "token_id", req.TokenId)
	return nil
}

func (a *App) UnauthorizeTokenTransfer(ctx context.Context, req *UnauthorizeTokenTransferRequest) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Unauthorize token transfer operation initiated", "account_id", req.AccountID, "token_owner_id", req.TokenOwnerID, "token_id", req.TokenId)
	a.mu.RLock()
	defer a.mu.RUnlock()

	authorizerAcc, exists := a.accounts[req.AccountID]
	if !exists {
		logger.Warn("Authorizer account not found", "account_id", req.AccountID)
		return fmt.Errorf("account not found: %s", req.AccountID)
	}

	tokenOwnerAcc, exists := a.accounts[req.TokenOwnerID]
	if !exists {
		logger.Warn("Token owner account not found", "token_owner_id", req.TokenOwnerID)
		return fmt.Errorf("token owner account not found: %s", req.TokenOwnerID)
	}

	unauthorizeTx := newUnauthorizeTokenTransferTransaction(authorizerAcc, tokenOwnerAcc, &req.TokenId)
	if err := authorizerAcc.appendTransaction(unauthorizeTx); err != nil {
		logger.Error("Failed to append unauthorize token transfer transaction (authorizer)", "error", err)
		return err
	}

	if err := tokenOwnerAcc.appendTransaction(unauthorizeTx); err != nil {
		logger.Error("Failed to append unauthorize token transfer transaction (token owner)", "error", err)
		authorizerAcc.rollbacklatestTransaction(unauthorizeTx)
		return err
	}

	logger.Info("Unauthorize token transfer completed", "account_id", req.AccountID, "token_owner_id", req.TokenOwnerID, "token_id", req.TokenId)
	return nil
}

// TransferToken transfers a token between accounts
func (a *App) TransferToken(ctx context.Context, req *TransferTokenRequest) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Transfer token operation initiated", "from", req.From, "to", req.To, "token_id", req.TokenId)
	a.mu.RLock()
	defer a.mu.RUnlock()

	fromAcc, exists := a.accounts[req.From]
	if !exists {
		logger.Warn("Source account not found for token transfer", "from", req.From)
		return fmt.Errorf("source account not found: %s", req.From)
	}

	toAcc, exists := a.accounts[req.To]
	if !exists {
		logger.Warn("Destination account not found for token transfer", "to", req.To)
		return fmt.Errorf("destination account not found: %s", req.To)
	}

	token, exists := fromAcc.GetToken(req.TokenId)
	if !exists {
		logger.Warn("Token not found in source account", "token_id", req.TokenId, "from_account", req.From)
		return fmt.Errorf("token not found in source account: %s", req.TokenId)
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

	logger.Info("Transfer token completed", "from", req.From, "to", req.To, "token_id", token.ID())
	return nil
}

// AccountCount returns the number of accounts
func (a *App) AccountCount(ctx context.Context, req *AccountCountRequest) (*AccountCountResponse, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return &AccountCountResponse{Count: len(a.accounts)}, nil
}

func (a *App) BurnToken(ctx context.Context, req *BurnTokenRequest) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Burn token operation initiated", "account_id", req.AccountID, "token_id", req.TokenId)
	a.mu.RLock()
	defer a.mu.RUnlock()

	acc, exists := a.accounts[req.AccountID]
	if !exists {
		logger.Warn("Account not found for burn token", "account_id", req.AccountID)
		return fmt.Errorf("account not found: %s", req.AccountID)
	}

	burnTx := newBurnTokenTransaction(acc, req.TokenId)
	if err := acc.appendTransaction(burnTx); err != nil {
		logger.Error("Failed to append burn token transaction", "error", err)
		return err
	}

	logger.Info("Burn token completed", "account_id", req.AccountID, "token_id", req.TokenId)
	return nil
}

func (a *App) ClawbackToken(ctx context.Context, req *ClawbackTokenRequest) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Clawback token operation initiated", "from", req.From, "to", req.To, "token_id", req.TokenId)
	a.mu.RLock()
	defer a.mu.RUnlock()

	fromAcc, exists := a.accounts[req.From]
	if !exists {
		logger.Warn("Source account not found for clawback token", "from", req.From)
		return fmt.Errorf("source account not found: %s", req.From)
	}

	toAcc, exists := a.accounts[req.To]
	if !exists {
		logger.Warn("Destination account not found for clawback token", "to", req.To)
		return fmt.Errorf("destination account not found: %s", req.To)
	}

	token, exists := fromAcc.GetToken(req.TokenId)
	if !exists {
		logger.Warn("Token not found in source account", "token_id", req.TokenId, "from_account", req.From)
		return fmt.Errorf("token not found in source account: %s", req.TokenId)
	}

	if token.ClawbackAddress().String() != toAcc.ID().String() {
		logger.Warn("Clawback address mismatch", "token_id", req.TokenId, "expected_clawback_address", token.ClawbackAddress(), "to_account", req.To)
		return fmt.Errorf("clawback address mismatch: token expects %s, but destination account is %s", token.ClawbackAddress(), req.To)
	}

	clawbackTx := newClawbackTokenTransaction(fromAcc, toAcc, *token)

	if err := fromAcc.appendTransaction(clawbackTx); err != nil {
		logger.Error("Failed to append clawback token transaction", "error", err)
		return err
	}

	if err := toAcc.appendTransaction(clawbackTx); err != nil {
		logger.Error("Failed to append clawback token transaction", "error", err)
		// Rollback: remove transaction from sender if receiver append fails
		fromAcc.rollbacklatestTransaction(clawbackTx)
		return err
	}

	logger.Info("Clawback token completed", "from", req.From, "to", req.To, "token_id", token.ID())
	return nil
}

func (a *App) FreezeToken(ctx context.Context, req *FreezeTokenRequest) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Freeze token operation initiated", "freeze_authority", req.FreezeAuthority, "token_holder", req.TokenHolder, "token_id", req.TokenId)
	a.mu.RLock()
	defer a.mu.RUnlock()

	authorityAcc, exists := a.accounts[req.FreezeAuthority]
	if !exists {
		return fmt.Errorf("freeze authority account not found: %s", req.FreezeAuthority)
	}

	holderAcc, exists := a.accounts[req.TokenHolder]
	if !exists {
		return fmt.Errorf("token holder account not found: %s", req.TokenHolder)
	}

	token, exists := holderAcc.GetToken(req.TokenId)
	if !exists {
		return fmt.Errorf("token not found in holder account: %s", req.TokenId)
	}

	if token.FreezeAddress() == nil || token.FreezeAddress().String() != req.FreezeAuthority.String() {
		return fmt.Errorf("account %s is not the freeze authority for token %s", req.FreezeAuthority, req.TokenId)
	}

	freezeTx := newFreezeTokenTransaction(authorityAcc, holderAcc, req.TokenId)

	if err := authorityAcc.appendTransaction(freezeTx); err != nil {
		logger.Error("Failed to append freeze token transaction to authority", "error", err)
		return err
	}

	if err := holderAcc.appendTransaction(freezeTx); err != nil {
		logger.Error("Failed to append freeze token transaction to holder", "error", err)
		authorityAcc.rollbacklatestTransaction(freezeTx)
		return err
	}

	logger.Info("Freeze token completed", "freeze_authority", req.FreezeAuthority, "token_holder", req.TokenHolder, "token_id", req.TokenId)
	return nil
}

func (a *App) UnfreezeToken(ctx context.Context, req *UnfreezeTokenRequest) error {
	logger := a.logger
	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	logger.Debug("Unfreeze token operation initiated", "freeze_authority", req.FreezeAuthority, "token_holder", req.TokenHolder, "token_id", req.TokenId)
	a.mu.RLock()
	defer a.mu.RUnlock()

	authorityAcc, exists := a.accounts[req.FreezeAuthority]
	if !exists {
		return fmt.Errorf("freeze authority account not found: %s", req.FreezeAuthority)
	}

	holderAcc, exists := a.accounts[req.TokenHolder]
	if !exists {
		return fmt.Errorf("token holder account not found: %s", req.TokenHolder)
	}

	token, exists := holderAcc.GetToken(req.TokenId)
	if !exists {
		return fmt.Errorf("token not found in holder account: %s", req.TokenId)
	}

	if token.FreezeAddress() == nil || token.FreezeAddress().String() != req.FreezeAuthority.String() {
		return fmt.Errorf("account %s is not the freeze authority for token %s", req.FreezeAuthority, req.TokenId)
	}

	unfreezeTx := newUnfreezeTokenTransaction(authorityAcc, holderAcc, req.TokenId)

	if err := authorityAcc.appendTransaction(unfreezeTx); err != nil {
		logger.Error("Failed to append unfreeze token transaction to authority", "error", err)
		return err
	}

	if err := holderAcc.appendTransaction(unfreezeTx); err != nil {
		logger.Error("Failed to append unfreeze token transaction to holder", "error", err)
		authorityAcc.rollbacklatestTransaction(unfreezeTx)
		return err
	}

	logger.Info("Unfreeze token completed", "freeze_authority", req.FreezeAuthority, "token_holder", req.TokenHolder, "token_id", req.TokenId)
	return nil
}
