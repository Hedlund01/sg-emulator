package scalegraph

import (
	"testing"
)

func TestNewAccount_Integration(t *testing.T) {
	acc, err := newAccount()
	if err != nil {
		t.Fatalf("newAccount() failed: %v", err)
	}
	if acc == nil {
		t.Fatal("newAccount() returned nil")
	}

	// Test initial state
	if acc.Balance() != 0 {
		t.Errorf("expected initial balance 0, got %.2f", acc.Balance())
	}
	if acc.Blockchain() == nil {
		t.Error("blockchain not initialized")
	}

	// Test that ID is set
	if acc.ID() == (ScalegraphId{}) {
		t.Error("account ID is zero value")
	}
}

func TestAccountID_Integration(t *testing.T) {
	acc1, _ := newAccount()
	acc2, _ := newAccount()

	// Test that IDs are unique
	if acc1.ID() == acc2.ID() {
		t.Error("two accounts have the same ID")
	}

	// Test that ID is consistent
	id := acc1.ID()
	if acc1.ID() != id {
		t.Error("account ID changed between calls")
	}
}

func TestAccountBalance_Integration(t *testing.T) {
	acc, _ := newAccount()

	// Test initial balance
	if acc.Balance() != 0 {
		t.Errorf("expected balance 0, got %.2f", acc.Balance())
	}

	// Test balance after minting
	acc.mint(100.0)
	if acc.Balance() != 100.0 {
		t.Errorf("expected balance 100.0, got %.2f", acc.Balance())
	}

	// Test balance is thread-safe (multiple reads)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = acc.Balance()
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestAccountMint_Integration(t *testing.T) {
	acc, _ := newAccount()

	// Test minting positive amount
	err := acc.mint(50.0)
	if err != nil {
		t.Fatalf("mint(50.0) failed: %v", err)
	}
	if acc.Balance() != 50.0 {
		t.Errorf("expected balance 50.0, got %.2f", acc.Balance())
	}

	// Test multiple mints
	acc.mint(30.0)
	acc.mint(20.0)
	if acc.Balance() != 100.0 {
		t.Errorf("expected balance 100.0, got %.2f", acc.Balance())
	}

	// Test that mint creates transaction
	blockchain := acc.Blockchain()
	blocks := blockchain.GetBlocks()
	if len(blocks) == 0 {
		t.Error("no blocks created after mint")
	}
}

func TestAccountMintZero_Integration(t *testing.T) {
	acc, _ := newAccount()

	// Minting 0 should work
	err := acc.mint(0)
	if err != nil {
		t.Fatalf("mint(0) failed: %v", err)
	}
	if acc.Balance() != 0 {
		t.Errorf("expected balance 0, got %.2f", acc.Balance())
	}
}

func TestAccountBlockchain_Integration(t *testing.T) {
	acc, _ := newAccount()

	blockchain := acc.Blockchain()
	if blockchain == nil {
		t.Fatal("Blockchain() returned nil")
	}

	// Test that blockchain is consistent
	if acc.Blockchain() != blockchain {
		t.Error("Blockchain() returns different instances")
	}

	// Test that minting adds to blockchain
	initialBlocks := len(blockchain.GetBlocks())
	acc.mint(100.0)

	if len(blockchain.GetBlocks()) <= initialBlocks {
		t.Error("mint did not add block to blockchain")
	}
}

func TestAccountString_Integration(t *testing.T) {
	acc, _ := newAccount()
	acc.mint(123.45)

	str := acc.String()
	if str == "" {
		t.Error("String() returned empty string")
	}

	// Check that string contains key information
	idStr := acc.ID().String()
	if len(idStr) < 8 {
		t.Error("ID string too short")
	}
}

func TestAccountConcurrentMint_Integration(t *testing.T) {
	acc, _ := newAccount()
	done := make(chan bool)

	// Mint concurrently from 100 goroutines
	for i := 0; i < 100; i++ {
		go func() {
			err := acc.mint(1.0)
			if err != nil {
				t.Errorf("concurrent mint failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Check final balance
	if acc.Balance() != 100.0 {
		t.Errorf("expected balance 100.0 after concurrent mints, got %.2f", acc.Balance())
	}
}

func TestAccountUpdateValue_Integration(t *testing.T) {
	acc, _ := newAccount()

	// Test updating value
	err := acc.updateValue("key1", "value1")
	if err != nil {
		t.Fatalf("updateValue failed: %v", err)
	}

	// Test that update creates transaction with value
	blockchain := acc.Blockchain()
	blocks := blockchain.GetBlocks()
	if len(blocks) == 0 {
		t.Error("no blocks created after updateValue")
	}

	// Find the transaction with the value
	found := false
	for _, block := range blocks {
		tx := block.Transaction()
		if tx != nil && tx.Value() == "value1" {
			found = true
			if tx.Amount() != 0 {
				t.Error("value transaction should have 0 amount")
			}
		}
	}
	if !found {
		t.Error("value transaction not found in blockchain")
	}
}
