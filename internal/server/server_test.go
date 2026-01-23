package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"sg-emulator/internal/scalegraph"
)

// TestNew verifies server creation
func TestNew(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	if srv == nil {
		t.Fatal("New() returned nil")
	}

	if srv.app == nil {
		t.Error("New() server has nil app")
	}

	if srv.registry == nil {
		t.Error("New() server has nil registry")
	}

	if cap(srv.requestChan) != 1000 {
		t.Errorf("New() request channel capacity = %d, want 1000", cap(srv.requestChan))
	}
}

// TestServer_Start verifies server lifecycle
func TestServer_Start(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	srv.Start()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	// Verify it's running by sending a request
	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	acc, err := client.CreateAccount(ctx, 100.0)
	if err != nil {
		t.Errorf("Server not responding after Start(): %v", err)
	}
	if acc == nil {
		t.Error("Server returned nil account")
	}

	srv.Stop()
}

// TestServer_Stop verifies graceful shutdown
func TestServer_Stop(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	srv.Start()
	time.Sleep(10 * time.Millisecond)

	// Stop the server
	srv.Stop()

	// Server should shut down gracefully
	// Verify context is cancelled
	select {
	case <-srv.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Server context not cancelled after Stop()")
	}
}

// TestServer_Stop_Multiple verifies multiple Stop calls are safe
func TestServer_Stop_Multiple(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	srv.Start()
	time.Sleep(10 * time.Millisecond)

	// Call Stop multiple times
	srv.Stop()
	srv.Stop()
	srv.Stop()

	// Should not panic or cause issues
}

// TestServer_Start_Multiple verifies multiple Start calls
func TestServer_Start_Multiple(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	// Start multiple times
	srv.Start()
	srv.Start()
	srv.Start()

	time.Sleep(10 * time.Millisecond)

	// Server should still be functional
	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	_, err := client.CreateAccount(ctx, 100.0)
	if err != nil {
		t.Errorf("Server not responding after multiple Start() calls: %v", err)
	}

	srv.Stop()
}

// TestServer_RequestChannel verifies request channel accessor
func TestServer_RequestChannel(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	reqChan := srv.RequestChannel()
	if reqChan == nil {
		t.Error("RequestChannel() returned nil")
	}

	// Verify it's the same channel
	if reqChan != srv.requestChan {
		t.Error("RequestChannel() returned different channel")
	}
}

// TestServer_Registry verifies registry accessor
func TestServer_Registry(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	registry := srv.Registry()
	if registry == nil {
		t.Error("Registry() returned nil")
	}

	if registry != srv.registry {
		t.Error("Registry() returned different registry")
	}
}

// TestServer_CreateVirtualApp verifies virtual app creation
func TestServer_CreateVirtualApp(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	vapp, err := srv.CreateVirtualApp()
	if err != nil {
		t.Fatalf("CreateVirtualApp() error = %v, want nil", err)
	}

	if vapp == nil {
		t.Fatal("CreateVirtualApp() returned nil")
	}

	// Verify it's registered
	registry := srv.Registry()
	retrieved, exists := registry.GetByID(vapp.ID())
	if !exists {
		t.Error("CreateVirtualApp() did not register virtual app")
	}
	if retrieved != vapp {
		t.Error("Registry returned different virtual app instance")
	}
}

// TestServer_CreateVirtualApp_Multiple verifies multiple virtual apps
func TestServer_CreateVirtualApp_Multiple(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	count := 10
	vapps := make([]*VirtualApp, count)

	for i := 0; i < count; i++ {
		vapp, err := srv.CreateVirtualApp()
		if err != nil {
			t.Fatalf("CreateVirtualApp() %d error = %v", i, err)
		}
		vapps[i] = vapp
	}

	// Verify all have unique IDs
	seen := make(map[scalegraph.ScalegraphId]bool)
	for _, vapp := range vapps {
		if seen[vapp.ID()] {
			t.Errorf("CreateVirtualApp() generated duplicate ID: %s", vapp.ID())
		}
		seen[vapp.ID()] = true
	}

	// Verify all are registered
	if srv.Registry().Count() != count {
		t.Errorf("Registry count = %d, want %d", srv.Registry().Count(), count)
	}
}

// TestServer_HandleRequest_CreateAccount verifies request handling
func TestServer_HandleRequest_CreateAccount(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	req := Request{
		ID:   "test-req-1",
		Type: ReqCreateAccount,
		Payload: CreateAccountPayload{
			InitialBalance: 100.0,
		},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	if !resp.Success {
		t.Errorf("handleRequest() failed: %s", resp.Error)
	}

	if resp.Payload == nil {
		t.Fatal("handleRequest() returned nil payload")
	}

	createResp, ok := resp.Payload.(CreateAccountResponse)
	if !ok {
		t.Fatal("handleRequest() returned wrong payload type")
	}

	if createResp.Account.Balance() != 100.0 {
		t.Errorf("Account balance = %.2f, want 100.00", createResp.Account.Balance())
	}
}

// TestServer_HandleRequest_GetAccount verifies account retrieval
func TestServer_HandleRequest_GetAccount(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	// Create account first
	acc, _ := srv.app.CreateAccount(context.Background(), 50.0)

	req := Request{
		ID:   "test-req-2",
		Type: ReqGetAccount,
		Payload: GetAccountPayload{
			ID: acc.ID(),
		},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	if !resp.Success {
		t.Errorf("handleRequest() failed: %s", resp.Error)
	}

	getResp := resp.Payload.(GetAccountResponse)
	if getResp.Account.ID() != acc.ID() {
		t.Error("handleRequest() returned wrong account")
	}
}

// TestServer_HandleRequest_GetAccounts verifies account listing
func TestServer_HandleRequest_GetAccounts(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	// Create multiple accounts
	for i := 0; i < 5; i++ {
		srv.app.CreateAccount(context.Background(), float64(i*10))
	}

	req := Request{
		ID:      "test-req-3",
		Type:    ReqGetAccounts,
		Payload: GetAccountsPayload{},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	if !resp.Success {
		t.Errorf("handleRequest() failed: %s", resp.Error)
	}

	getResp := resp.Payload.(GetAccountsResponse)
	if len(getResp.Accounts) != 5 {
		t.Errorf("handleRequest() returned %d accounts, want 5", len(getResp.Accounts))
	}
}

// TestServer_HandleRequest_Transfer verifies fund transfer
func TestServer_HandleRequest_Transfer(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	from, _ := srv.app.CreateAccount(context.Background(), 100.0)
	to, _ := srv.app.CreateAccount(context.Background(), 50.0)

	req := Request{
		ID:   "test-req-4",
		Type: ReqTransfer,
		Payload: TransferPayload{
			From:   from.ID(),
			To:     to.ID(),
			Amount: 30.0,
		},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	if !resp.Success {
		t.Errorf("handleRequest() failed: %s", resp.Error)
	}

	// Verify balances
	fromAcc, _ := srv.app.GetAccount(context.Background(), from.ID())
	toAcc, _ := srv.app.GetAccount(context.Background(), to.ID())

	if fromAcc.Balance() != 70.0 {
		t.Errorf("From balance = %.2f, want 70.00", fromAcc.Balance())
	}
	if toAcc.Balance() != 80.0 {
		t.Errorf("To balance = %.2f, want 80.00", toAcc.Balance())
	}
}

// TestServer_HandleRequest_Mint verifies token minting
func TestServer_HandleRequest_Mint(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	acc, _ := srv.app.CreateAccount(context.Background(), 100.0)

	req := Request{
		ID:   "test-req-5",
		Type: ReqMint,
		Payload: MintPayload{
			To:     acc.ID(),
			Amount: 50.0,
		},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	if !resp.Success {
		t.Errorf("handleRequest() failed: %s", resp.Error)
	}

	// Verify balance
	updated, _ := srv.app.GetAccount(context.Background(), acc.ID())
	if updated.Balance() != 150.0 {
		t.Errorf("Balance = %.2f, want 150.00", updated.Balance())
	}
}

// TestServer_HandleRequest_AccountCount verifies account counting
func TestServer_HandleRequest_AccountCount(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	// Create accounts
	for i := 0; i < 3; i++ {
		srv.app.CreateAccount(context.Background(), 0)
	}

	req := Request{
		ID:      "test-req-6",
		Type:    ReqAccountCount,
		Payload: AccountCountPayload{},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	if !resp.Success {
		t.Errorf("handleRequest() failed: %s", resp.Error)
	}

	countResp := resp.Payload.(AccountCountResponse)
	if countResp.Count != 3 {
		t.Errorf("Count = %d, want 3", countResp.Count)
	}
}

// TestServer_HandleRequest_UnknownType verifies unknown request type handling
func TestServer_HandleRequest_UnknownType(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	req := Request{
		ID:      "test-req-7",
		Type:    99, // Invalid request type
		Payload: nil,
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	if resp.Success {
		t.Error("handleRequest() with unknown type succeeded, want error")
	}

	if resp.Error == "" {
		t.Error("handleRequest() with unknown type returned empty error")
	}
}

// TestServer_HandleRequest_Errors verifies error handling
func TestServer_HandleRequest_Errors(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	tests := []struct {
		name    string
		reqType RequestType
		payload any
	}{
		{
			name:    "GetAccount non-existent",
			reqType: ReqGetAccount,
			payload: GetAccountPayload{ID: idRandom1},
		},
		{
			name:    "Transfer insufficient funds",
			reqType: ReqTransfer,
			payload: TransferPayload{
				From:   idRandom1,
				To:     idRandom2,
				Amount: 100.0,
			},
		},
		{
			name:    "Mint to non-existent",
			reqType: ReqMint,
			payload: MintPayload{
				To:     idRandom1,
				Amount: 100.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := Request{
				ID:      "test-req",
				Type:    tt.reqType,
				Payload: tt.payload,
				Context: context.Background(),
			}

			resp := srv.handleRequest(req)

			if resp.Success {
				t.Error("handleRequest() succeeded, want error")
			}
		})
	}
}

// TestServer_ConcurrentRequests verifies concurrent request processing
func TestServer_ConcurrentRequests(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// Send 100 concurrent requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(balance float64) {
			defer wg.Done()
			_, err := client.CreateAccount(ctx, balance)
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
	if count != 100 {
		t.Errorf("AccountCount() = %d, want 100", count)
	}
}

// TestServer_StopDuringProcessing verifies graceful shutdown during active requests
func TestServer_StopDuringProcessing(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	srv.Start()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Start many concurrent requests
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.CreateAccount(ctx, 100.0)
		}()
	}

	// Stop server while requests are in flight
	time.Sleep(10 * time.Millisecond)
	srv.Stop()

	// Wait for all goroutines to complete
	// Some may succeed, some may fail - we just want no panics
	wg.Wait()

	// Server should be stopped
	select {
	case <-srv.ctx.Done():
		// Expected
	default:
		t.Error("Server context not cancelled after Stop()")
	}
}

// TestServer_CreateVirtualApp_AfterStop verifies behavior after shutdown
func TestServer_CreateVirtualApp_AfterStop(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	srv.Start()
	srv.Stop()

	// Attempt to create virtual app after shutdown
	// This should still work as CreateVirtualApp doesn't depend on running state
	vapp, err := srv.CreateVirtualApp()
	if err != nil {
		t.Errorf("CreateVirtualApp() after Stop() error = %v", err)
	}
	if vapp == nil {
		t.Error("CreateVirtualApp() after Stop() returned nil")
	}
}

// BenchmarkServer_HandleRequest benchmarks request processing
func BenchmarkServer_HandleRequest(b *testing.B) {
	logger := newTestLogger()
	srv := New(logger)

	req := Request{
		ID:   "bench-req",
		Type: ReqCreateAccount,
		Payload: CreateAccountPayload{
			InitialBalance: 100.0,
		},
		Context: context.Background(),
	}

	for b.Loop() {
		srv.handleRequest(req)
	}
}

// BenchmarkServer_ConcurrentRequests benchmarks concurrent processing
func BenchmarkServer_ConcurrentRequests(b *testing.B) {
	logger := newTestLogger()
	srv := New(logger)
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client.CreateAccount(ctx, 100.0)
		}
	})
}
