package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"sg-emulator/internal/scalegraph"
	mocks "sg-emulator/internal/server/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestVirtualApp_ID verifies ID accessor
func TestVirtualApp_ID(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	id := vapp.ID()
	assert.NotEmpty(t, id.String(), "VirtualApp ID should not be empty")

	// ID should be stable
	assert.Equal(t, id, vapp.ID(), "VirtualApp ID should be stable between calls")
}

// TestVirtualApp_Client verifies client accessor
func TestVirtualApp_Client(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	client := vapp.Client()
	assert.NotNil(t, client, "VirtualApp Client() should not return nil")
	assert.Same(t, client, vapp.Client(), "VirtualApp Client() should return the same instance")
}

// TestVirtualApp_Context verifies context accessor
func TestVirtualApp_Context(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	ctx := vapp.Context()
	assert.NotNil(t, ctx, "VirtualApp Context() should not return nil")

	// Context should not be cancelled initially
	select {
	case <-ctx.Done():
		t.Error("VirtualApp context cancelled before Stop()")
	default:
		// Expected
	}
}

// TestVirtualApp_AddTransport verifies transport addition
func TestVirtualApp_AddTransport(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	mockTransport := mocks.NewMockTransport(t)
	vapp.AddTransport(mockTransport)

	transports := vapp.Transports()
	assert.Len(t, transports, 1, "Transports() count")
	assert.Equal(t, mockTransport, transports[0], "Transports() should return the added transport")
}

// TestVirtualApp_AddTransport_Multiple verifies multiple transports
func TestVirtualApp_AddTransport_Multiple(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	count := 3
	for i := 0; i < count; i++ {
		vapp.AddTransport(mocks.NewMockTransport(t))
	}

	assert.Len(t, vapp.Transports(), count, "Transports() count")
}

// TestVirtualApp_Transports_Empty verifies empty transport list
func TestVirtualApp_Transports_Empty(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	assert.Empty(t, vapp.Transports(), "Transports() should be empty initially")
}

// TestVirtualApp_Addresses verifies address map generation
func TestVirtualApp_Addresses(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	// Add mock transports with different types and addresses
	mockREST := mocks.NewMockTransport(t)
	mockREST.EXPECT().Type().Return("REST")
	mockREST.EXPECT().Address().Return("localhost:8080")

	mockGRPC := mocks.NewMockTransport(t)
	mockGRPC.EXPECT().Type().Return("gRPC")
	mockGRPC.EXPECT().Address().Return("localhost:50051")

	vapp.AddTransport(mockREST)
	vapp.AddTransport(mockGRPC)

	addresses := vapp.Addresses()

	assert.Len(t, addresses, 2, "Addresses() count")
	assert.Equal(t, "localhost:8080", addresses["REST"], "Addresses()[REST]")
	assert.Equal(t, "localhost:50051", addresses["gRPC"], "Addresses()[gRPC]")
}

// TestVirtualApp_Addresses_Empty verifies empty address map
func TestVirtualApp_Addresses_Empty(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	assert.Empty(t, vapp.Addresses(), "Addresses() should be empty with no transports")
}

// TestVirtualApp_Start verifies transport startup
func TestVirtualApp_Start(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	mockTransport := mocks.NewMockTransport(t)
	mockTransport.EXPECT().Start(mock.Anything).Return(nil).Once()

	vapp.AddTransport(mockTransport)
	vapp.Start()

	// Give transports time to start
	time.Sleep(20 * time.Millisecond)

	// Cleanup
	mockTransport.EXPECT().Stop().Return(nil).Once()
	vapp.Stop()
}

// TestVirtualApp_Start_MultipleTransports verifies all transports start
func TestVirtualApp_Start_MultipleTransports(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	// Add multiple transports
	mock1 := mocks.NewMockTransport(t)
	mock1.EXPECT().Start(mock.Anything).Return(nil).Once()
	mock1.EXPECT().Stop().Return(nil).Once()

	mock2 := mocks.NewMockTransport(t)
	mock2.EXPECT().Start(mock.Anything).Return(nil).Once()
	mock2.EXPECT().Stop().Return(nil).Once()

	mock3 := mocks.NewMockTransport(t)
	mock3.EXPECT().Start(mock.Anything).Return(nil).Once()
	mock3.EXPECT().Stop().Return(nil).Once()

	vapp.AddTransport(mock1)
	vapp.AddTransport(mock2)
	vapp.AddTransport(mock3)

	vapp.Start()
	time.Sleep(20 * time.Millisecond)

	vapp.Stop()
}

// TestVirtualApp_Start_NoTransports verifies start with no transports
func TestVirtualApp_Start_NoTransports(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	// Start with no transports should not panic
	vapp.Start()
	time.Sleep(10 * time.Millisecond)
	vapp.Stop()
}

// TestVirtualApp_Stop verifies transport shutdown
func TestVirtualApp_Stop(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	mockTransport := mocks.NewMockTransport(t)
	mockTransport.EXPECT().Start(mock.Anything).Return(nil).Once()
	mockTransport.EXPECT().Stop().Return(nil).Once()

	vapp.AddTransport(mockTransport)
	vapp.Start()
	time.Sleep(20 * time.Millisecond)

	vapp.Stop()

	// Verify context cancelled
	select {
	case <-vapp.Context().Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("VirtualApp context not cancelled after Stop()")
	}
}

// TestVirtualApp_Stop_Multiple verifies multiple Stop calls are safe
func TestVirtualApp_Stop_Multiple(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	mockTransport := mocks.NewMockTransport(t)
	mockTransport.EXPECT().Start(mock.Anything).Return(nil).Once()
	// Stop will be called multiple times
	mockTransport.EXPECT().Stop().Return(nil).Maybe()

	vapp.AddTransport(mockTransport)
	vapp.Start()
	time.Sleep(20 * time.Millisecond)

	// Call Stop multiple times
	vapp.Stop()
	vapp.Stop()
	vapp.Stop()

	// Should not panic
}

// TestVirtualApp_Stop_WithoutStart verifies Stop before Start
func TestVirtualApp_Stop_WithoutStart(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	mockTransport := mocks.NewMockTransport(t)
	// Stop should be called even if Start wasn't
	mockTransport.EXPECT().Stop().Return(nil).Once()

	vapp.AddTransport(mockTransport)

	// Stop without Start should not panic
	vapp.Stop()
}

// TestVirtualApp_Start_TransportError verifies transport start error handling
func TestVirtualApp_Start_TransportError(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	mockTransport := mocks.NewMockTransport(t)
	mockTransport.EXPECT().Start(mock.Anything).Return(errors.New("start failed")).Once()

	vapp.AddTransport(mockTransport)
	vapp.Start()

	// Wait for start attempt
	time.Sleep(20 * time.Millisecond)

	// Cleanup - stop should still be called
	mockTransport.EXPECT().Stop().Return(nil).Once()
	vapp.Stop()
}

// TestVirtualApp_Stop_TransportError verifies transport stop error handling
func TestVirtualApp_Stop_TransportError(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	mockTransport := mocks.NewMockTransport(t)
	mockTransport.EXPECT().Start(mock.Anything).Return(nil).Once()
	mockTransport.EXPECT().Stop().Return(errors.New("stop failed")).Once()

	vapp.AddTransport(mockTransport)
	vapp.Start()
	time.Sleep(20 * time.Millisecond)

	// Stop should not panic even if transport fails
	vapp.Stop()
}

// TestVirtualApp_Start_MixedTransportSuccess verifies partial failures
func TestVirtualApp_Start_MixedTransportSuccess(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	// One transport succeeds
	mock1 := mocks.NewMockTransport(t)
	mock1.EXPECT().Start(mock.Anything).Return(nil).Once()
	mock1.EXPECT().Stop().Return(nil).Once()

	// One transport fails
	mock2 := mocks.NewMockTransport(t)
	mock2.EXPECT().Start(mock.Anything).Return(errors.New("failed")).Once()
	mock2.EXPECT().Stop().Return(nil).Once()

	vapp.AddTransport(mock1)
	vapp.AddTransport(mock2)

	vapp.Start()
	time.Sleep(20 * time.Millisecond)

	// Should still stop cleanly
	vapp.Stop()
}

// TestVirtualApp_ContextCancellation verifies context propagation
func TestVirtualApp_ContextCancellation(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	mockTransport := mocks.NewMockTransport(t)

	// Start should receive a context that gets cancelled on Stop
	startCalled := make(chan context.Context, 1)
	mockTransport.EXPECT().Start(mock.Anything).Run(func(ctx context.Context) {
		startCalled <- ctx
	}).Return(nil).Once()

	vapp.AddTransport(mockTransport)
	vapp.Start()

	// Get the context passed to Start
	var transportCtx context.Context
	select {
	case transportCtx = <-startCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Start was not called")
	}

	// Context should not be cancelled yet
	select {
	case <-transportCtx.Done():
		t.Error("Transport context cancelled before Stop()")
	default:
		// Expected
	}

	mockTransport.EXPECT().Stop().Return(nil).Once()
	vapp.Stop()

	// Context should now be cancelled
	select {
	case <-transportCtx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Transport context not cancelled after Stop()")
	}
}

// TestVirtualApp_AddTransport_AfterStart verifies adding transport after start
func TestVirtualApp_AddTransport_AfterStart(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	mock1 := mocks.NewMockTransport(t)
	mock1.EXPECT().Start(mock.Anything).Return(nil).Once()
	mock1.EXPECT().Stop().Return(nil).Once()

	vapp.AddTransport(mock1)
	vapp.Start()
	time.Sleep(20 * time.Millisecond)

	// Add another transport after Start
	mock2 := mocks.NewMockTransport(t)
	// This transport won't be started automatically
	mock2.EXPECT().Stop().Return(nil).Once()

	vapp.AddTransport(mock2)

	// Verify both transports present
	assert.Len(t, vapp.Transports(), 2, "Transports() count")

	vapp.Stop()
}

// TestVirtualApp_ClientIntegration verifies client works with virtual app
func TestVirtualApp_ClientIntegration(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("newTestServer() error = %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	vapp, _ := srv.CreateVirtualApp()

	// Use virtual app's client to make requests
	client := vapp.Client()
	ctx := context.Background()

	acc, err := createTestAccount(ctx, srv, client, 100.0)
	if err != nil {
		t.Fatalf("createTestAccount() error = %v", err)
	}

	if acc.Balance() != 100.0 {
		t.Errorf("Account balance = %.2f, want 100.00", acc.Balance())
	}

	// Verify the account is in the shared app state
	getResp, err := srv.app.GetAccount(ctx, &scalegraph.GetAccountRequest{AccountID: acc.ID()})
	if err != nil {
		t.Errorf("GetAccount() error = %v", err)
	}

	if getResp.Account.ID() != acc.ID() {
		t.Error("Virtual app created account not in shared state")
	}
}

// TestVirtualApp_MultipleVirtualApps verifies multiple virtual apps work independently
func TestVirtualApp_MultipleVirtualApps(t *testing.T) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		t.Fatalf("newTestServer() error = %v", err)
	}
	defer cleanup()
	srv.Start()
	defer srv.Stop()

	// Create multiple virtual apps
	vapp1, _ := srv.CreateVirtualApp()
	vapp2, _ := srv.CreateVirtualApp()
	vapp3, _ := srv.CreateVirtualApp()

	ctx := context.Background()

	// Each creates an account
	acc1, _ := createTestAccount(ctx, srv, vapp1.Client(), 100.0)
	acc2, _ := createTestAccount(ctx, srv, vapp2.Client(), 200.0)
	acc3, _ := createTestAccount(ctx, srv, vapp3.Client(), 300.0)

	// All should have unique IDs
	if acc1.ID() == acc2.ID() || acc2.ID() == acc3.ID() || acc1.ID() == acc3.ID() {
		t.Error("Virtual apps created accounts with duplicate IDs")
	}

	// All should be in shared state
	countResp, err := srv.app.AccountCount(ctx, &scalegraph.AccountCountRequest{})
	if err != nil {
		t.Fatalf("AccountCount() error = %v", err)
	}
	if countResp.Count != 3 {
		t.Errorf("AccountCount() = %d, want 3", countResp.Count)
	}
}

// BenchmarkVirtualApp_Start benchmarks startup
func BenchmarkVirtualApp_Start(b *testing.B) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup()

	// Pre-create virtual apps to exclude creation/mock setup from benchmark
	vapps := make([]*VirtualApp, 0, 1000)
	for i := 0; i < 1000; i++ {
		vapp, _ := srv.CreateVirtualApp()
		mockTransport := mocks.NewMockTransport(b)
		mockTransport.EXPECT().Start(mock.Anything).Return(nil).Maybe()
		mockTransport.EXPECT().Stop().Return(nil).Maybe()
		vapp.AddTransport(mockTransport)
		vapps = append(vapps, vapp)
	}

	idx := 0
	for b.Loop() {
		vapps[idx%len(vapps)].Start()
		idx++
	}
}
