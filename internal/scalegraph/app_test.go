package scalegraph

import "context"

import (
	"log/slog"
	"testing"
)

func testLogger() *slog.Logger {
	// Create a no-op logger for tests
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func TestNew(t *testing.T) {
	app := New(testLogger())
	if app == nil {
		t.Fatal("New() returned nil")
	}
	if app.accounts == nil {
		t.Error("accounts map not initialized")
	}
	if len(app.accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(app.accounts))
	}
}

func TestCreateAccount(t *testing.T) {
	app := New(testLogger())

	// Test creating account with zero balance
	acc1, err := app.CreateAccount(context.Background(), 0)
	if err != nil {
		t.Fatalf("CreateAccount(0) failed: %v", err)
	}
	if acc1 == nil {
		t.Fatal("CreateAccount(0) returned nil account")
	}
	if acc1.Balance() != 0 {
		t.Errorf("expected balance 0, got %.2f", acc1.Balance())
	}

	// Test creating account with initial balance
	acc2, err := app.CreateAccount(context.Background(), 100.0)
	if err != nil {
		t.Fatalf("CreateAccount(100) failed: %v", err)
	}
	if acc2.Balance() != 100.0 {
		t.Errorf("expected balance 100.0, got %.2f", acc2.Balance())
	}

	// Test that accounts are stored
	if app.AccountCount(context.Background()) != 2 {
		t.Errorf("expected 2 accounts, got %d", app.AccountCount(context.Background()))
	}

	// Test that accounts have unique IDs
	if acc1.ID() == acc2.ID() {
		t.Error("accounts have identical IDs")
	}
}

func TestGetAccounts(t *testing.T) {
	app := New(testLogger())

	// Test empty app
	accounts := app.GetAccounts(context.Background())
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(accounts))
	}

	// Create some accounts
	acc1, _ := app.CreateAccount(context.Background(), 50.0)
	acc2, _ := app.CreateAccount(context.Background(), 100.0)
	acc3, _ := app.CreateAccount(context.Background(), 150.0)

	accounts = app.GetAccounts(context.Background())
	if len(accounts) != 3 {
		t.Errorf("expected 3 accounts, got %d", len(accounts))
	}

	// Verify all accounts are present (order doesn't matter)
	ids := make(map[ScalegraphId]bool)
	ids[acc1.ID()] = true
	ids[acc2.ID()] = true
	ids[acc3.ID()] = true

	for _, acc := range accounts {
		if !ids[acc.ID()] {
			t.Errorf("unexpected account ID: %s", acc.ID())
		}
	}
}

func TestGetAccount(t *testing.T) {
	app := New(testLogger())
	acc, _ := app.CreateAccount(context.Background(), 100.0)

	// Test getting existing account
	retrieved, err := app.GetAccount(context.Background(), acc.ID())
	if err != nil {
		t.Fatalf("GetAccount() failed: %v", err)
	}
	if retrieved.ID() != acc.ID() {
		t.Error("retrieved account has different ID")
	}
	if retrieved.Balance() != 100.0 {
		t.Errorf("expected balance 100.0, got %.2f", retrieved.Balance())
	}

	// Test getting non-existent account
	fakeID, _ := NewScalegraphId()
	_, err = app.GetAccount(context.Background(), fakeID)
	if err == nil {
		t.Error("expected error for non-existent account, got nil")
	}
}

func TestAccountCount(t *testing.T) {
	app := New(testLogger())

	if app.AccountCount(context.Background()) != 0 {
		t.Errorf("expected count 0, got %d", app.AccountCount(context.Background()))
	}

	app.CreateAccount(context.Background(), 10.0)
	if app.AccountCount(context.Background()) != 1 {
		t.Errorf("expected count 1, got %d", app.AccountCount(context.Background()))
	}

	app.CreateAccount(context.Background(), 20.0)
	app.CreateAccount(context.Background(), 30.0)
	if app.AccountCount(context.Background()) != 3 {
		t.Errorf("expected count 3, got %d", app.AccountCount(context.Background()))
	}
}

func TestTransfer(t *testing.T) {
	app := New(testLogger())
	acc1, _ := app.CreateAccount(context.Background(), 100.0)
	acc2, _ := app.CreateAccount(context.Background(), 50.0)

	// Test successful transfer
	err := app.Transfer(context.Background(), acc1.ID(), acc2.ID(), 30.0)
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	if acc1.Balance() != 70.0 {
		t.Errorf("expected sender balance 70.0, got %.2f", acc1.Balance())
	}
	if acc2.Balance() != 80.0 {
		t.Errorf("expected receiver balance 80.0, got %.2f", acc2.Balance())
	}

	// Test transfer with insufficient funds
	err = app.Transfer(context.Background(), acc1.ID(), acc2.ID(), 100.0)
	if err == nil {
		t.Error("expected error for insufficient funds, got nil")
	}

	// Verify balances unchanged after failed transfer
	if acc1.Balance() != 70.0 {
		t.Errorf("sender balance changed after failed transfer: %.2f", acc1.Balance())
	}
	if acc2.Balance() != 80.0 {
		t.Errorf("receiver balance changed after failed transfer: %.2f", acc2.Balance())
	}

	// Test transfer from non-existent account
	fakeID, _ := NewScalegraphId()
	err = app.Transfer(context.Background(), fakeID, acc2.ID(), 10.0)
	if err == nil {
		t.Error("expected error for non-existent sender, got nil")
	}

	// Test transfer to non-existent account
	err = app.Transfer(context.Background(), acc1.ID(), fakeID, 10.0)
	if err == nil {
		t.Error("expected error for non-existent receiver, got nil")
	}
}

func TestTransferZeroAmount(t *testing.T) {
	app := New(testLogger())
	acc1, _ := app.CreateAccount(context.Background(), 100.0)
	acc2, _ := app.CreateAccount(context.Background(), 50.0)

	// Transfer 0 should succeed but not change balances
	err := app.Transfer(context.Background(), acc1.ID(), acc2.ID(), 0)
	if err != nil {
		t.Fatalf("Transfer(0) failed: %v", err)
	}

	if acc1.Balance() != 100.0 {
		t.Errorf("expected sender balance 100.0, got %.2f", acc1.Balance())
	}
	if acc2.Balance() != 50.0 {
		t.Errorf("expected receiver balance 50.0, got %.2f", acc2.Balance())
	}
}

func TestMint(t *testing.T) {
	app := New(testLogger())
	acc, _ := app.CreateAccount(context.Background(), 100.0)

	// Test minting funds
	err := app.Mint(context.Background(), acc.ID(), 50.0)
	if err != nil {
		t.Fatalf("Mint failed: %v", err)
	}

	if acc.Balance() != 150.0 {
		t.Errorf("expected balance 150.0, got %.2f", acc.Balance())
	}

	// Test minting to non-existent account
	fakeID, _ := NewScalegraphId()
	err = app.Mint(context.Background(), fakeID, 10.0)
	if err == nil {
		t.Error("expected error for non-existent account, got nil")
	}
}

func TestTransferAtomicity(t *testing.T) {
	app := New(testLogger())
	acc1, _ := app.CreateAccount(context.Background(), 100.0)
	acc2, _ := app.CreateAccount(context.Background(), 50.0)

	initialTotal := acc1.Balance() + acc2.Balance()

	// Successful transfer should preserve total balance
	app.Transfer(context.Background(), acc1.ID(), acc2.ID(), 25.0)
	finalTotal := acc1.Balance() + acc2.Balance()

	if initialTotal != finalTotal {
		t.Errorf("total balance changed: %.2f -> %.2f", initialTotal, finalTotal)
	}

	// Failed transfer should also preserve balances
	beforeAcc1 := acc1.Balance()
	beforeAcc2 := acc2.Balance()

	app.Transfer(context.Background(), acc1.ID(), acc2.ID(), 1000.0) // Should fail

	if acc1.Balance() != beforeAcc1 || acc2.Balance() != beforeAcc2 {
		t.Error("balances changed after failed transfer")
	}
}

func TestConcurrentAccountCreation(t *testing.T) {
	app := New(testLogger())
	done := make(chan bool)

	// Create 100 accounts concurrently
	for i := 0; i < 100; i++ {
		go func(balance float64) {
			_, err := app.CreateAccount(context.Background(), balance)
			if err != nil {
				t.Errorf("concurrent CreateAccount failed: %v", err)
			}
			done <- true
		}(float64(i))
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	if app.AccountCount(context.Background()) != 100 {
		t.Errorf("expected 100 accounts, got %d", app.AccountCount(context.Background()))
	}
}
