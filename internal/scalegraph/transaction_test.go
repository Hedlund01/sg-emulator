package scalegraph

import (
	"testing"
)

func TestNewTransaction(t *testing.T) {
	sender, _ := newAccount()
	receiver, _ := newAccount()

	// Test creating transaction with amount
	tx, err := newTransaction(sender, receiver, 50.0, "")
	if err != nil {
		t.Fatalf("newTransaction() failed: %v", err)
	}
	if tx == nil {
		t.Fatal("newTransaction() returned nil")
	}

	if tx.Amount() != 50.0 {
		t.Errorf("expected amount 50.0, got %.2f", tx.Amount())
	}
	if tx.Sender() != sender {
		t.Error("sender mismatch")
	}
	if tx.Receiver() != receiver {
		t.Error("receiver mismatch")
	}
	if tx.Value() != "" {
		t.Errorf("expected empty value, got %s", tx.Value())
	}
}

func TestNewTransactionWithValue(t *testing.T) {
	receiver, _ := newAccount()

	// Test creating transaction with value
	tx, err := newTransaction(nil, receiver, 0, "some-value")
	if err != nil {
		t.Fatalf("newTransaction() with value failed: %v", err)
	}

	if tx.Value() != "some-value" {
		t.Errorf("expected value 'some-value', got %s", tx.Value())
	}
	if tx.Amount() != 0 {
		t.Errorf("expected amount 0, got %.2f", tx.Amount())
	}
	if tx.Sender() != nil {
		t.Error("expected nil sender")
	}
}

func TestNewTransactionAmountAndValue(t *testing.T) {
	sender, _ := newAccount()
	receiver, _ := newAccount()

	// Test that transaction cannot have both amount and value
	_, err := newTransaction(sender, receiver, 50.0, "value")
	if err == nil {
		t.Error("expected error for transaction with both amount and value, got nil")
	}
}

func TestTransactionID(t *testing.T) {
	sender, _ := newAccount()
	receiver, _ := newAccount()

	tx1, _ := newTransaction(sender, receiver, 10.0, "")
	tx2, _ := newTransaction(sender, receiver, 10.0, "")

	// Test that transactions have unique IDs
	if tx1.ID() == tx2.ID() {
		t.Error("two transactions have the same ID")
	}

	// Test that ID is not zero value
	if tx1.ID() == (ScalegraphId{}) {
		t.Error("transaction ID is zero value")
	}

	// Test that ID is consistent
	id := tx1.ID()
	if tx1.ID() != id {
		t.Error("transaction ID changed between calls")
	}
}

func TestTransactionSender(t *testing.T) {
	sender, _ := newAccount()
	receiver, _ := newAccount()

	tx, _ := newTransaction(sender, receiver, 50.0, "")
	if tx.Sender() != sender {
		t.Error("Sender() returned wrong account")
	}

	// Test mint transaction (nil sender)
	mintTx, _ := newTransaction(nil, receiver, 100.0, "")
	if mintTx.Sender() != nil {
		t.Error("mint transaction should have nil sender")
	}
}

func TestTransactionReceiver(t *testing.T) {
	sender, _ := newAccount()
	receiver, _ := newAccount()

	tx, _ := newTransaction(sender, receiver, 50.0, "")
	if tx.Receiver() != receiver {
		t.Error("Receiver() returned wrong account")
	}
}

func TestTransactionAmount(t *testing.T) {
	sender, _ := newAccount()
	receiver, _ := newAccount()

	amounts := []float64{0, 1.0, 50.5, 100.0, 999.99}
	for _, amount := range amounts {
		tx, _ := newTransaction(sender, receiver, amount, "")
		if tx.Amount() != amount {
			t.Errorf("expected amount %.2f, got %.2f", amount, tx.Amount())
		}
	}
}

func TestTransactionValue(t *testing.T) {
	receiver, _ := newAccount()

	values := []string{"", "test", "some-data", "{}"}
	for _, value := range values {
		tx, _ := newTransaction(nil, receiver, 0, value)
		if tx.Value() != value {
			t.Errorf("expected value %s, got %s", value, tx.Value())
		}
	}
}

func TestTransactionString(t *testing.T) {
	sender, _ := newAccount()
	receiver, _ := newAccount()

	tx, _ := newTransaction(sender, receiver, 50.0, "")
	str := tx.String()

	if str == "" {
		t.Error("String() returned empty string")
	}

	// Test mint transaction string
	mintTx, _ := newTransaction(nil, receiver, 100.0, "")
	mintStr := mintTx.String()
	if mintStr == "" {
		t.Error("String() for mint transaction returned empty string")
	}
}

func TestTransactionImmutability(t *testing.T) {
	sender, _ := newAccount()
	receiver, _ := newAccount()

	tx, _ := newTransaction(sender, receiver, 50.0, "")

	// Store original values
	originalID := tx.ID()
	originalAmount := tx.Amount()
	originalSender := tx.Sender()
	originalReceiver := tx.Receiver()

	// Access transaction multiple times
	for i := 0; i < 10; i++ {
		if tx.ID() != originalID {
			t.Error("transaction ID changed")
		}
		if tx.Amount() != originalAmount {
			t.Error("transaction amount changed")
		}
		if tx.Sender() != originalSender {
			t.Error("transaction sender changed")
		}
		if tx.Receiver() != originalReceiver {
			t.Error("transaction receiver changed")
		}
	}
}
