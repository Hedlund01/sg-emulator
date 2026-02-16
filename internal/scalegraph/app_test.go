package scalegraph

import (
	"testing"

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
	assert.Equal(t, 2, app.AccountCount(testCtx()))

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
	accounts := app.GetAccounts(testCtx())
	assert.Empty(t, accounts)

	// Create some accounts
	acc1 := createTestAccountInApp(t, app, 50.0)
	acc2 := createTestAccountInApp(t, app, 100.0)
	acc3 := createTestAccountInApp(t, app, 150.0)

	accounts = app.GetAccounts(testCtx())
	assert.Len(t, accounts, 3)

	// Verify all accounts are present (order doesn't matter)
	ids := map[ScalegraphId]bool{
		acc1.ID(): true,
		acc2.ID(): true,
		acc3.ID(): true,
	}

	for _, acc := range accounts {
		assert.True(t, ids[acc.ID()], "unexpected account ID: %s", acc.ID())
	}
}

func TestGetAccount(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 100.0)

	// Test getting existing account
	retrieved, err := app.GetAccount(testCtx(), acc.ID())
	require.NoError(t, err)
	assert.Equal(t, acc.ID(), retrieved.ID())
	assert.Equal(t, 100.0, retrieved.Balance())

	// Test getting non-existent account
	fakeID, _ := NewScalegraphId()
	_, err = app.GetAccount(testCtx(), fakeID)
	assert.Error(t, err, "should error for non-existent account")
}

func TestAccountCount(t *testing.T) {
	app := testApp()

	assert.Equal(t, 0, app.AccountCount(testCtx()))

	createTestAccountInApp(t, app, 10.0)
	assert.Equal(t, 1, app.AccountCount(testCtx()))

	createTestAccountInApp(t, app, 20.0)
	createTestAccountInApp(t, app, 30.0)
	assert.Equal(t, 3, app.AccountCount(testCtx()))
}

func TestTransfer(t *testing.T) {
	app := testApp()
	acc1 := createTestAccountInApp(t, app, 100.0)
	acc2 := createTestAccountInApp(t, app, 50.0)

	// Nonce = blockchain length + 1
	// After CreateAccountWithKeys with balance, acc has genesis + mint = 2 blocks, so nonce is 2, next is 3
	nonce := acc1.GetNonce() + 1

	// Test successful transfer
	err := app.Transfer(testCtx(), acc1.ID(), acc2.ID(), 30.0, nonce)
	require.NoError(t, err)

	assert.Equal(t, 70.0, acc1.Balance(), "sender balance after transfer")
	assert.Equal(t, 80.0, acc2.Balance(), "receiver balance after transfer")

	// Test transfer with insufficient funds
	nonce = acc1.GetNonce() + 1
	err = app.Transfer(testCtx(), acc1.ID(), acc2.ID(), 100.0, nonce)
	assert.Error(t, err, "should error for insufficient funds")

	// Verify balances unchanged after failed transfer
	assert.Equal(t, 70.0, acc1.Balance(), "sender balance should not change after failed transfer")
	assert.Equal(t, 80.0, acc2.Balance(), "receiver balance should not change after failed transfer")

	// Test transfer from non-existent account
	fakeID, _ := NewScalegraphId()
	err = app.Transfer(testCtx(), fakeID, acc2.ID(), 10.0, 1)
	assert.Error(t, err, "should error for non-existent sender")

	// Test transfer to non-existent account
	nonce = acc1.GetNonce() + 1
	err = app.Transfer(testCtx(), acc1.ID(), fakeID, 10.0, nonce)
	assert.Error(t, err, "should error for non-existent receiver")
}

func TestTransferSelfTransfer(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 100.0)

	nonce := acc.GetNonce() + 1
	err := app.Transfer(testCtx(), acc.ID(), acc.ID(), 10.0, nonce)
	assert.Error(t, err, "self-transfer should not be allowed")
	assert.Equal(t, 100.0, acc.Balance(), "balance should not change after rejected self-transfer")
}

func TestTransferZeroAmount(t *testing.T) {
	app := testApp()
	acc1 := createTestAccountInApp(t, app, 100.0)
	acc2 := createTestAccountInApp(t, app, 50.0)

	nonce := acc1.GetNonce() + 1
	err := app.Transfer(testCtx(), acc1.ID(), acc2.ID(), 0, nonce)
	require.NoError(t, err)

	assert.Equal(t, 100.0, acc1.Balance(), "sender balance should not change for zero transfer")
	assert.Equal(t, 50.0, acc2.Balance(), "receiver balance should not change for zero transfer")
}

func TestTransferNonceMismatch(t *testing.T) {
	app := testApp()
	acc1 := createTestAccountInApp(t, app, 100.0)
	acc2 := createTestAccountInApp(t, app, 50.0)

	// Use wrong nonce
	err := app.Transfer(testCtx(), acc1.ID(), acc2.ID(), 10.0, 999)
	assert.Error(t, err, "should error for nonce mismatch")
	assert.Equal(t, 100.0, acc1.Balance(), "balance should not change after nonce mismatch")
}

func TestMint(t *testing.T) {
	app := testApp()
	acc := createTestAccountInApp(t, app, 100.0)

	// Test minting funds
	err := app.Mint(testCtx(), acc.ID(), 50.0)
	require.NoError(t, err)
	assert.Equal(t, 150.0, acc.Balance())

	// Test minting to non-existent account
	fakeID, _ := NewScalegraphId()
	err = app.Mint(testCtx(), fakeID, 10.0)
	assert.Error(t, err, "should error for non-existent account")
}

func TestTransferAtomicity(t *testing.T) {
	app := testApp()
	acc1 := createTestAccountInApp(t, app, 100.0)
	acc2 := createTestAccountInApp(t, app, 50.0)

	initialTotal := acc1.Balance() + acc2.Balance()

	// Successful transfer should preserve total balance
	nonce := acc1.GetNonce() + 1
	err := app.Transfer(testCtx(), acc1.ID(), acc2.ID(), 25.0, nonce)
	require.NoError(t, err)

	finalTotal := acc1.Balance() + acc2.Balance()
	assert.Equal(t, initialTotal, finalTotal, "total balance should be preserved after transfer")

	// Failed transfer should also preserve balances
	beforeAcc1 := acc1.Balance()
	beforeAcc2 := acc2.Balance()

	nonce = acc1.GetNonce() + 1
	err = app.Transfer(testCtx(), acc1.ID(), acc2.ID(), 1000.0, nonce)
	assert.Error(t, err, "transfer should fail for insufficient funds")

	assert.Equal(t, beforeAcc1, acc1.Balance(), "sender balance should not change after failed transfer")
	assert.Equal(t, beforeAcc2, acc2.Balance(), "receiver balance should not change after failed transfer")
}

func TestConcurrentAccountCreation(t *testing.T) {
	app := testApp()

	runConcurrent(t, 100, func(i int) {
		createTestAccountInApp(t, app, float64(i))
	})

	assert.Equal(t, 100, app.AccountCount(testCtx()))
}
