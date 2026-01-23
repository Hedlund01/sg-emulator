package server

import (
	"context"
	"errors"
	"testing"
	"time"

	mocks "sg-emulator/internal/server/mocks"

	"github.com/stretchr/testify/mock"
)

// TestVirtualApp_ID verifies ID accessor
func TestVirtualApp_ID(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	vapp, err := srv.CreateVirtualApp()
	if err != nil {
		t.Fatalf("CreateVirtualApp() error = %v", err)
	}

	id := vapp.ID()
	if id.String() == "" {
		t.Error("VirtualApp ID is empty")
	}

	// ID should be stable
	id2 := vapp.ID()
	if id != id2 {
		t.Error("VirtualApp ID changed between calls")
	}
}

// TestVirtualApp_Client verifies client accessor
func TestVirtualApp_Client(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	vapp, _ := srv.CreateVirtualApp()

	client := vapp.Client()
	if client == nil {
		t.Error("VirtualApp Client() returned nil")
	}

	// Client should be stable
	client2 := vapp.Client()
	if client != client2 {
		t.Error("VirtualApp Client() returned different instance")
	}
}

// TestVirtualApp_Context verifies context accessor
func TestVirtualApp_Context(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)

	vapp, _ := srv.CreateVirtualApp()

	ctx := vapp.Context()
	if ctx == nil {
		t.Error("VirtualApp Context() returned nil")
	}

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

	mockTransport := mocks.NewMockTransport(t)

	vapp.AddTransport(mockTransport)

	transports := vapp.Transports()
	if len(transports) != 1 {
		t.Errorf("Transports() count = %d, want 1", len(transports))
	}

	if transports[0] != mockTransport {
		t.Error("Transports() returned wrong transport")
	}
}

// TestVirtualApp_AddTransport_Multiple verifies multiple transports
func TestVirtualApp_AddTransport_Multiple(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

	// Add multiple mock transports
	count := 3
	for i := 0; i < count; i++ {
		mockTransport := mocks.NewMockTransport(t)
		vapp.AddTransport(mockTransport)
	}

	transports := vapp.Transports()
	if len(transports) != count {
		t.Errorf("Transports() count = %d, want %d", len(transports), count)
	}
}

// TestVirtualApp_Transports_Empty verifies empty transport list
func TestVirtualApp_Transports_Empty(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

	transports := vapp.Transports()
	if len(transports) != 0 {
		t.Errorf("Transports() count = %d, want 0", len(transports))
	}
}

// TestVirtualApp_Addresses verifies address map generation
func TestVirtualApp_Addresses(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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

	if len(addresses) != 2 {
		t.Errorf("Addresses() count = %d, want 2", len(addresses))
	}

	if addresses["REST"] != "localhost:8080" {
		t.Errorf("Addresses()[REST] = %s, want localhost:8080", addresses["REST"])
	}

	if addresses["gRPC"] != "localhost:50051" {
		t.Errorf("Addresses()[gRPC] = %s, want localhost:50051", addresses["gRPC"])
	}
}

// TestVirtualApp_Addresses_Empty verifies empty address map
func TestVirtualApp_Addresses_Empty(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

	addresses := vapp.Addresses()
	if len(addresses) != 0 {
		t.Errorf("Addresses() count = %d, want 0", len(addresses))
	}
}

// TestVirtualApp_Start verifies transport startup
func TestVirtualApp_Start(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

	// Start with no transports should not panic
	vapp.Start()
	time.Sleep(10 * time.Millisecond)
	vapp.Stop()
}

// TestVirtualApp_Stop verifies transport shutdown
func TestVirtualApp_Stop(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	srv := New(logger)
	vapp, _ := srv.CreateVirtualApp()

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
	if len(vapp.Transports()) != 2 {
		t.Errorf("Transports() count = %d, want 2", len(vapp.Transports()))
	}

	vapp.Stop()
}

// TestVirtualApp_ClientIntegration verifies client works with virtual app
func TestVirtualApp_ClientIntegration(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	srv.Start()
	defer srv.Stop()

	vapp, _ := srv.CreateVirtualApp()

	// Use virtual app's client to make requests
	client := vapp.Client()
	ctx := context.Background()

	acc, err := client.CreateAccount(ctx, 100.0)
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	if acc.Balance() != 100.0 {
		t.Errorf("Account balance = %.2f, want 100.00", acc.Balance())
	}

	// Verify the account is in the shared app state
	retrieved, err := srv.app.GetAccount(ctx, acc.ID())
	if err != nil {
		t.Errorf("GetAccount() error = %v", err)
	}

	if retrieved.ID() != acc.ID() {
		t.Error("Virtual app created account not in shared state")
	}
}

// TestVirtualApp_MultipleVirtualApps verifies multiple virtual apps work independently
func TestVirtualApp_MultipleVirtualApps(t *testing.T) {
	logger := newTestLogger()
	srv := New(logger)
	srv.Start()
	defer srv.Stop()

	// Create multiple virtual apps
	vapp1, _ := srv.CreateVirtualApp()
	vapp2, _ := srv.CreateVirtualApp()
	vapp3, _ := srv.CreateVirtualApp()

	ctx := context.Background()

	// Each creates an account
	acc1, _ := vapp1.Client().CreateAccount(ctx, 100.0)
	acc2, _ := vapp2.Client().CreateAccount(ctx, 200.0)
	acc3, _ := vapp3.Client().CreateAccount(ctx, 300.0)

	// All should have unique IDs
	if acc1.ID() == acc2.ID() || acc2.ID() == acc3.ID() || acc1.ID() == acc3.ID() {
		t.Error("Virtual apps created accounts with duplicate IDs")
	}

	// All should be in shared state
	count := srv.app.AccountCount(ctx)
	if count != 3 {
		t.Errorf("AccountCount() = %d, want 3", count)
	}
}

// BenchmarkVirtualApp_Start benchmarks startup
func BenchmarkVirtualApp_Start(b *testing.B) {
	logger := newTestLogger()
	srv := New(logger)

	// Pre-create virtual apps
	vapps := make([]*VirtualApp, b.N)
	for i := 0; i < b.N; i++ {
		vapp, _ := srv.CreateVirtualApp()
		mockTransport := mocks.NewMockTransport(b)
		mockTransport.EXPECT().Start(mock.Anything).Return(nil).Maybe()
		mockTransport.EXPECT().Stop().Return(nil).Maybe()
		vapp.AddTransport(mockTransport)
		vapps[i] = vapp
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vapps[i].Start()
	}
}
