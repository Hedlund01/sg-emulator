package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"sg-emulator/internal/scalegraph"
	"sg-emulator/internal/server/messages"

	"github.com/stretchr/testify/assert"
)

// TestNew verifies server creation
func TestNew(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	assert.NotNil(t, srv, "New() should not return nil")
	assert.NotNil(t, srv.app, "New() server should have non-nil app")
	assert.NotNil(t, srv.registry, "New() server should have non-nil registry")
	assert.Equal(t, 1000, cap(srv.requestChan), "New() request channel capacity")
}

// TestServer_Start verifies server lifecycle
func TestServer_Start(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()

	srv.Start()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	// Verify it's running by sending a request
	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	acc, err := createTestAccount(ctx, srv, client, 100.0)
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
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()

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
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()

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
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()

	// Start multiple times
	srv.Start()
	srv.Start()
	srv.Start()

	time.Sleep(10 * time.Millisecond)

	// Server should still be functional
	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	_, err = createTestAccount(ctx, srv, client, 100.0)
	if err != nil {
		t.Errorf("Server not responding after multiple Start() calls: %v", err)
	}

	srv.Stop()
}

// TestServer_RequestChannel verifies request channel accessor
func TestServer_RequestChannel(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	reqChan := srv.RequestChannel()
	assert.NotNil(t, reqChan, "RequestChannel() should not return nil")
	// RequestChannel() returns chan<- (send-only), which is the same underlying channel
	assert.Equal(t, (chan<- messages.Request)(srv.requestChan), reqChan, "RequestChannel() should return the same channel")
}

// TestServer_Registry verifies registry accessor
func TestServer_Registry(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	registry := srv.Registry()
	assert.NotNil(t, registry, "Registry() should not return nil")
	assert.Equal(t, srv.registry, registry, "Registry() should return the same registry")
}

// TestServer_CreateVirtualApp verifies virtual app creation
func TestServer_CreateVirtualApp(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)
	assert.NotNil(t, vapp, "CreateVirtualApp() should not return nil")

	// Verify it's registered
	retrieved, exists := srv.Registry().GetByID(vapp.ID())
	assert.True(t, exists, "CreateVirtualApp() should register virtual app")
	assert.Same(t, vapp, retrieved, "Registry should return the same virtual app instance")
}

// TestServer_CreateVirtualApp_Multiple verifies multiple virtual apps
func TestServer_CreateVirtualApp_Multiple(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	count := 10
	vapps := make([]*VirtualApp, count)

	for i := 0; i < count; i++ {
		vapp, err := srv.CreateVirtualApp()
		assert.NoError(t, err, "CreateVirtualApp() %d should succeed", i)
		vapps[i] = vapp
	}

	// Verify all have unique IDs
	seen := make(map[scalegraph.ScalegraphId]bool)
	for _, vapp := range vapps {
		assert.False(t, seen[vapp.ID()], "CreateVirtualApp() generated duplicate ID: %s", vapp.ID())
		seen[vapp.ID()] = true
	}

	// Verify all are registered
	assert.Equal(t, count, srv.Registry().Count(), "Registry count")
}

// TestServer_HandleRequest_CreateAccount verifies request handling
func TestServer_HandleRequest_CreateAccount(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()

	signedReq, err := createSignedAccountRequest(srv, 100.0)
	if err != nil {
		t.Fatalf("Failed to create signed account request: %v", err)
	}

	req := messages.Request{
		ID: "test-req-1",
		Payload: &scalegraph.CreateAccountRequest{
			InitialBalance: 100.0,
			SignedEnvelope: signedReq,
		},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	if resp.Error != nil {
		t.Errorf("handleRequest() failed: %v", resp.Error)
	}

	if resp.Payload == nil {
		t.Fatal("handleRequest() returned nil payload")
	}

	createResp, ok := resp.Payload.(*scalegraph.CreateAccountResponse)
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
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	// Create account first with credentials
	acc, err := createTestAccount(ctx, srv, client, 50.0)
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Create signed get account request
	signedReq, err := createSignedGetAccountRequest(srv, acc.ID())
	if err != nil {
		t.Fatalf("Failed to create signed get account request: %v", err)
	}

	req := messages.Request{
		ID: "test-req-2",
		Payload: &scalegraph.GetAccountRequest{
			AccountID:      acc.ID(),
			SignedEnvelope: signedReq,
		},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	if resp.Error != nil {
		t.Errorf("handleRequest() failed: %v", resp.Error)
	}

	getResp := resp.Payload.(*scalegraph.GetAccountResponse)
	if getResp.Account.ID() != acc.ID() {
		t.Error("handleRequest() returned wrong account")
	}
}

// TestServer_HandleRequest_GetAccounts verifies account listing
func TestServer_HandleRequest_GetAccounts(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	// Create multiple accounts
	for i := 0; i < 5; i++ {
		createTestAccountDirect(t, srv.app, float64(i*10))
	}

	req := messages.Request{
		ID:      "test-req-3",
		Payload: &scalegraph.GetAccountsRequest{},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	assert.Nil(t, resp.Error, "handleRequest() failed: %v", resp.Error)
	getResp := resp.Payload.(*scalegraph.GetAccountsResponse)
	assert.Len(t, getResp.Accounts, 5, "handleRequest() should return 5 accounts")
}

// TestServer_HandleRequest_Transfer verifies fund transfer
func TestServer_HandleRequest_Transfer(t *testing.T) {
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

	from, _ := createTestAccount(ctx, srv, client, 100.0)
	to, _ := createTestAccount(ctx, srv, client, 50.0)

	signedTransfer, err := createSignedTransfer(ctx, srv, client, from.ID(), to.ID(), 30.0)
	if err != nil {
		t.Fatalf("Failed to create signed transfer: %v", err)
	}
	_, err = client.TransferSigned(ctx, signedTransfer)
	if err != nil {
		t.Fatalf("TransferSigned() error = %v", err)
	}

	// Verify balances
	fromAcc, _ := getTestAccount(ctx, srv, client, from.ID())
	toAcc, _ := getTestAccount(ctx, srv, client, to.ID())

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
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	acc := createTestAccountDirect(t, srv.app, 100.0)

	req := messages.Request{
		ID: "test-req-5",
		Payload: &scalegraph.MintRequest{
			To:     acc.ID(),
			Amount: 50.0,
		},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	assert.Nil(t, resp.Error, "handleRequest() failed: %v", resp.Error)

	// Verify balance
	getResp, _ := srv.app.GetAccount(context.Background(), &scalegraph.GetAccountRequest{AccountID: acc.ID()})
	assert.Equal(t, 150.0, getResp.Account.Balance(), "Balance after mint")
}

// TestServer_HandleRequest_AccountCount verifies account counting
func TestServer_HandleRequest_AccountCount(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	// Create accounts
	for i := 0; i < 3; i++ {
		createTestAccountDirect(t, srv.app, 0)
	}

	req := messages.Request{
		ID:      "test-req-6",
		Payload: &scalegraph.AccountCountRequest{},
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	assert.Nil(t, resp.Error, "handleRequest() failed: %v", resp.Error)
	countResp := resp.Payload.(*scalegraph.AccountCountResponse)
	assert.Equal(t, 3, countResp.Count, "Count")
}

// TestServer_HandleRequest_UnknownType verifies unknown request type handling
func TestServer_HandleRequest_UnknownType(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	req := messages.Request{
		ID:      "test-req-7",
		Payload: "invalid payload",
		Context: context.Background(),
	}

	resp := srv.handleRequest(req)

	assert.NotNil(t, resp.Error, "handleRequest() with unknown type should fail")
}

// TestServer_HandleRequest_Errors verifies error handling
func TestServer_HandleRequest_Errors(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()

	tests := []struct {
		name    string
		payload any
	}{
		{
			name:    "GetAccount non-existent",
			payload: &scalegraph.GetAccountRequest{AccountID: idRandom1},
		},
		{
			name: "Transfer insufficient funds",
			payload: &scalegraph.TransferRequest{
				From:   idRandom1,
				To:     idRandom2,
				Amount: 100.0,
				Nonce:  1,
			},
		},
		{
			name: "Mint to non-existent",
			payload: &scalegraph.MintRequest{
				To:     idRandom1,
				Amount: 100.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := messages.Request{
				ID:      "test-req",
				Payload: tt.payload,
				Context: context.Background(),
			}

			resp := srv.handleRequest(req)

			if resp.Error == nil {
				t.Error("handleRequest() succeeded, want error")
			}
		})
	}
}

// TestServer_ConcurrentRequests verifies concurrent request processing
func TestServer_ConcurrentRequests(t *testing.T) {
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
	errChan := make(chan error, 100)

	// Send 100 concurrent requests
	for i := 0; i < 100; i++ {
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
	if count != 100 {
		t.Errorf("AccountCount() = %d, want 100", count)
	}
}

// TestServer_StopDuringProcessing verifies graceful shutdown during active requests
func TestServer_StopDuringProcessing(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
	srv.Start()

	client := NewClient(srv.RequestChannel(), logger)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Start many concurrent requests
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			createTestAccount(ctx, srv, client, 100.0)
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
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()
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
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		b.Fatalf("Failed to create test server: %v", err)
	}
	defer cleanup()

	signedReq, err := createSignedAccountRequest(srv, 100.0)
	if err != nil {
		b.Fatalf("Failed to create signed account request: %v", err)
	}

	req := messages.Request{
		ID: "bench-req",
		Payload: &scalegraph.CreateAccountRequest{
			InitialBalance: 100.0,
			SignedEnvelope: signedReq,
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
