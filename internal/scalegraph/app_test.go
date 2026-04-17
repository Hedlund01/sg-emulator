package scalegraph

import (
	"encoding/pem"
	"testing"

	sgcrypto "sg-emulator/internal/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	app := testApp()

	require.NotNil(t, app)
	assert.NotNil(t, app.accounts, "accounts map should be initialized")
	assert.Equal(t, 0, len(app.accounts))
}

func TestCreateAccountWithKeys(t *testing.T) {
	app := testApp()

	// Test creating account with zero balance
	acc1 := createTestAccountInApp(t, app, 0)
	require.NotNil(t, acc1)
	assert.Equal(t, 0.0, acc1.Balance())

	// Test creating account with initial balance
	acc2 := createTestAccountInApp(t, app, 100.0)
	require.NotNil(t, acc2)
	assert.Equal(t, 100.0, acc2.Balance())

	// Test that accounts are stored
	countResp, err := app.AccountCount(testCtx(), &AccountCountRequest{})
	require.NoError(t, err)
	assert.Equal(t, 2, countResp.Count)

	// Test that accounts have unique IDs
	assert.NotEqual(t, acc1.ID(), acc2.ID(), "accounts should have unique IDs")
}

func TestCreateAccountWithKeysDuplicateKey(t *testing.T) {
	app := testApp()

	pubKey, _, cert := testKeyPairAndCert(t)

	// Create first account
	_, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, 0)
	require.NoError(t, err)

	// Attempt to create account with same public key should fail
	_, err = app.CreateAccountWithKeys(testCtx(), pubKey, cert, 0)
	assert.Error(t, err, "should reject duplicate public key")
}

func TestCreateAccountIDDerivedFromPublicKey(t *testing.T) {
	app := testApp()

	pubKey, _, cert := testKeyPairAndCert(t)

	acc, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, 0)
	require.NoError(t, err)

	expectedID := ScalegraphIdFromPublicKey(pubKey)
	assert.Equal(t, expectedID, acc.ID(), "account ID should be derived from public key")
}

func TestGetAccounts(t *testing.T) {
	app := testApp()

	// Test empty app
	resp, err := app.GetAccounts(testCtx(), &GetAccountsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Accounts)

	// Create some accounts
	acc1 := createTestAccountInApp(t, app, 50.0)
	acc2 := createTestAccountInApp(t, app, 100.0)
	acc3 := createTestAccountInApp(t, app, 150.0)

	resp, err = app.GetAccounts(testCtx(), &GetAccountsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Accounts, 3)

	// Verify all accounts are present (order doesn't matter)
	ids := map[ScalegraphId]bool{
		acc1.ID(): true,
		acc2.ID(): true,
		acc3.ID(): true,
	}

	for _, acc := range resp.Accounts {
		assert.True(t, ids[acc.ID()], "unexpected account ID: %s", acc.ID())
	}
}

func TestGetAccount(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 100.0)

	// Test getting existing account
	resp, err := app.GetAccount(testCtx(), &GetAccountRequest{AccountID: acc.ID()})
	require.NoError(t, err)
	assert.Equal(t, acc.ID(), resp.Account.ID())
	assert.Equal(t, 100.0, resp.Account.Balance())

	// Test getting non-existent account
	fakeID, _ := NewScalegraphId()
	_, err = app.GetAccount(testCtx(), &GetAccountRequest{AccountID: fakeID})
	assert.Error(t, err, "should error for non-existent account")
}

func TestAccountCount(t *testing.T) {
	app := testApp()

	resp, err := app.AccountCount(testCtx(), &AccountCountRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Count)

	createTestAccountInApp(t, app, 10.0)
	resp, err = app.AccountCount(testCtx(), &AccountCountRequest{})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Count)

	createTestAccountInApp(t, app, 20.0)
	createTestAccountInApp(t, app, 30.0)
	resp, err = app.AccountCount(testCtx(), &AccountCountRequest{})
	require.NoError(t, err)
	assert.Equal(t, 3, resp.Count)
}

func TestTransfer(t *testing.T) {
	app := testApp()
	acc1 := createTestAccountInApp(t, app, 100.0)
	acc2 := createTestAccountInApp(t, app, 50.0)

	// Nonce = current outgoingTxCount (fresh account has nonce 0, first transfer uses 0)
	nonce := acc1.GetNonce()

	// Test successful transfer
	_, err := app.Transfer(testCtx(), &TransferRequest{From: acc1.ID(), To: acc2.ID(), Amount: 30.0, Nonce: nonce})
	require.NoError(t, err)

	assert.Equal(t, 70.0, acc1.Balance(), "sender balance after transfer")
	assert.Equal(t, 80.0, acc2.Balance(), "receiver balance after transfer")

	// Test transfer with insufficient funds
	nonce = acc1.GetNonce()
	_, err = app.Transfer(testCtx(), &TransferRequest{From: acc1.ID(), To: acc2.ID(), Amount: 100.0, Nonce: nonce})
	assert.Error(t, err, "should error for insufficient funds")

	// Verify balances unchanged after failed transfer
	assert.Equal(t, 70.0, acc1.Balance(), "sender balance should not change after failed transfer")
	assert.Equal(t, 80.0, acc2.Balance(), "receiver balance should not change after failed transfer")

	// Test transfer from non-existent account
	fakeID, _ := NewScalegraphId()
	_, err = app.Transfer(testCtx(), &TransferRequest{From: fakeID, To: acc2.ID(), Amount: 10.0, Nonce: 0})
	assert.Error(t, err, "should error for non-existent sender")

	// Test transfer to non-existent account
	nonce = acc1.GetNonce()
	_, err = app.Transfer(testCtx(), &TransferRequest{From: acc1.ID(), To: fakeID, Amount: 10.0, Nonce: nonce})
	assert.Error(t, err, "should error for non-existent receiver")
}

func TestTransferSelfTransfer(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 100.0)

	nonce := acc.GetNonce()
	_, err := app.Transfer(testCtx(), &TransferRequest{From: acc.ID(), To: acc.ID(), Amount: 10.0, Nonce: nonce})
	assert.Error(t, err, "self-transfer should not be allowed")
	assert.Equal(t, 100.0, acc.Balance(), "balance should not change after rejected self-transfer")
}

func TestTransferZeroAmount(t *testing.T) {
	app := testApp()
	acc1 := createTestAccountInApp(t, app, 100.0)
	acc2 := createTestAccountInApp(t, app, 50.0)

	nonce := acc1.GetNonce()
	_, err := app.Transfer(testCtx(), &TransferRequest{From: acc1.ID(), To: acc2.ID(), Amount: 0, Nonce: nonce})
	require.NoError(t, err)

	assert.Equal(t, 100.0, acc1.Balance(), "sender balance should not change for zero transfer")
	assert.Equal(t, 50.0, acc2.Balance(), "receiver balance should not change for zero transfer")
}

func TestTransferNonceMismatch(t *testing.T) {
	app := testApp()
	acc1 := createTestAccountInApp(t, app, 100.0)
	acc2 := createTestAccountInApp(t, app, 50.0)

	// Use wrong nonce
	_, err := app.Transfer(testCtx(), &TransferRequest{From: acc1.ID(), To: acc2.ID(), Amount: 10.0, Nonce: 999})
	assert.Error(t, err, "should error for nonce mismatch")
	assert.Equal(t, 100.0, acc1.Balance(), "balance should not change after nonce mismatch")
}

func TestAuthorizeTokenTransferNonceMismatch(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)
	authorizer := createTestAccountInApp(t, app, MBR_SLOT_COST)

	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "authorize-nonce", nil, nil)

	err = app.AuthorizeTokenTransfer(testCtx(), &AuthorizeTokenTransferRequest{
		AccountID:    authorizer.ID(),
		TokenOwnerID: holder.ID(),
		TokenId:      tokenID,
		Nonce:        999,
	})
	assert.Error(t, err, "should error for nonce mismatch")
}

func TestUnauthorizeTokenTransferNonceMismatch(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)
	authorizer := createTestAccountInApp(t, app, MBR_SLOT_COST)

	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "unauthorize-nonce", nil, nil)

	err = app.AuthorizeTokenTransfer(testCtx(), &AuthorizeTokenTransferRequest{
		AccountID:    authorizer.ID(),
		TokenOwnerID: holder.ID(),
		TokenId:      tokenID,
		Nonce:        authorizer.GetNonce(),
	})
	require.NoError(t, err)

	err = app.UnauthorizeTokenTransfer(testCtx(), &UnauthorizeTokenTransferRequest{
		AccountID:    authorizer.ID(),
		TokenOwnerID: holder.ID(),
		TokenId:      tokenID,
		Nonce:        999,
	})
	assert.Error(t, err, "should error for nonce mismatch")
}

func TestTransferTokenNonceMismatch(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)
	receiver := createTestAccountInApp(t, app, 0)

	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "transfer-token-nonce", nil, nil)

	err = app.TransferToken(testCtx(), &TransferTokenRequest{
		From:    holder.ID(),
		To:      receiver.ID(),
		TokenId: tokenID,
		Nonce:   999,
	})
	assert.Error(t, err, "should error for nonce mismatch")
}

func TestBurnTokenNonceMismatch(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)
	owner, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, owner, "burn-token-nonce", nil, nil)

	err = app.BurnToken(testCtx(), &BurnTokenRequest{AccountID: owner.ID(), TokenId: tokenID, Nonce: 999})
	assert.Error(t, err, "should error for nonce mismatch")
}

func TestClawbackTokenNonceMismatch(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)
	authority := createTestAccountInApp(t, app, 0)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	authorityID := authority.ID()
	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "clawback-nonce", &authorityID, nil)

	err = app.ClawbackToken(testCtx(), &ClawbackTokenRequest{
		From:    holder.ID(),
		To:      authority.ID(),
		TokenId: tokenID,
		Nonce:   999,
	})
	assert.Error(t, err, "should error for nonce mismatch")
}

func TestFreezeTokenNonceMismatch(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)
	authority := createTestAccountInApp(t, app, MBR_FREEZE_COST)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	authorityID := authority.ID()
	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "freeze-nonce", nil, &authorityID)

	err = app.FreezeToken(testCtx(), &FreezeTokenRequest{
		FreezeAuthority: authority.ID(),
		TokenHolder:     holder.ID(),
		TokenId:         tokenID,
		Nonce:           999,
	})
	assert.Error(t, err, "should error for nonce mismatch")
}

func TestUnfreezeTokenNonceMismatch(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)
	authority := createTestAccountInApp(t, app, MBR_FREEZE_COST)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	authorityID := authority.ID()
	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "unfreeze-nonce", nil, &authorityID)

	err = app.FreezeToken(testCtx(), &FreezeTokenRequest{
		FreezeAuthority: authority.ID(),
		TokenHolder:     holder.ID(),
		TokenId:         tokenID,
		Nonce:           authority.GetNonce(),
	})
	require.NoError(t, err)

	err = app.UnfreezeToken(testCtx(), &UnfreezeTokenRequest{
		FreezeAuthority: authority.ID(),
		TokenHolder:     holder.ID(),
		TokenId:         tokenID,
		Nonce:           999,
	})
	assert.Error(t, err, "should error for nonce mismatch")
}

func TestMint(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 100.0)

	// Test minting funds
	err := app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: 50.0})
	require.NoError(t, err)
	assert.Equal(t, 150.0, acc.Balance())

	// Test minting to non-existent account
	fakeID, _ := NewScalegraphId()
	err = app.Mint(testCtx(), &MintRequest{To: fakeID, Amount: 10.0})
	assert.Error(t, err, "should error for non-existent account")
}

func TestTransferAtomicity(t *testing.T) {
	app := testApp()
	acc1 := createTestAccountInApp(t, app, 100.0)
	acc2 := createTestAccountInApp(t, app, 50.0)

	initialTotal := acc1.Balance() + acc2.Balance()

	// Successful transfer should preserve total balance
	nonce := acc1.GetNonce()
	_, err := app.Transfer(testCtx(), &TransferRequest{From: acc1.ID(), To: acc2.ID(), Amount: 25.0, Nonce: nonce})
	require.NoError(t, err)

	finalTotal := acc1.Balance() + acc2.Balance()
	assert.Equal(t, initialTotal, finalTotal, "total balance should be preserved after transfer")

	// Failed transfer should also preserve balances
	beforeAcc1 := acc1.Balance()
	beforeAcc2 := acc2.Balance()

	nonce = acc1.GetNonce()
	_, err = app.Transfer(testCtx(), &TransferRequest{From: acc1.ID(), To: acc2.ID(), Amount: 1000.0, Nonce: nonce})
	assert.Error(t, err, "transfer should fail for insufficient funds")

	assert.Equal(t, beforeAcc1, acc1.Balance(), "sender balance should not change after failed transfer")
	assert.Equal(t, beforeAcc2, acc2.Balance(), "receiver balance should not change after failed transfer")
}

func TestConcurrentAccountCreation(t *testing.T) {
	app := testApp()

	runConcurrent(t, 100, func(i int) {
		createTestAccountInApp(t, app, float64(i))
	})

	resp, err := app.AccountCount(testCtx(), &AccountCountRequest{})
	require.NoError(t, err)
	assert.Equal(t, 100, resp.Count)
}

// --- Token end-to-end tests ---

func TestMintTokenEndToEnd(t *testing.T) {
	// Regression: minting a token via App.MintToken crashed due to:
	//   1. tokenStore never initialized (nil map panic)
	//   2. addToken guard inverted (token rejected even when slot was authorized)
	//   3. newMintTokenTransaction set sender = receiver instead of nil
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	// Create account with enough balance to cover MBR_TOKEN_COST
	acc, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	// Build a real signed envelope (same path as the TUI/REST transport)
	payload := &sgcrypto.MintTokenPayload{TokenValue: "hello-token", Nonce: 0}
	certDER := cert.Raw
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	envelope, err := sgcrypto.CreateSignedEnvelope(payload, privKey, acc.ID().String(), certPEM)
	require.NoError(t, err)

	req := &MintTokenRequest{
		TokenValue:      "hello-token",
		ClawbackAddress: nil,
		Nonce:           0,
		SignedEnvelope:  envelope,
	}

	resp, err := app.MintToken(testCtx(), req)
	require.NoError(t, err, "MintToken should not return an error")
	assert.NotEmpty(t, resp.TokenID, "response should contain a non-empty token ID")

	// The token must be retrievable from the account
	token, ok := acc.GetToken(resp.TokenID)
	assert.True(t, ok, "GetToken should return true for the minted token")
	assert.NotNil(t, token, "GetToken should return the minted token, not nil")
	assert.Equal(t, "hello-token", token.Value(), "token value should match the minted value")
}

func TestMintTokenEndToEndWithClawback(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	acc, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	// Use the account itself as the clawback address
	clawbackID := acc.ID()

	clawbackStr := clawbackID.String()
	payload := &sgcrypto.MintTokenPayload{TokenValue: "clawback-token", ClawbackAddress: &clawbackStr, Nonce: 0}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
	envelope, err := sgcrypto.CreateSignedEnvelope(payload, privKey, acc.ID().String(), certPEM)
	require.NoError(t, err)

	req := &MintTokenRequest{
		TokenValue:      "clawback-token",
		ClawbackAddress: &clawbackID,
		Nonce:           0,
		SignedEnvelope:  envelope,
	}

	resp, err := app.MintToken(testCtx(), req)
	require.NoError(t, err, "MintToken with clawback address should not return an error")
	assert.NotEmpty(t, resp.TokenID)

	token, ok := acc.GetToken(resp.TokenID)
	assert.True(t, ok)
	require.NotNil(t, token)
	assert.Equal(t, "clawback-token", token.Value())
	require.NotNil(t, token.ClawbackAddress(), "clawback address should be set")
	assert.Equal(t, clawbackID, *token.ClawbackAddress())
}

func TestMintTokenEndToEndWithFreezeAddress(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	acc, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	freezeID := acc.ID() // use same account as freeze authority for simplicity
	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, acc, "freeze-token", nil, &freezeID)

	token, ok := acc.GetToken(tokenID)
	assert.True(t, ok)
	require.NotNil(t, token)
	require.NotNil(t, token.FreezeAddress(), "freeze address should be set")
	assert.Equal(t, freezeID, *token.FreezeAddress())
}

func TestMintTokenInsufficientBalanceForMBR(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	// Create account with 0 balance — cannot cover MBR_TOKEN_COST
	acc, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, 0)
	require.NoError(t, err)

	payload := &sgcrypto.MintTokenPayload{TokenValue: "some-token", Nonce: 0}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
	envelope, err := sgcrypto.CreateSignedEnvelope(payload, privKey, acc.ID().String(), certPEM)
	require.NoError(t, err)

	req := &MintTokenRequest{
		TokenValue:     "some-token",
		SignedEnvelope: envelope,
	}

	_, err = app.MintToken(testCtx(), req)
	assert.Error(t, err, "MintToken should fail when account has insufficient balance for MBR")
	assert.Empty(t, acc.GetTokens(), "no tokens should be stored after a failed mint")
}

// --- BurnToken app-level tests ---

func TestBurnTokenEndToEnd(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	acc, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, acc, "burn-token", nil, nil)

	mbrBeforeBurn := acc.mbr

	accID := acc.ID()
	err = app.BurnToken(testCtx(), &BurnTokenRequest{AccountID: accID, TokenId: tokenID, Nonce: acc.GetNonce()})
	require.NoError(t, err)

	_, ok := acc.GetToken(tokenID)
	assert.False(t, ok, "token should be gone after burn")
	assert.Equal(t, mbrBeforeBurn-MBR_TOKEN_COST, acc.mbr, "MBR should decrease after burn")
}

// --- UnauthorizeTokenTransfer app-level tests ---

func TestUnauthorizeTokenTransferEndToEnd(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)
	authorizer := createTestAccountInApp(t, app, MBR_SLOT_COST)

	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "unauth-token", nil, nil)

	// Authorize
	err = app.AuthorizeTokenTransfer(testCtx(), &AuthorizeTokenTransferRequest{
		AccountID:    authorizer.ID(),
		TokenOwnerID: holder.ID(),
		TokenId:      tokenID,
		Nonce:        authorizer.GetNonce(),
	})
	require.NoError(t, err)

	mbrAfterAuth := authorizer.mbr

	// Unauthorize
	err = app.UnauthorizeTokenTransfer(testCtx(), &UnauthorizeTokenTransferRequest{
		AccountID:    authorizer.ID(),
		TokenOwnerID: holder.ID(),
		TokenId:      tokenID,
		Nonce:        authorizer.GetNonce(),
	})
	require.NoError(t, err)

	assert.Empty(t, authorizer.GetTokens(), "authorizer should have no tokens after unauthorize")
	assert.Nil(t, authorizer.tokenStore[tokenID], "placeholder slot should be removed")
	assert.Equal(t, mbrAfterAuth-MBR_SLOT_COST, authorizer.mbr, "MBR should decrease after unauthorize")
}

// --- ClawbackToken app-level tests ---

func TestClawbackTokenEndToEnd(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	// Clawback uses addToken (no pre-authorization slot required on receiver side).
	authority := createTestAccountInApp(t, app, 0)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	authorityID := authority.ID()
	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "clawback-token", &authorityID, nil)

	// Clawback: from=holder, to=authority — no prior authorization needed
	err = app.ClawbackToken(testCtx(), &ClawbackTokenRequest{
		From:    holder.ID(),
		To:      authority.ID(),
		TokenId: tokenID,
		Nonce:   authority.GetNonce(),
	})
	require.NoError(t, err)

	_, holderHasToken := holder.GetToken(tokenID)
	assert.False(t, holderHasToken, "holder should not have token after clawback")
	_, authorityHasToken := authority.GetToken(tokenID)
	assert.True(t, authorityHasToken, "authority should have token after clawback")
}

func TestClawbackTokenRejectsWrongAuthority(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	realAuthority := createTestAccountInApp(t, app, 0)
	wrongAuthority := createTestAccountInApp(t, app, 0)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	realAuthorityID := realAuthority.ID()
	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "clawback-token", &realAuthorityID, nil)

	err = app.ClawbackToken(testCtx(), &ClawbackTokenRequest{
		From:    holder.ID(),
		To:      wrongAuthority.ID(),
		TokenId: tokenID,
	})
	assert.Error(t, err, "clawback with wrong authority should fail")
}

// --- FreezeToken / UnfreezeToken app-level tests ---

func TestFreezeTokenEndToEnd(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	authority := createTestAccountInApp(t, app, MBR_FREEZE_COST)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	authorityID := authority.ID()
	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "freeze-token", nil, &authorityID)

	mbrBefore := authority.mbr

	// Freeze
	err = app.FreezeToken(testCtx(), &FreezeTokenRequest{
		FreezeAuthority: authority.ID(),
		TokenHolder:     holder.ID(),
		TokenId:         tokenID,
		Nonce:           authority.GetNonce(),
	})
	require.NoError(t, err)

	tok, ok := holder.GetToken(tokenID)
	require.True(t, ok)
	assert.True(t, tok.Frozen(), "token should be frozen")
	assert.Equal(t, mbrBefore+MBR_FREEZE_COST, authority.mbr, "authority MBR should increase")

	// Transfer should fail while frozen
	err = app.TransferToken(testCtx(), &TransferTokenRequest{
		From:    holder.ID(),
		To:      authority.ID(),
		TokenId: tokenID,
		Nonce:   holder.GetNonce(),
	})
	assert.Error(t, err, "transfer of frozen token should fail")

	// Unfreeze
	err = app.UnfreezeToken(testCtx(), &UnfreezeTokenRequest{
		FreezeAuthority: authority.ID(),
		TokenHolder:     holder.ID(),
		TokenId:         tokenID,
		Nonce:           authority.GetNonce(),
	})
	require.NoError(t, err)

	tok, ok = holder.GetToken(tokenID)
	require.True(t, ok)
	assert.False(t, tok.Frozen(), "token should be unfrozen")
	assert.Equal(t, mbrBefore, authority.mbr, "authority MBR should be restored after unfreeze")
}

func TestFreezeTokenRejectsWrongAuthority(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	realAuthority := createTestAccountInApp(t, app, 0)
	wrongAuthority := createTestAccountInApp(t, app, MBR_FREEZE_COST)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	realAuthorityID := realAuthority.ID()
	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "freeze-token", nil, &realAuthorityID)

	err = app.FreezeToken(testCtx(), &FreezeTokenRequest{
		FreezeAuthority: wrongAuthority.ID(),
		TokenHolder:     holder.ID(),
		TokenId:         tokenID,
	})
	assert.Error(t, err, "freeze by wrong authority should fail")
}

func TestUnfreezeTokenRejectsWhenNotFrozen(t *testing.T) {
	app := testApp()
	pubKey, privKey, cert := testKeyPairAndCert(t)

	authority := createTestAccountInApp(t, app, MBR_FREEZE_COST)
	holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
	require.NoError(t, err)

	authorityID := authority.ID()
	tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "freeze-token", nil, &authorityID)

	err = app.UnfreezeToken(testCtx(), &UnfreezeTokenRequest{
		FreezeAuthority: authority.ID(),
		TokenHolder:     holder.ID(),
		TokenId:         tokenID,
	})
	assert.Error(t, err, "unfreeze of non-frozen token should fail")
}
