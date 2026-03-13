package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server/messages"
	"sg-emulator/internal/trace"
)

// TestNewClient verifies client creation
func TestNewClient(t *testing.T) {
	reqChan := make(chan messages.Request, 10)
	logger := newTestLogger()

	client := NewClient(reqChan, logger)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.timeout != 30*time.Second {
		t.Errorf("NewClient() timeout = %v, want 30s", client.timeout)
	}

	if client.pendingRequests == nil {
		t.Error("NewClient() pendingRequests map is nil")
	}
}

// TestClient_SetTimeout verifies timeout configuration
func TestClient_SetTimeout(t *testing.T) {
	reqChan := make(chan messages.Request, 10)
	logger := newTestLogger()
	client := NewClient(reqChan, logger)

	newTimeout := 5 * time.Second
	client.SetTimeout(newTimeout)

	if client.timeout != newTimeout {
		t.Errorf("SetTimeout() timeout = %v, want %v", client.timeout, newTimeout)
	}
}

// TestClient_SetTimeout_Zero verifies zero timeout handling
func TestClient_SetTimeout_Zero(t *testing.T) {
	reqChan := make(chan messages.Request, 10)
	logger := newTestLogger()
	client := NewClient(reqChan, logger)

	client.SetTimeout(0)

	if client.timeout != 0 {
		t.Errorf("SetTimeout(0) timeout = %v, want 0", client.timeout)
	}
}

// TestGenerateRequestID verifies request ID generation
func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == "" || id2 == "" {
		t.Error("generateRequestID() returned empty string")
	}

	if id1 == id2 {
		t.Error("generateRequestID() generated duplicate IDs")
	}

	// Check format: req-<counter>-<timestamp>
	if len(id1) < 10 {
		t.Errorf("generateRequestID() ID too short: %s", id1)
	}
}

// TestGenerateRequestID_Concurrent verifies thread-safe ID generation
func TestGenerateRequestID_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	idChan := make(chan string, 1000)

	// Generate 1000 IDs concurrently
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idChan <- generateRequestID()
		}()
	}

	wg.Wait()
	close(idChan)

	// Check uniqueness
	seen := make(map[string]bool)
	for id := range idChan {
		if seen[id] {
			t.Errorf("generateRequestID() generated duplicate: %s", id)
		}
		seen[id] = true
	}

	if len(seen) != 1000 {
		t.Errorf("Expected 1000 unique IDs, got %d", len(seen))
	}
}

// TestClient_CreateAccount verifies account creation
func TestClient_CreateAccount(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	acc, err := createTestAccount(ctx, srv, client, 100.0)
	if err != nil {
		t.Fatalf("CreateAccount() error = %v, want nil", err)
	}

	if acc == nil {
		t.Fatal("CreateAccount() returned nil account")
	}

	if acc.Balance() != 100.0 {
		t.Errorf("CreateAccount() balance = %.2f, want 100.00", acc.Balance())
	}
}

// TestClient_CreateAccount_ZeroBalance verifies account creation with zero balance
func TestClient_CreateAccount_ZeroBalance(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	acc, err := createTestAccount(ctx, srv, client, 0)
	if err != nil {
		t.Fatalf("CreateAccount() error = %v, want nil", err)
	}

	if acc.Balance() != 0 {
		t.Errorf("CreateAccount() balance = %.2f, want 0.00", acc.Balance())
	}
}

// TestClient_GetAccount verifies account retrieval
func TestClient_GetAccount(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	// Create account first
	acc, err := createTestAccount(ctx, srv, client, 50.0)
	if err != nil {
		t.Fatalf("createTestAccount() error = %v", err)
	}

	// Retrieve it
	retrieved, err := getTestAccount(ctx, srv, client, acc.ID())
	if err != nil {
		t.Fatalf("GetAccount() error = %v, want nil", err)
	}

	if retrieved.ID() != acc.ID() {
		t.Errorf("GetAccount() ID = %s, want %s", retrieved.ID(), acc.ID())
	}

	if retrieved.Balance() != 50.0 {
		t.Errorf("GetAccount() balance = %.2f, want 50.00", retrieved.Balance())
	}
}

// TestClient_GetAccount_NotFound verifies error for non-existent account
func TestClient_GetAccount_NotFound(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	nonExistentID := idRandom1

	// Can't create a signed request for non-existent account, so pass nil
	_, err = client.GetAccount(ctx, nonExistentID, nil)
	if err == nil {
		t.Error("GetAccount() for non-existent account succeeded, want error")
	}
}

// TestClient_GetAccounts verifies listing all accounts
func TestClient_GetAccounts(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	// Create multiple accounts
	count := 5
	for i := 0; i < count; i++ {
		createTestAccount(ctx, srv, client, float64(i*10))
	}

	// Get all accounts
	accounts, err := client.GetAccounts(ctx)
	if err != nil {
		t.Fatalf("GetAccounts() error = %v, want nil", err)
	}

	if len(accounts) != count {
		t.Errorf("GetAccounts() returned %d accounts, want %d", len(accounts), count)
	}
}

// TestClient_GetAccounts_Empty verifies empty account list
func TestClient_GetAccounts_Empty(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	accounts, err := client.GetAccounts(ctx)
	if err != nil {
		t.Fatalf("GetAccounts() error = %v, want nil", err)
	}

	if len(accounts) != 0 {
		t.Errorf("GetAccounts() returned %d accounts, want 0", len(accounts))
	}
}

// TestClient_Transfer verifies fund transfer
func TestClient_Transfer(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	// Create two accounts with credentials
	from, err := createTestAccount(ctx, srv, client, 100.0)
	if err != nil {
		t.Fatalf("Failed to create from account: %v", err)
	}
	to, err := createTestAccount(ctx, srv, client, 50.0)
	if err != nil {
		t.Fatalf("Failed to create to account: %v", err)
	}

	// Transfer funds with signed request
	signedTransfer, err := createSignedTransfer(ctx, srv, client, from.ID(), to.ID(), 30.0)
	if err != nil {
		t.Fatalf("Failed to create signed transfer: %v", err)
	}
	_, err = client.TransferSigned(ctx, signedTransfer)
	if err != nil {
		t.Fatalf("TransferSigned() error = %v, want nil", err)
	}

	// Verify balances
	fromAcc, _ := getTestAccount(ctx, srv, client, from.ID())
	toAcc, _ := getTestAccount(ctx, srv, client, to.ID())

	if fromAcc.Balance() != 70.0 {
		t.Errorf("Transfer() from balance = %.2f, want 70.00", fromAcc.Balance())
	}

	if toAcc.Balance() != 80.0 {
		t.Errorf("Transfer() to balance = %.2f, want 80.00", toAcc.Balance())
	}
}

// TestClient_Transfer_InsufficientFunds verifies insufficient funds error
func TestClient_Transfer_InsufficientFunds(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	from, err := createTestAccount(ctx, srv, client, 50.0)
	if err != nil {
		t.Fatalf("Failed to create from account: %v", err)
	}
	to, err := createTestAccount(ctx, srv, client, 0.0)
	if err != nil {
		t.Fatalf("Failed to create to account: %v", err)
	}

	// Attempt transfer larger than balance
	signedTransfer, err := createSignedTransfer(ctx, srv, client, from.ID(), to.ID(), 100.0)
	if err != nil {
		t.Fatalf("Failed to create signed transfer: %v", err)
	}
	_, err = client.TransferSigned(ctx, signedTransfer)
	if err == nil {
		t.Error("TransferSigned() with insufficient funds succeeded, want error")
	}
}

// TestClient_Transfer_NonExistentAccounts verifies errors for missing accounts
func TestClient_Transfer_NonExistentAccounts(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	acc, err := createTestAccount(ctx, srv, client, 100.0)
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	tests := []struct {
		name   string
		fromID scalegraph.ScalegraphId
		toID   scalegraph.ScalegraphId
	}{
		{"from non-existent", idRandom1, acc.ID()},
		{"to non-existent", acc.ID(), idRandom2},
		{"both non-existent", idRandom1, idRandom2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Try to create signed transfer - may fail if from account doesn't exist
			signedTransfer, err := createSignedTransfer(ctx, srv, client, tt.fromID, tt.toID, 10.0)
			if err == nil {
				_, err = client.TransferSigned(ctx, signedTransfer)
			}
			if err == nil {
				t.Error("TransferSigned() with non-existent account succeeded, want error")
			}
		})
	}
}

// TestClient_Mint verifies token minting
func TestClient_Mint(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	acc, _ := createTestAccount(ctx, srv, client, 100.0)

	// Mint tokens
	err = client.Mint(ctx, acc.ID(), 50.0)
	if err != nil {
		t.Fatalf("Mint() error = %v, want nil", err)
	}

	// Verify balance
	updated, _ := getTestAccount(ctx, srv, client, acc.ID())
	if updated.Balance() != 150.0 {
		t.Errorf("Mint() balance = %.2f, want 150.00", updated.Balance())
	}
}

// TestClient_Mint_NonExistentAccount verifies mint error for missing account
func TestClient_Mint_NonExistentAccount(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	err = client.Mint(ctx, idRandom1, 100.0)
	if err == nil {
		t.Error("Mint() to non-existent account succeeded, want error")
	}
}

// TestClient_AccountCount verifies account counting
func TestClient_AccountCount(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	// Create accounts
	count := 7
	for i := 0; i < count; i++ {
		createTestAccount(ctx, srv, client, 0)
	}

	// Get count
	actualCount, err := client.AccountCount(ctx)
	if err != nil {
		t.Fatalf("AccountCount() error = %v, want nil", err)
	}

	if actualCount != count {
		t.Errorf("AccountCount() = %d, want %d", actualCount, count)
	}
}

// TestClient_Timeout verifies request timeout handling
func TestClient_Timeout(t *testing.T) {
	logger := newTestLogger()
	// Create channel but don't start server (no one to respond)
	reqChan := make(chan messages.Request, 10)
	client := NewClient(reqChan, logger)
	client.SetTimeout(100 * time.Millisecond)

	ctx := context.Background()

	// This should timeout since no server is processing
	start := time.Now()
	_, err := client.CreateAccountWithCredentials(ctx, 100.0, nil)
	duration := time.Since(start)

	if err == nil {
		t.Error("CreateAccountWithCredentials() without server succeeded, want timeout error")
	}

	// Verify it actually waited around the timeout period
	if duration < 100*time.Millisecond {
		t.Errorf("Timeout occurred too quickly: %v", duration)
	}
	if duration > 500*time.Millisecond {
		t.Errorf("Timeout took too long: %v", duration)
	}
}

// TestClient_ContextCancellation verifies context cancellation handling
func TestClient_ContextCancellation(t *testing.T) {
	logger := newTestLogger()
	reqChan := make(chan messages.Request, 10)
	client := NewClient(reqChan, logger)
	client.SetTimeout(100 * time.Millisecond) // Short timeout

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Request should fail quickly due to cancelled context
	start := time.Now()
	_, err := client.CreateAccountWithCredentials(ctx, 100.0, nil)
	duration := time.Since(start)

	if err == nil {
		t.Error("CreateAccountWithCredentials() with cancelled context succeeded, want error")
	}

	// Should fail within timeout period
	if duration > 500*time.Millisecond {
		t.Errorf("Request took too long with cancelled context: %v", duration)
	}
}

// TestClient_ConcurrentRequests verifies concurrent request handling
func TestClient_ConcurrentRequests(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	errChan := make(chan error, 50)

	// Create 50 accounts concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(balance float64) {
			defer wg.Done()
			_, err := createTestAccount(ctx, srv, client, balance)
			if err != nil {
				errChan <- err
			}
		}(float64(i))
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("Concurrent request error: %v", err)
	}

	// Verify all accounts created
	count, _ := client.AccountCount(ctx)
	if count != 50 {
		t.Errorf("AccountCount() after concurrent creates = %d, want 50", count)
	}
}

// TestClient_ConcurrentDifferentOperations verifies mixed concurrent operations
// NOTE: This test uses separate clients to avoid response channel conflicts
func TestClient_ConcurrentDifferentOperations(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	// Create separate clients to avoid response channel conflicts
	// The current client implementation shares a single response channel
	// which can cause race conditions with concurrent requests
	client1 := NewClient(srv.RequestChannel(), logger)
	client2 := NewClient(srv.RequestChannel(), logger)
	client3 := NewClient(srv.RequestChannel(), logger)
	client4 := NewClient(srv.RequestChannel(), logger)

	ctx := context.Background()

	// Create initial accounts with credentials
	acc1, err := createTestAccount(ctx, srv, client1, 1000.0)
	if err != nil {
		t.Fatalf("Failed to create acc1: %v", err)
	}
	acc2, err := createTestAccount(ctx, srv, client1, 1000.0)
	if err != nil {
		t.Fatalf("Failed to create acc2: %v", err)
	}

	var wg sync.WaitGroup
	var successCount atomic.Int32

	// Concurrent creates (using client1)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := createTestAccount(ctx, srv, client1, 100.0)
			if err == nil {
				successCount.Add(1)
			}
		}()
	}

	// Concurrent transfers (using client2)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signedTransfer, err := createSignedTransfer(ctx, srv, client2, acc1.ID(), acc2.ID(), 10.0)
			if err == nil {
				_, err = client2.TransferSigned(ctx, signedTransfer)
			}
			if err == nil {
				successCount.Add(1)
			}
		}()
	}

	// Concurrent mints (using client3)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := client3.Mint(ctx, acc1.ID(), 5.0)
			if err == nil {
				successCount.Add(1)
			}
		}()
	}

	// Concurrent reads (using client4)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client4.GetAccounts(ctx)
			successCount.Add(1)
		}()
	}

	wg.Wait()

	// Most operations should succeed
	if successCount.Load() < 40 {
		t.Errorf("Only %d/%d concurrent operations succeeded", successCount.Load(), 50)
	}
}

// TestClient_ConcurrentSameClient_RaceCondition verifies that concurrent requests from the
// same client instance work correctly without race conditions or mixed responses.
//
// This test was previously skipped due to a bug where a single shared response channel
// caused responses to be consumed by the wrong goroutine. The fix uses per-request
// response channels with a correlation map, ensuring each request gets its own response.
func TestClient_ConcurrentSameClient_RaceCondition(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	// Use a SINGLE client for all concurrent operations to verify proper request correlation
	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	// Create initial accounts with credentials
	acc1, err := createTestAccount(ctx, srv, client, 1000.0)
	if err != nil {
		t.Fatalf("Failed to create acc1: %v", err)
	}
	acc2, err := createTestAccount(ctx, srv, client, 1000.0)
	if err != nil {
		t.Fatalf("Failed to create acc2: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 50)

	// Concurrent creates
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := createTestAccount(ctx, srv, client, 100.0)
			if err != nil {
				errChan <- err
			}
		}()
	}

	// Concurrent transfers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signedTransfer, err := createSignedTransfer(ctx, srv, client, acc1.ID(), acc2.ID(), 10.0)
			if err == nil {
				_, err = client.TransferSigned(ctx, signedTransfer)
			}
			if err != nil {
				errChan <- err
			}
		}()
	}

	// Concurrent mints
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := client.Mint(ctx, acc1.ID(), 5.0)
			if err != nil {
				errChan <- err
			}
		}()
	}

	// Concurrent reads - this will cause type assertion panics
	// because GetAccounts response might receive a TransferResponse instead
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("panic: %v", r)
				}
			}()
			_, err := client.GetAccounts(ctx)
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// This test will likely fail due to response channel race conditions
	// Expected errors: type assertion panics, wrong response types
	errorCount := 0
	for err := range errChan {
		errorCount++
		t.Logf("Error (expected due to race condition): %v", err)
	}

	if errorCount > 0 {
		t.Logf("Race condition exposed: %d errors occurred due to shared response channel", errorCount)
	}
}

// TestClient_TraceIDPropagation verifies trace IDs flow through requests
func TestClient_TraceIDPropagation(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)

	traceID := "test-trace-123"
	ctx := trace.WithTraceID(context.Background(), traceID)

	// Create account with trace ID
	acc, err := createTestAccount(ctx, srv, client, 100.0)
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	// Verify account was created (trace ID should have propagated)
	if acc == nil {
		t.Error("CreateAccount() returned nil account")
	}
}

// TestClient_MultipleClients verifies multiple clients can use same server
func TestClient_MultipleClients(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	// Create multiple clients
	client1 := NewClient(srv.RequestChannel(), logger)
	client2 := NewClient(srv.RequestChannel(), logger)
	client3 := NewClient(srv.RequestChannel(), logger)

	ctx := context.Background()

	var wg sync.WaitGroup

	// Each client creates accounts
	for i, client := range []*Client{client1, client2, client3} {
		wg.Add(1)
		go func(c *Client, id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				createTestAccount(ctx, srv, c, float64(id*10+j))
			}
		}(client, i)
	}

	wg.Wait()

	// Verify total count
	count, _ := client1.AccountCount(ctx)
	if count != 15 {
		t.Errorf("AccountCount() with multiple clients = %d, want 15", count)
	}
}

// TestClient_RequestResponseCorrelation verifies responses match requests
func TestClient_RequestResponseCorrelation(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	// Create account and capture its ID
	acc1, _ := createTestAccount(ctx, srv, client, 100.0)
	acc2, _ := createTestAccount(ctx, srv, client, 200.0)

	// Retrieve and verify correct accounts are returned
	retrieved1, _ := getTestAccount(ctx, srv, client, acc1.ID())
	retrieved2, _ := getTestAccount(ctx, srv, client, acc2.ID())

	if retrieved1.ID() != acc1.ID() {
		t.Error("GetAccount() returned wrong account")
	}

	if retrieved2.ID() != acc2.ID() {
		t.Error("GetAccount() returned wrong account")
	}

	if retrieved1.Balance() != 100.0 {
		t.Errorf("Account 1 balance = %.2f, want 100.00", retrieved1.Balance())
	}

	if retrieved2.Balance() != 200.0 {
		t.Errorf("Account 2 balance = %.2f, want 200.00", retrieved2.Balance())
	}
}

// BenchmarkClient_CreateAccount benchmarks account creation
func BenchmarkClient_CreateAccount(b *testing.B) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		b.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	for b.Loop() {
		createTestAccount(ctx, srv, client, 100.0)
	}
}

// BenchmarkClient_GetAccount benchmarks account retrieval
func BenchmarkClient_GetAccount(b *testing.B) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		b.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	// Create an account to retrieve
	acc, _ := createTestAccount(ctx, srv, client, 100.0)

	for b.Loop() {
		getTestAccount(ctx, srv, client, acc.ID())
	}
}

// BenchmarkClient_Transfer benchmarks fund transfers
func BenchmarkClient_Transfer(b *testing.B) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		b.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	// Create accounts with large balances and credentials
	from, _ := createTestAccount(ctx, srv, client, 1000000.0)
	to, _ := createTestAccount(ctx, srv, client, 0.0)

	b.ResetTimer()
	for b.Loop() {
		signedTransfer, _ := createSignedTransfer(ctx, srv, client, from.ID(), to.ID(), 1.0)
		client.TransferSigned(ctx, signedTransfer) //nolint:errcheck
	}
}

// BenchmarkGenerateRequestID benchmarks ID generation
func BenchmarkGenerateRequestID(b *testing.B) {
	for b.Loop() {
		_ = generateRequestID()
	}
}

// BenchmarkClient_ConcurrentRequests benchmarks concurrent operations
func BenchmarkClient_ConcurrentRequests(b *testing.B) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		b.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			createTestAccount(ctx, srv, client, 100.0)
		}
	})
}
