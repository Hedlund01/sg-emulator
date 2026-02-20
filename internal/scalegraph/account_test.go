package scalegraph

import (
	"testing"

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
	acc := createTestAccountInApp(t, app, 0)

	// Nonce is based on blockchain length
	initialNonce := acc.GetNonce()

	err := app.Mint(testCtx(), &MintRequest{To: acc.ID(), Amount: 50.0})
	require.NoError(t, err)

	assert.Greater(t, acc.GetNonce(), initialNonce, "nonce should increase after transaction")
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
