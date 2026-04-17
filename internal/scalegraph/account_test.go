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
	t.Run("currency transfer increments sender only", func(t *testing.T) {
		app := testApp()
		sender := createTestAccountInApp(t, app, 100)
		receiver := createTestAccountInApp(t, app, 0)

		assert.Equal(t, uint64(0), sender.GetNonce(), "fresh account nonce should be 0")

		err := app.Mint(testCtx(), &MintRequest{To: sender.ID(), Amount: 50.0})
		require.NoError(t, err)
		assert.Equal(t, uint64(0), sender.GetNonce(), "nonce must not change after incoming mint")

		_, err = app.Transfer(testCtx(), &TransferRequest{
			From: sender.ID(), To: receiver.ID(), Amount: 1.0, Nonce: 0,
		})
		require.NoError(t, err)
		assert.Equal(t, uint64(1), sender.GetNonce(), "nonce should increment for outgoing transfer")
		assert.Equal(t, uint64(0), receiver.GetNonce(), "incoming transfer must not increment receiver nonce")
	})

	t.Run("authorize transfer increments authorizer only", func(t *testing.T) {
		app := testApp()
		pubKey, privKey, cert := testKeyPairAndCert(t)
		owner, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
		require.NoError(t, err)
		authorizer := createTestAccountInApp(t, app, MBR_SLOT_COST)

		tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, owner, "authorize-nonce", nil, nil)
		ownerNonceBefore := owner.GetNonce()
		authorizerNonceBefore := authorizer.GetNonce()
		err = app.AuthorizeTokenTransfer(testCtx(), &AuthorizeTokenTransferRequest{
			AccountID:    authorizer.ID(),
			TokenOwnerID: owner.ID(),
			TokenId:      tokenID,
			Nonce:        authorizer.GetNonce(),
		})
		require.NoError(t, err)
		assert.Equal(t, authorizerNonceBefore+1, authorizer.GetNonce(), "authorizer nonce should increment")
		assert.Equal(t, ownerNonceBefore, owner.GetNonce(), "token owner nonce should not increment on incoming side")
	})

	t.Run("unauthorize transfer increments authorizer only", func(t *testing.T) {
		app := testApp()
		pubKey, privKey, cert := testKeyPairAndCert(t)
		owner, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
		require.NoError(t, err)
		authorizer := createTestAccountInApp(t, app, MBR_SLOT_COST)

		tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, owner, "unauthorize-nonce", nil, nil)
		ownerNonceBefore := owner.GetNonce()
		err = app.AuthorizeTokenTransfer(testCtx(), &AuthorizeTokenTransferRequest{
			AccountID:    authorizer.ID(),
			TokenOwnerID: owner.ID(),
			TokenId:      tokenID,
			Nonce:        authorizer.GetNonce(),
		})
		require.NoError(t, err)
		authorizerNonceAfterAuthorize := authorizer.GetNonce()

		err = app.UnauthorizeTokenTransfer(testCtx(), &UnauthorizeTokenTransferRequest{
			AccountID:    authorizer.ID(),
			TokenOwnerID: owner.ID(),
			TokenId:      tokenID,
			Nonce:        authorizer.GetNonce(),
		})
		require.NoError(t, err)
		assert.Equal(t, authorizerNonceAfterAuthorize+1, authorizer.GetNonce(), "authorizer nonce should increment again")
		assert.Equal(t, ownerNonceBefore, owner.GetNonce(), "token owner nonce should remain unchanged")
	})

	t.Run("transfer token increments sender only", func(t *testing.T) {
		app := testApp()
		pubKey, privKey, cert := testKeyPairAndCert(t)
		sender, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
		require.NoError(t, err)
		receiver := createTestAccountInApp(t, app, MBR_SLOT_COST)

		tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, sender, "transfer-token-nonce", nil, nil)
		senderNonceBefore := sender.GetNonce()
		err = app.AuthorizeTokenTransfer(testCtx(), &AuthorizeTokenTransferRequest{
			AccountID:    receiver.ID(),
			TokenOwnerID: sender.ID(),
			TokenId:      tokenID,
			Nonce:        receiver.GetNonce(),
		})
		require.NoError(t, err)
		receiverNonceBeforeIncoming := receiver.GetNonce()

		err = app.TransferToken(testCtx(), &TransferTokenRequest{
			From:    sender.ID(),
			To:      receiver.ID(),
			TokenId: tokenID,
			Nonce:   sender.GetNonce(),
		})
		require.NoError(t, err)
		assert.Equal(t, senderNonceBefore+1, sender.GetNonce(), "sender nonce should increment after transfer token")
		assert.Equal(t, receiverNonceBeforeIncoming, receiver.GetNonce(), "receiver nonce should not increment on incoming transfer")
	})

	t.Run("burn token increments owner", func(t *testing.T) {
		app := testApp()
		pubKey, privKey, cert := testKeyPairAndCert(t)
		owner, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
		require.NoError(t, err)

		tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, owner, "burn-token-nonce", nil, nil)
		ownerNonceBefore := owner.GetNonce()
		err = app.BurnToken(testCtx(), &BurnTokenRequest{AccountID: owner.ID(), TokenId: tokenID, Nonce: owner.GetNonce()})
		require.NoError(t, err)
		assert.Equal(t, ownerNonceBefore+1, owner.GetNonce(), "owner nonce should increment after burn token")
	})

	t.Run("clawback increments authority only", func(t *testing.T) {
		app := testApp()
		pubKey, privKey, cert := testKeyPairAndCert(t)
		authority := createTestAccountInApp(t, app, 0)
		holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
		require.NoError(t, err)

		authorityID := authority.ID()
		tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "clawback-token-nonce", &authorityID, nil)
		holderNonceBefore := holder.GetNonce()
		authorityNonceBefore := authority.GetNonce()
		err = app.ClawbackToken(testCtx(), &ClawbackTokenRequest{
			From:    holder.ID(),
			To:      authority.ID(),
			TokenId: tokenID,
			Nonce:   authority.GetNonce(),
		})
		require.NoError(t, err)
		assert.Equal(t, authorityNonceBefore+1, authority.GetNonce(), "authority nonce should increment after clawback")
		assert.Equal(t, holderNonceBefore, holder.GetNonce(), "holder nonce should not increment on incoming clawback")
	})

	t.Run("freeze and unfreeze increment authority only", func(t *testing.T) {
		app := testApp()
		pubKey, privKey, cert := testKeyPairAndCert(t)
		authority := createTestAccountInApp(t, app, MBR_FREEZE_COST)
		holder, err := app.CreateAccountWithKeys(testCtx(), pubKey, cert, MBR_TOKEN_COST)
		require.NoError(t, err)

		authorityID := authority.ID()
		tokenID := testMintTokenWithAddressesInApp(t, app, privKey, cert, holder, "freeze-unfreeze-nonce", nil, &authorityID)
		holderNonceBefore := holder.GetNonce()
		authorityNonceBefore := authority.GetNonce()

		err = app.FreezeToken(testCtx(), &FreezeTokenRequest{
			FreezeAuthority: authority.ID(),
			TokenHolder:     holder.ID(),
			TokenId:         tokenID,
			Nonce:           authority.GetNonce(),
		})
		require.NoError(t, err)
		assert.Equal(t, authorityNonceBefore+1, authority.GetNonce(), "authority nonce should increment after freeze")
		assert.Equal(t, holderNonceBefore, holder.GetNonce(), "holder nonce should not increment on freeze receiver side")
		authorityNonceBeforeUnfreeze := authority.GetNonce()

		err = app.UnfreezeToken(testCtx(), &UnfreezeTokenRequest{
			FreezeAuthority: authority.ID(),
			TokenHolder:     holder.ID(),
			TokenId:         tokenID,
			Nonce:           authority.GetNonce(),
		})
		require.NoError(t, err)
		assert.Equal(t, authorityNonceBeforeUnfreeze+1, authority.GetNonce(), "authority nonce should increment after unfreeze")
		assert.Equal(t, holderNonceBefore, holder.GetNonce(), "holder nonce should remain unchanged on unfreeze receiver side")
	})
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
	authorizer, _ := testCreateAccount(t)
	tokenOwner, kp := testCreateAccount(t)
	minted := testMintTokenIntoAccount(t, tokenOwner, kp.PrivateKey)

	testAuthorizeTokenTransfer(t, authorizer, tokenOwner, minted.ID())

	token, ok := authorizer.GetToken(minted.ID())
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
	authorizer, _ := testCreateAccount(t)
	tokenOwner, kp := testCreateAccount(t)

	// Mint a token into the owner, then authorize transfer to authorizer.
	minted := testMintTokenIntoAccount(t, tokenOwner, kp.PrivateKey)
	testAuthorizeTokenTransfer(t, authorizer, tokenOwner, minted.ID())

	// Authorizer only has a placeholder slot, not an owned token.
	tokens := authorizer.GetTokens()
	assert.Empty(t, tokens, "GetTokens should not return placeholder slots")
}

func TestAuthorizeTokenTransferRequiresSufficientBalance(t *testing.T) {
	// Account with 0 balance cannot cover MBR_SLOT_COST.
	authorizer, _ := testCreateAccount(t)
	tokenOwner, kp := testCreateAccount(t)
	minted := testMintTokenIntoAccount(t, tokenOwner, kp.PrivateKey)

	mintedID := minted.ID()
	tx := newAuthorizeTokenTransferTransaction(authorizer, tokenOwner, &mintedID, authorizer.GetNonce())
	err := authorizer.appendTransaction(tx)
	assert.Error(t, err, "authorizing a token slot with 0 balance should fail")
	assert.Equal(t, 0.0, authorizer.Balance(), "balance should be unchanged after failed authorization")
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
	authorizer, _ := testCreateAccount(t)
	tokenOwner, kp := testCreateAccount(t)
	minted := testMintTokenIntoAccount(t, tokenOwner, kp.PrivateKey)
	tokenId := minted.ID()

	// Give enough balance to cover MBR_SLOT_COST.
	require.NoError(t, authorizer.appendTransaction(newMintTransaction(authorizer, 10.0)))

	authTx := newAuthorizeTokenTransferTransaction(authorizer, tokenOwner, &tokenId, authorizer.GetNonce())
	require.NoError(t, authorizer.appendTransaction(authTx))

	assert.Equal(t, MBR_SLOT_COST, authorizer.mbr, "mbr should equal MBR_SLOT_COST after authorization")
	assert.NotNil(t, authorizer.tokenStore[tokenId], "placeholder slot should exist after authorization")

	require.NoError(t, authorizer.rollbacklatestTransaction(authTx))

	assert.Equal(t, 0.0, authorizer.mbr, "mbr should be 0 after rolling back authorization")
	assert.Nil(t, authorizer.tokenStore[tokenId], "placeholder slot should be removed after rollback")
}

func TestAuthorizeTokenTransferAppendsToOwnerBlockchain(t *testing.T) {
	authorizer, _ := testCreateAccount(t)
	tokenOwner, kp := testCreateAccount(t)
	minted := testMintTokenIntoAccount(t, tokenOwner, kp.PrivateKey)

	ownerInitialLen := tokenOwner.Blockchain().Len()
	authorizerInitialLen := authorizer.Blockchain().Len()

	testAuthorizeTokenTransfer(t, authorizer, tokenOwner, minted.ID())

	// Both blockchains should have grown (authorizer also got a mint for MBR)
	assert.Greater(t, tokenOwner.Blockchain().Len(), ownerInitialLen, "token owner blockchain should grow after authorize")
	assert.Greater(t, authorizer.Blockchain().Len(), authorizerInitialLen, "authorizer blockchain should grow after authorize")
}

func TestAuthorizeTokenTransferFailsIfTokenNotOnOwner(t *testing.T) {
	authorizer, _ := testCreateAccount(t)
	tokenOwner, _ := testCreateAccount(t)
	// Give authorizer balance so MBR check passes
	require.NoError(t, authorizer.appendTransaction(newMintTransaction(authorizer, MBR_SLOT_COST)))

	nonExistentID := "nonexistent-token-id"
	tx := newAuthorizeTokenTransferTransaction(authorizer, tokenOwner, &nonExistentID, authorizer.GetNonce())

	// Authorizer side succeeds (balance is fine)
	require.NoError(t, authorizer.appendTransaction(tx))

	// Token owner side should fail (token doesn't exist on owner)
	err := tokenOwner.appendTransaction(tx)
	assert.Error(t, err, "token owner should reject authorize if token doesn't exist")
}

func TestAuthorizeTokenTransferRollbackOnOwnerFailure(t *testing.T) {
	authorizer, _ := testCreateAccount(t)
	tokenOwner, _ := testCreateAccount(t)

	// Give authorizer balance
	require.NoError(t, authorizer.appendTransaction(newMintTransaction(authorizer, MBR_SLOT_COST+1)))
	authorizerBalanceBefore := authorizer.Balance()
	authorizerMBRBefore := authorizer.mbr

	nonExistentID := "nonexistent-token-id"
	tx := newAuthorizeTokenTransferTransaction(authorizer, tokenOwner, &nonExistentID, authorizer.GetNonce())

	// Authorizer side succeeds
	require.NoError(t, authorizer.appendTransaction(tx))
	assert.Equal(t, authorizerMBRBefore+MBR_SLOT_COST, authorizer.mbr, "MBR frozen after authorizer side")

	// Token owner side fails
	require.Error(t, tokenOwner.appendTransaction(tx))

	// Rollback authorizer
	require.NoError(t, authorizer.rollbacklatestTransaction(tx))

	assert.Equal(t, authorizerMBRBefore, authorizer.mbr, "MBR restored after rollback")
	assert.Equal(t, authorizerBalanceBefore, authorizer.Balance(), "balance restored after rollback")
	assert.Nil(t, authorizer.tokenStore[nonExistentID], "placeholder slot removed after rollback")
}

// --- BurnToken handler tests ---

func TestBurnTokenRemovesTokenAndUnfreezesMBR(t *testing.T) {
	acc, kp := testCreateAccount(t)
	tok := testMintTokenIntoAccount(t, acc, kp.PrivateKey)

	mbrBefore := acc.mbr

	burnTx := newBurnTokenTransaction(acc, tok.ID(), acc.GetNonce())
	err := acc.appendTransaction(burnTx)
	require.NoError(t, err)

	_, ok := acc.GetToken(tok.ID())
	assert.False(t, ok, "token should be removed after burn")
	assert.Equal(t, mbrBefore-MBR_TOKEN_COST, acc.mbr, "MBR should decrease by MBR_TOKEN_COST after burn")
}

func TestBurnTokenFailsForNonExistentToken(t *testing.T) {
	acc, _ := testCreateAccount(t)

	burnTx := newBurnTokenTransaction(acc, "nonexistent-token-id", acc.GetNonce())
	err := acc.appendTransaction(burnTx)
	assert.Error(t, err, "burn should fail for non-existent token")
}

// --- UnauthorizeTokenTransfer handler tests ---

func TestUnauthorizeTokenTransferRemovesSlotAndUnfreezesMBR(t *testing.T) {
	authorizer, _ := testCreateAccount(t)
	tokenOwner, kp := testCreateAccount(t)
	minted := testMintTokenIntoAccount(t, tokenOwner, kp.PrivateKey)
	testAuthorizeTokenTransfer(t, authorizer, tokenOwner, minted.ID())

	mbrAfterAuth := authorizer.mbr

	tokenId := minted.ID()
	tx := newUnauthorizeTokenTransferTransaction(authorizer, tokenOwner, &tokenId, authorizer.GetNonce())
	require.NoError(t, authorizer.appendTransaction(tx))
	require.NoError(t, tokenOwner.appendTransaction(tx))

	assert.Nil(t, authorizer.tokenStore[minted.ID()], "placeholder slot should be removed after unauthorize")
	assert.Equal(t, mbrAfterAuth-MBR_SLOT_COST, authorizer.mbr, "MBR should decrease by MBR_SLOT_COST after unauthorize")
}

func TestRollbackUnauthorizeTokenTransferRestoresState(t *testing.T) {
	authorizer, _ := testCreateAccount(t)
	tokenOwner, kp := testCreateAccount(t)
	minted := testMintTokenIntoAccount(t, tokenOwner, kp.PrivateKey)
	testAuthorizeTokenTransfer(t, authorizer, tokenOwner, minted.ID())

	mbrAfterAuth := authorizer.mbr

	tokenId := minted.ID()
	tx := newUnauthorizeTokenTransferTransaction(authorizer, tokenOwner, &tokenId, authorizer.GetNonce())
	require.NoError(t, authorizer.appendTransaction(tx))
	require.NoError(t, tokenOwner.appendTransaction(tx))

	require.NoError(t, authorizer.rollbacklatestTransaction(tx))

	assert.NotNil(t, authorizer.tokenStore[minted.ID()], "placeholder slot should be restored after rollback")
	assert.Equal(t, mbrAfterAuth, authorizer.mbr, "MBR should be restored to post-authorize value after rollback")
}

// --- ClawbackToken handler tests ---

func TestClawbackTokenMovesTokenToAuthority(t *testing.T) {
	// Note: clawback authority validation is at the app layer; account handler uses addToken (no slot required).
	authority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)
	tok := testMintTokenIntoAccount(t, holder, kp.PrivateKey)

	// from=holder (sender, loses token), to=authority (receiver, gains token)
	clawbackTx := newClawbackTokenTransaction(holder, authority, *tok, authority.GetNonce())
	require.NoError(t, holder.appendTransaction(clawbackTx))
	require.NoError(t, authority.appendTransaction(clawbackTx))

	_, holderHasToken := holder.GetToken(tok.ID())
	assert.False(t, holderHasToken, "holder should not have token after clawback")

	_, authorityHasToken := authority.GetToken(tok.ID())
	assert.True(t, authorityHasToken, "authority should have token after clawback")
}

func TestClawbackTokenRollback(t *testing.T) {
	authority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)
	tok := testMintTokenIntoAccount(t, holder, kp.PrivateKey)

	clawbackTx := newClawbackTokenTransaction(holder, authority, *tok, authority.GetNonce())
	require.NoError(t, holder.appendTransaction(clawbackTx))
	require.NoError(t, authority.appendTransaction(clawbackTx))

	// Rollback both sides (authority first, then holder, matching two-party rollback pattern)
	require.NoError(t, authority.rollbacklatestTransaction(clawbackTx))
	require.NoError(t, holder.rollbacklatestTransaction(clawbackTx))

	_, holderHasToken := holder.GetToken(tok.ID())
	assert.True(t, holderHasToken, "holder should have token restored after rollback")

	_, authorityHasToken := authority.GetToken(tok.ID())
	assert.False(t, authorityHasToken, "authority should not have token after rollback")
}

// --- FreezeToken handler tests ---

func TestFreezeTokenSetsFrozenFlag(t *testing.T) {
	authority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)

	authorityID := authority.ID()
	tok := testCreateTokenWithAddresses(t, holder, kp.PrivateKey, nil, &authorityID)
	require.NoError(t, holder.appendTransaction(newMintTransaction(holder, MBR_TOKEN_COST)))
	require.NoError(t, holder.appendTransaction(newMintTokenTransaction(holder, tok)))

	// Give authority enough balance for MBR_FREEZE_COST
	require.NoError(t, authority.appendTransaction(newMintTransaction(authority, MBR_FREEZE_COST)))
	mbrBefore := authority.mbr

	freezeTx := newFreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	require.NoError(t, authority.appendTransaction(freezeTx))
	require.NoError(t, holder.appendTransaction(freezeTx))

	assert.True(t, holder.tokenStore[tok.ID()].frozen, "token should be frozen after freeze transaction")
	assert.Equal(t, mbrBefore+MBR_FREEZE_COST, authority.mbr, "authority MBR should increase by MBR_FREEZE_COST")
}

func TestFreezeTokenFailsWithoutFreezeAddress(t *testing.T) {
	authority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)

	// Mint token with no freeze address
	tok := testMintTokenIntoAccount(t, holder, kp.PrivateKey)
	require.NoError(t, authority.appendTransaction(newMintTransaction(authority, MBR_FREEZE_COST)))

	freezeTx := newFreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	require.NoError(t, authority.appendTransaction(freezeTx), "authority side succeeds (just locks MBR)")
	err := holder.appendTransaction(freezeTx)
	assert.Error(t, err, "holder should reject freeze when token has no freeze address")
}

func TestFreezeTokenFailsForWrongAuthority(t *testing.T) {
	wrongAuthority, _ := testCreateAccount(t)
	realAuthority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)

	realAuthorityID := realAuthority.ID()
	tok := testCreateTokenWithAddresses(t, holder, kp.PrivateKey, nil, &realAuthorityID)
	require.NoError(t, holder.appendTransaction(newMintTransaction(holder, MBR_TOKEN_COST)))
	require.NoError(t, holder.appendTransaction(newMintTokenTransaction(holder, tok)))

	require.NoError(t, wrongAuthority.appendTransaction(newMintTransaction(wrongAuthority, MBR_FREEZE_COST)))

	freezeTx := newFreezeTokenTransaction(wrongAuthority, holder, tok.ID(), wrongAuthority.GetNonce())
	require.NoError(t, wrongAuthority.appendTransaction(freezeTx))
	err := holder.appendTransaction(freezeTx)
	assert.Error(t, err, "holder should reject freeze when wrong authority signs")
}

func TestFreezeTokenFailsInsufficientMBRBalance(t *testing.T) {
	authority, _ := testCreateAccount(t) // balance = 0
	holder, kp := testCreateAccount(t)

	authorityID := authority.ID()
	tok := testCreateTokenWithAddresses(t, holder, kp.PrivateKey, nil, &authorityID)
	require.NoError(t, holder.appendTransaction(newMintTransaction(holder, MBR_TOKEN_COST)))
	require.NoError(t, holder.appendTransaction(newMintTokenTransaction(holder, tok)))

	freezeTx := newFreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	err := authority.appendTransaction(freezeTx)
	assert.Error(t, err, "freeze should fail when authority has insufficient balance for MBR_FREEZE_COST")
}

func TestRollbackFreezeTokenRestoresState(t *testing.T) {
	authority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)

	authorityID := authority.ID()
	tok := testCreateTokenWithAddresses(t, holder, kp.PrivateKey, nil, &authorityID)
	require.NoError(t, holder.appendTransaction(newMintTransaction(holder, MBR_TOKEN_COST)))
	require.NoError(t, holder.appendTransaction(newMintTokenTransaction(holder, tok)))
	require.NoError(t, authority.appendTransaction(newMintTransaction(authority, MBR_FREEZE_COST)))

	mbrBefore := authority.mbr

	freezeTx := newFreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	require.NoError(t, authority.appendTransaction(freezeTx))
	require.NoError(t, holder.appendTransaction(freezeTx))

	require.NoError(t, holder.rollbacklatestTransaction(freezeTx))
	require.NoError(t, authority.rollbacklatestTransaction(freezeTx))

	assert.False(t, holder.tokenStore[tok.ID()].frozen, "token should be unfrozen after rollback")
	assert.Equal(t, mbrBefore, authority.mbr, "authority MBR should be restored after rollback")
}

// --- UnfreezeToken handler tests ---

func TestUnfreezeTokenClearsFrozenFlag(t *testing.T) {
	authority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)

	authorityID := authority.ID()
	tok := testCreateTokenWithAddresses(t, holder, kp.PrivateKey, nil, &authorityID)
	require.NoError(t, holder.appendTransaction(newMintTransaction(holder, MBR_TOKEN_COST)))
	require.NoError(t, holder.appendTransaction(newMintTokenTransaction(holder, tok)))
	require.NoError(t, authority.appendTransaction(newMintTransaction(authority, MBR_FREEZE_COST)))

	// Freeze first
	freezeTx := newFreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	require.NoError(t, authority.appendTransaction(freezeTx))
	require.NoError(t, holder.appendTransaction(freezeTx))
	require.True(t, holder.tokenStore[tok.ID()].frozen)

	mbrAfterFreeze := authority.mbr

	// Now unfreeze
	unfreezeTx := newUnfreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	require.NoError(t, authority.appendTransaction(unfreezeTx))
	require.NoError(t, holder.appendTransaction(unfreezeTx))

	assert.False(t, holder.tokenStore[tok.ID()].frozen, "token should be unfrozen after unfreeze transaction")
	assert.Equal(t, mbrAfterFreeze-MBR_UNFREEZE_COST, authority.mbr, "authority MBR should decrease by MBR_UNFREEZE_COST")
}

func TestRollbackUnfreezeTokenRestoresState(t *testing.T) {
	authority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)

	authorityID := authority.ID()
	tok := testCreateTokenWithAddresses(t, holder, kp.PrivateKey, nil, &authorityID)
	require.NoError(t, holder.appendTransaction(newMintTransaction(holder, MBR_TOKEN_COST)))
	require.NoError(t, holder.appendTransaction(newMintTokenTransaction(holder, tok)))
	require.NoError(t, authority.appendTransaction(newMintTransaction(authority, MBR_FREEZE_COST)))

	freezeTx := newFreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	require.NoError(t, authority.appendTransaction(freezeTx))
	require.NoError(t, holder.appendTransaction(freezeTx))

	mbrAfterFreeze := authority.mbr

	unfreezeTx := newUnfreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	require.NoError(t, authority.appendTransaction(unfreezeTx))
	require.NoError(t, holder.appendTransaction(unfreezeTx))

	// Rollback unfreeze
	require.NoError(t, holder.rollbacklatestTransaction(unfreezeTx))
	require.NoError(t, authority.rollbacklatestTransaction(unfreezeTx))

	assert.True(t, holder.tokenStore[tok.ID()].frozen, "token should be frozen again after rollback of unfreeze")
	assert.Equal(t, mbrAfterFreeze, authority.mbr, "authority MBR should be restored to post-freeze value")
}

// --- Transfer blocking tests ---

func TestTransferFrozenTokenFails(t *testing.T) {
	receiver, _ := testCreateAccount(t)
	authority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)

	authorityID := authority.ID()
	tok := testCreateTokenWithAddresses(t, holder, kp.PrivateKey, nil, &authorityID)
	require.NoError(t, holder.appendTransaction(newMintTransaction(holder, MBR_TOKEN_COST)))
	require.NoError(t, holder.appendTransaction(newMintTokenTransaction(holder, tok)))
	require.NoError(t, authority.appendTransaction(newMintTransaction(authority, MBR_FREEZE_COST)))

	freezeTx := newFreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	require.NoError(t, authority.appendTransaction(freezeTx))
	require.NoError(t, holder.appendTransaction(freezeTx))

	// Attempt transfer of frozen token
	transferTx := newTransferTokenTransaction(holder, receiver, tok, holder.GetNonce())
	err := holder.appendTransaction(transferTx)
	assert.Error(t, err, "transfer of frozen token should fail")
	assert.Contains(t, err.Error(), "frozen", "error should mention frozen")

	_, holderStillHasToken := holder.GetToken(tok.ID())
	assert.True(t, holderStillHasToken, "token should still be in holder's store after failed transfer")
}

func TestClawbackFrozenTokenSucceeds(t *testing.T) {
	authority, _ := testCreateAccount(t)
	holder, kp := testCreateAccount(t)

	authorityID := authority.ID()
	tok := testCreateTokenWithAddresses(t, holder, kp.PrivateKey, &authorityID, &authorityID)
	require.NoError(t, holder.appendTransaction(newMintTransaction(holder, MBR_TOKEN_COST)))
	require.NoError(t, holder.appendTransaction(newMintTokenTransaction(holder, tok)))
	require.NoError(t, authority.appendTransaction(newMintTransaction(authority, MBR_FREEZE_COST)))

	// Freeze the token
	freezeTx := newFreezeTokenTransaction(authority, holder, tok.ID(), authority.GetNonce())
	require.NoError(t, authority.appendTransaction(freezeTx))
	require.NoError(t, holder.appendTransaction(freezeTx))

	// Clawback should succeed even when frozen (no frozen check in handleClawbackTokenTx)
	clawbackTx := newClawbackTokenTransaction(holder, authority, *tok, authority.GetNonce())
	require.NoError(t, holder.appendTransaction(clawbackTx))
	require.NoError(t, authority.appendTransaction(clawbackTx))

	_, authorityHasToken := authority.GetToken(tok.ID())
	assert.True(t, authorityHasToken, "authority should have token after clawback of frozen token")
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
	transferTx := newTransferTokenTransaction(sender, receiver, tok, sender.GetNonce())

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
