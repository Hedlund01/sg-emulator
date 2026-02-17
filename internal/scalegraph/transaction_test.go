package scalegraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- MintTransaction Tests ---

func TestNewMintTransaction(t *testing.T) {
	receiver, _ := testCreateAccount(t)

	tx := newMintTransaction(receiver, 100.0)

	require.NotNil(t, tx)
	assert.Equal(t, 100.0, tx.Amount())
	assert.Nil(t, tx.Sender(), "mint transaction should have nil sender")
	assert.Equal(t, receiver, tx.Receiver())
}

func TestMintTransactionType(t *testing.T) {
	receiver, _ := testCreateAccount(t)

	tx := newMintTransaction(receiver, 50.0)

	require.NotNil(t, tx.Type())
	assert.Equal(t, Mint, tx.Type())
}

func TestMintTransactionID(t *testing.T) {
	receiver, _ := testCreateAccount(t)

	tx1 := newMintTransaction(receiver, 10.0)
	tx2 := newMintTransaction(receiver, 10.0)

	require.NotNil(t, tx1.ID())
	require.NotNil(t, tx2.ID())
	assert.NotEqual(t, tx1.ID(), tx2.ID(), "two mint transactions should have unique IDs")
	assert.NotEqual(t, ScalegraphId{}, tx1.ID(), "transaction ID should not be zero value")
}

func TestMintTransactionImmutability(t *testing.T) {
	receiver, _ := testCreateAccount(t)

	tx := newMintTransaction(receiver, 75.0)
	originalID := tx.ID()
	originalAmount := tx.Amount()

	for i := 0; i < 10; i++ {
		assert.Equal(t, originalID, tx.ID(), "transaction ID changed on access %d", i)
		assert.Equal(t, originalAmount, tx.Amount(), "transaction amount changed on access %d", i)
		assert.Nil(t, tx.Sender())
		assert.Equal(t, receiver, tx.Receiver())
	}
}

// --- TransferTransaction Tests ---

func TestNewTransferTransaction(t *testing.T) {
	sender, receiver := testCreateTwoAccounts(t)

	tx := newTransferTransaction(sender, receiver, 50.0)

	require.NotNil(t, tx)
	assert.Equal(t, 50.0, tx.Amount())
	assert.Equal(t, sender, tx.Sender())
	assert.Equal(t, receiver, tx.Receiver())
}

func TestTransferTransactionType(t *testing.T) {
	sender, receiver := testCreateTwoAccounts(t)

	tx := newTransferTransaction(sender, receiver, 25.0)

	require.NotNil(t, tx.Type())
	assert.Equal(t, Transfer, tx.Type())
}

func TestTransferTransactionID(t *testing.T) {
	sender, receiver := testCreateTwoAccounts(t)

	tx1 := newTransferTransaction(sender, receiver, 10.0)
	tx2 := newTransferTransaction(sender, receiver, 10.0)

	require.NotNil(t, tx1.ID())
	require.NotNil(t, tx2.ID())
	assert.NotEqual(t, tx1.ID(), tx2.ID(), "two transfer transactions should have unique IDs")
	assert.NotEqual(t, ScalegraphId{}, tx1.ID(), "transaction ID should not be zero value")
}

func TestTransferTransactionImmutability(t *testing.T) {
	sender, receiver := testCreateTwoAccounts(t)

	tx := newTransferTransaction(sender, receiver, 50.0)
	originalID := tx.ID()
	originalAmount := tx.Amount()

	for i := 0; i < 10; i++ {
		assert.Equal(t, originalID, tx.ID(), "transaction ID changed on access %d", i)
		assert.Equal(t, originalAmount, tx.Amount(), "transaction amount changed on access %d", i)
		assert.Equal(t, sender, tx.Sender())
		assert.Equal(t, receiver, tx.Receiver())
	}
}

// --- BurnTransaction Tests ---

func TestNewBurnTransaction(t *testing.T) {
	receiver, _ := testCreateAccount(t)

	tx := newBurnTransaction(receiver, 30.0)

	require.NotNil(t, tx)
	assert.Equal(t, 30.0, tx.Amount())
	assert.Nil(t, tx.Sender(), "burn transaction should have nil sender")
	assert.Equal(t, receiver, tx.Receiver())
}

func TestBurnTransactionType(t *testing.T) {
	receiver, _ := testCreateAccount(t)

	tx := newBurnTransaction(receiver, 15.0)

	require.NotNil(t, tx.Type())
	assert.Equal(t, Burn, tx.Type())
}

func TestBurnTransactionID(t *testing.T) {
	receiver, _ := testCreateAccount(t)

	tx1 := newBurnTransaction(receiver, 10.0)
	tx2 := newBurnTransaction(receiver, 10.0)

	require.NotNil(t, tx1.ID())
	require.NotNil(t, tx2.ID())
	assert.NotEqual(t, tx1.ID(), tx2.ID(), "two burn transactions should have unique IDs")
	assert.NotEqual(t, ScalegraphId{}, tx1.ID(), "transaction ID should not be zero value")
}

// --- ITransaction Interface Compliance Tests ---

func TestTransactionsImplementITransaction(t *testing.T) {
	sender, receiver := testCreateTwoAccounts(t)

	tests := []struct {
		name string
		tx   ITransaction
	}{
		{"MintTransaction", newMintTransaction(receiver, 10.0)},
		{"TransferTransaction", newTransferTransaction(sender, receiver, 20.0)},
		{"BurnTransaction", newBurnTransaction(receiver, 5.0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.tx.ID(), "%s should have non-nil ID", tt.name)
			assert.NotNil(t, tt.tx.Type(), "%s should have non-nil Type", tt.name)
			// Receiver is always set
			assert.NotNil(t, tt.tx.Receiver(), "%s should have non-nil Receiver", tt.name)
		})
	}
}

// --- Transaction Amount Extraction Helper Tests ---

func TestGetTransactionAmount(t *testing.T) {
	sender, receiver := testCreateTwoAccounts(t)

	amounts := []float64{0, 1.0, 50.5, 100.0, 999.99}
	for _, amount := range amounts {
		mintTx := newMintTransaction(receiver, amount)
		assert.Equal(t, amount, getTransactionAmount(mintTx), "mint amount mismatch for %.2f", amount)

		transferTx := newTransferTransaction(sender, receiver, amount)
		assert.Equal(t, amount, getTransactionAmount(transferTx), "transfer amount mismatch for %.2f", amount)

		burnTx := newBurnTransaction(receiver, amount)
		assert.Equal(t, amount, getTransactionAmount(burnTx), "burn amount mismatch for %.2f", amount)
	}
}

// --- TransactionType Tests ---

func TestTransactionTypeString(t *testing.T) {
	tests := []struct {
		tt       TransactionType
		expected string
	}{
		{Transfer, "Transfer"},
		{Mint, "Mint"},
		{Burn, "Burn"},
		{MintToken, "MintToken"},
		{BurnToken, "BurnToken"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.tt.String())
		})
	}
}

func TestTransactionTypeEnumIndex(t *testing.T) {
	assert.Equal(t, 0, Transfer.EnumIndex())
	assert.Equal(t, 1, Mint.EnumIndex())
	assert.Equal(t, 2, Burn.EnumIndex())
}
