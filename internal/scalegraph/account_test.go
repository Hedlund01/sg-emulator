package scalegraph

import (
	"testing"

	sgcrypto "sg-emulator/internal/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccountWithPublicKey(t *testing.T) {
	acc, _ := testCreateAccount(t)

	assert.NotNil(t, acc)
	assert.Equal(t, 0.0, acc.Balance(), "initial balance should be 0")
	assert.NotNil(t, acc.Blockchain(), "blockchain should be initialized")
	assert.NotEqual(t, ScalegraphId{}, acc.ID(), "account ID should not be zero value")
	assert.NotNil(t, acc.PublicKey(), "public key should be set")
	assert.NotNil(t, acc.Certificate(), "certificate should be set")
}

func TestAccountIDUniqueness(t *testing.T) {
	acc1, acc2 := testCreateTwoAccounts(t)

	assert.NotEqual(t, acc1.ID(), acc2.ID(), "two accounts should have unique IDs")

	// Test that ID is consistent
	id := acc1.ID()
	assert.Equal(t, id, acc1.ID(), "account ID should be consistent between calls")
}

func TestAccountIDDerivedFromPublicKey(t *testing.T) {
	acc, kp := testCreateAccount(t)

	expectedID := ScalegraphIdFromPublicKey(kp.PublicKey)
	assert.Equal(t, expectedID, acc.ID(), "account ID should be derived from public key")
}

func TestAccountBalance(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 0)

	// Test initial balance
	assert.Equal(t, 0.0, acc.Balance())

	// Mint via app
	err := app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: 100.0})
	require.NoError(t, err)
	assert.Equal(t, 100.0, acc.Balance())

	// Test balance is thread-safe (multiple concurrent reads)
	runConcurrent(t, 10, func(i int) {
		_ = acc.Balance()
	})
}

func TestAccountMintViaApp(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 0)

	// Test minting positive amount
	err := app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: 50.0})
	require.NoError(t, err)
	assert.Equal(t, 50.0, acc.Balance())

	// Test multiple mints
	err = app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: 30.0})
	require.NoError(t, err)
	err = app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: 20.0})
	require.NoError(t, err)
	assert.Equal(t, 100.0, acc.Balance())

	// Test that mint creates transaction in blockchain
	blocks := acc.Blockchain().GetBlocks()
	assert.Greater(t, len(blocks), 1, "should have blocks after mint (genesis + mint)")
}

func TestAccountMintZero(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 0)

	err := app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: 0})
	require.NoError(t, err)
	assert.Equal(t, 0.0, acc.Balance())
}

func TestAccountBlockchain(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 0)

	blockchain := acc.Blockchain()
	require.NotNil(t, blockchain)

	// Test that blockchain is consistent (same instance)
	assert.Equal(t, blockchain, acc.Blockchain(), "Blockchain() should return same instance")

	// Test that minting adds to blockchain
	initialLen := blockchain.Len()
	err := app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: 100.0})
	require.NoError(t, err)
	assert.Greater(t, blockchain.Len(), initialLen, "mint should add block to blockchain")
}

func TestAccountString(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 123.45)

	str := acc.String()
	assert.NotEmpty(t, str, "String() should return non-empty string")
	assert.GreaterOrEqual(t, len(acc.ID().String()), 8, "ID string should be at least 8 chars")
}

func TestAccountGetNonce(t *testing.T) {
	app := testApp()
	acc1 := createTestAccountInApp(t, app, 100)
	acc2 := createTestAccountInApp(t, app, 0)

	assert.Equal(t, uint64(0), acc1.GetNonce(), "fresh account nonce should be 0")

	// Mint must NOT change nonce
	err := app.Mint(testCtx(), &MintRequest{To: acc1.ID(), Amount: 50.0})
	require.NoError(t, err)
	assert.Equal(t, uint64(0), acc1.GetNonce(), "nonce must not change after incoming mint")

	// Outgoing transfer must increment nonce
	_, err = app.Transfer(testCtx(), &TransferRequest{
		From: acc1.ID(), To: acc2.ID(), Amount: 1.0, Nonce: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), acc1.GetNonce(), "nonce should be 1 after first outgoing transfer")
	assert.Equal(t, uint64(0), acc2.GetNonce(), "receiver nonce must not change")
}

func TestAccountConcurrentMint(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 0)

	runConcurrent(t, 100, func(i int) {
		err := app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: 1.0})
		assert.NoError(t, err, "concurrent mint %d failed", i)
	})

	assert.Equal(t, 100.0, acc.Balance(), "balance should be 100.0 after 100 concurrent mints of 1.0")
}

func TestAccountAppendTransactionMint(t *testing.T) {
	acc, _ := testCreateAccount(t)

	mintTx := newMintTransaction(acc, 75.0)
	err := acc.appendTransaction(mintTx)
	require.NoError(t, err)
	assert.Equal(t, 75.0, acc.Balance())
}

func TestAccountAppendTransactionTransfer(t *testing.T) {
	sender, receiver := testCreateTwoAccounts(t)

	// Give sender some balance first
	mintTx := newMintTransaction(sender, 100.0)
	err := sender.appendTransaction(mintTx)
	require.NoError(t, err)

	// Transfer from sender
	transferTx := newTransferTransaction(sender, receiver, 30.0)
	err = sender.appendTransaction(transferTx)
	require.NoError(t, err)
	assert.Equal(t, 70.0, sender.Balance())

	// Receiver side
	err = receiver.appendTransaction(transferTx)
	require.NoError(t, err)
	assert.Equal(t, 30.0, receiver.Balance())
}

func TestAccountAppendTransactionInsufficientBalance(t *testing.T) {
	sender, receiver := testCreateTwoAccounts(t)

	// Sender has 0 balance
	transferTx := newTransferTransaction(sender, receiver, 50.0)
	err := sender.appendTransaction(transferTx)
	assert.Error(t, err, "should fail with insufficient balance")
	assert.Equal(t, 0.0, sender.Balance(), "balance should not change after failed transfer")
}

func TestAccountAppendTransactionBurn(t *testing.T) {
	acc, _ := testCreateAccount(t)

	// Give account some balance
	mintTx := newMintTransaction(acc, 100.0)
	err := acc.appendTransaction(mintTx)
	require.NoError(t, err)

	// Burn some tokens
	burnTx := newBurnTransaction(acc, 40.0)
	err = acc.appendTransaction(burnTx)
	require.NoError(t, err)
	assert.Equal(t, 60.0, acc.Balance())
}

func TestAccountAppendTransactionBurnInsufficientBalance(t *testing.T) {
	acc, _ := testCreateAccount(t)

	burnTx := newBurnTransaction(acc, 10.0)
	err := acc.appendTransaction(burnTx)
	assert.Error(t, err, "should fail when burning more than balance")
	assert.Equal(t, 0.0, acc.Balance())
}

// --- Token tests ---

func TestAccountTokenStoreInitialized(t *testing.T) {
	// Regression: tokenStore was never initialized in newAccountWithPublicKey,
	// causing a nil-map panic on any token operation.
	acc, kp := testCreateAccount(t)

	require.NotPanics(t, func() {
		testMintTokenIntoAccount(t, acc, kp.PrivateKey)
	}, "minting a token should not panic")
}

func TestGetTokenReturnsNilForPlaceholder(t *testing.T) {
	// Regression: GetToken guard was inverted — it returned the &Token{} placeholder
	// (authorized-but-not-owned) as if it were a real token.
	acc, _ := testCreateAccount(t)
	tokenId := "placeholder-token-id"
	testAuthorizeTokenTransfer(t, acc, tokenId)

	token, ok := acc.GetToken(tokenId)
	assert.False(t, ok, "GetToken should return false for a placeholder slot")
	assert.Nil(t, token, "GetToken should return nil for a placeholder slot")
}

func TestGetTokenReturnsTokenWhenOwned(t *testing.T) {
	// Regression: the same inverted guard in GetToken prevented real tokens from
	// being found after a successful mint.
	acc, kp := testCreateAccount(t)
	minted := testMintTokenIntoAccount(t, acc, kp.PrivateKey)

	token, ok := acc.GetToken(minted.ID())
	assert.True(t, ok, "GetToken should return true for an owned token")
	assert.Equal(t, minted, token, "GetToken should return the minted token")
}

func TestGetTokensExcludesPlaceholders(t *testing.T) {
	acc, kp := testCreateAccount(t)

	// Authorize a slot but don't mint — the store only has a placeholder.
	// We need a token ID to authorize; use one derived from a token we'll create
	// but NOT append.
	fakeToken := testCreateToken(t, acc, kp.PrivateKey)
	testAuthorizeTokenTransfer(t, acc, fakeToken.ID())

	tokens := acc.GetTokens()
	assert.Empty(t, tokens, "GetTokens should not return placeholder slots")
}

func TestAuthorizeTokenTransferRequiresSufficientBalance(t *testing.T) {
	// Account with 0 balance cannot cover MBR_SLOT_COST.
	acc, _ := testCreateAccount(t)
	tokenId := "some-token-id"

	tx := newAuthorizeTokenTransferTransaction(acc, &tokenId)
	err := acc.appendTransaction(tx)
	assert.Error(t, err, "authorizing a token slot with 0 balance should fail")
	assert.Equal(t, 0.0, acc.Balance(), "balance should be unchanged after failed authorization")
}

func TestMintTokenIncreasesBlockchainLength(t *testing.T) {
	acc, kp := testCreateAccount(t)
	initialLen := acc.Blockchain().Len()

	testMintTokenIntoAccount(t, acc, kp.PrivateKey)

	// The helper appends a mint-balance tx + a mint-token tx, so length grows by 2.
	assert.Equal(t, initialLen+2, acc.Blockchain().Len(), "blockchain should grow by 2 after minting a token")
}

// --- Rollback handler tests ---

func TestRollbackMintTokenRestoresState(t *testing.T) {
	acc, kp := testCreateAccount(t)

	// Give balance for two token mints.
	require.NoError(t, acc.appendTransaction(newMintTransaction(acc, MBR_TOKEN_COST*2)))

	// Mint first token.
	tok1 := testCreateToken(t, acc, kp.PrivateKey)
	mintTx1 := newMintTokenTransaction(acc, tok1)
	require.NoError(t, acc.appendTransaction(mintTx1))

	mbrAfterFirstMint := acc.mbr
	lenAfterFirstMint := acc.Blockchain().Len()

	// Mint a second token with a different value so it gets a different ID.
	nonce2 := int64(acc.GetNonce())
	payload2 := &sgcrypto.MintTokenPayload{TokenValue: "other-value", Nonce: nonce2}
	sig2, err := sgcrypto.Sign(payload2, kp.PrivateKey, acc.ID().String())
	require.NoError(t, err)
	tok2 := newToken("other-value", *sig2, nil, nil, nonce2)
	mintTx2 := newMintTokenTransaction(acc, tok2)
	require.NoError(t, acc.appendTransaction(mintTx2))
	require.Equal(t, 2, len(acc.GetTokens()), "should have two tokens before rollback")

	// Roll back the second mint.
	require.NoError(t, acc.rollbacklatestTransaction(mintTx2))

	assert.Equal(t, lenAfterFirstMint, acc.Blockchain().Len(), "blockchain length restored")
	assert.Equal(t, mbrAfterFirstMint, acc.mbr, "mbr restored after rollback of second mint")
	assert.Equal(t, 1, len(acc.GetTokens()), "token count restored to 1")
	_, ok := acc.GetToken(tok2.ID())
	assert.False(t, ok, "rolled-back token should not be present")
	_, ok = acc.GetToken(tok1.ID())
	assert.True(t, ok, "first token should still be present")
}

func TestRollbackAuthorizeTokenTransferRestoresState(t *testing.T) {
	acc, _ := testCreateAccount(t)
	// Give enough balance to cover MBR_SLOT_COST.
	require.NoError(t, acc.appendTransaction(newMintTransaction(acc, 10.0)))

	tokenId := "some-token-id"
	authTx := newAuthorizeTokenTransferTransaction(acc, &tokenId)
	require.NoError(t, acc.appendTransaction(authTx))

	assert.Equal(t, MBR_SLOT_COST, acc.mbr, "mbr should equal MBR_SLOT_COST after authorization")
	assert.NotNil(t, acc.tokenStore[tokenId], "placeholder slot should exist after authorization")

	require.NoError(t, acc.rollbacklatestTransaction(authTx))

	assert.Equal(t, 0.0, acc.mbr, "mbr should be 0 after rolling back authorization")
	assert.Nil(t, acc.tokenStore[tokenId], "placeholder slot should be removed after rollback")
}

func TestRollbackTransferTokenRestoresSenderToken(t *testing.T) {
	// This is the core regression: a failed transfer (receiver not authorized)
	// must leave the sender's token intact.
	sender, kp := testCreateAccount(t)
	receiver, _ := testCreateAccount(t)

	// Give sender balance and mint a token directly (no helper so we control the tail).
	require.NoError(t, sender.appendTransaction(newMintTransaction(sender, MBR_TOKEN_COST)))
	tok := testCreateToken(t, sender, kp.PrivateKey)
	mintTx := newMintTokenTransaction(sender, tok)
	require.NoError(t, sender.appendTransaction(mintTx))

	// Attempt a transfer without the receiver having an authorization slot.
	transferTx := newTransferTokenTransaction(sender, receiver, tok)

	// Sender side succeeds (removes token from tokenStore).
	require.NoError(t, sender.appendTransaction(transferTx))
	_, senderHasToken := sender.GetToken(tok.ID())
	assert.False(t, senderHasToken, "token should be gone from sender mid-transfer")

	// Receiver side fails because no authorization slot exists.
	err := receiver.appendTransaction(transferTx)
	require.Error(t, err, "receiver should reject transfer without authorization slot")

	// Roll back the sender.
	require.NoError(t, sender.rollbacklatestTransaction(transferTx))

	_, senderHasToken = sender.GetToken(tok.ID())
	assert.True(t, senderHasToken, "token should be restored to sender after rollback")
	assert.Equal(t, tok, sender.tokenStore[tok.ID()], "restored token should be the original pointer")
}
