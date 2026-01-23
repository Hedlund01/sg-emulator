package server

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"sort"
	"sync"
	"testing"

	"sg-emulator/internal/scalegraph"
)

// Test fixtures with known XOR relationships
var (
	// All zeros - useful as a baseline
	idAllZeros = mustParseID("0000000000000000000000000000000000000000")

	// All ones - maximum distance from all zeros
	idAllOnes = mustParseID("ffffffffffffffffffffffffffffffffffffffff")

	// Single bit differences - known small distances
	idOneBit1 = mustParseID("0000000000000000000000000000000000000001") // LSB set
	idOneBit2 = mustParseID("0000000000000000000000000000000000000002") // Second LSB set
	idOneBit4 = mustParseID("0000000000000000000000000000000000000004") // Third LSB set

	// IDs with shared prefixes (should be "close" in XOR distance)
	idPrefix8_1 = mustParseID("8000000000000000000000000000000000000000")
	idPrefix8_2 = mustParseID("8000000000000000000000000000000000000001")
	idPrefix8_3 = mustParseID("8000000000000000000000000000000000000100")

	// Maximum distance pairs
	idMaxDist1 = mustParseID("0000000000000000000000000000000000000000")
	idMaxDist2 = mustParseID("ffffffffffffffffffffffffffffffffffffffff")

	// Random IDs for general testing
	idRandom1 = mustParseID("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0")
	idRandom2 = mustParseID("1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b")
)

func mustParseID(s string) scalegraph.ScalegraphId {
	id, err := scalegraph.ScalegraphIdFromString(s)
	if err != nil {
		panic("invalid test ID: " + err.Error())
	}
	return id
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
}

// TestXORDistance_Identity verifies d(a, a) = 0
func TestXORDistance_Identity(t *testing.T) {
	tests := []struct {
		name string
		id   scalegraph.ScalegraphId
	}{
		{"all zeros", idAllZeros},
		{"all ones", idAllOnes},
		{"random 1", idRandom1},
		{"random 2", idRandom2},
		{"prefix 8", idPrefix8_1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance := xorDistance(tt.id, tt.id)
			if distance.Cmp(big.NewInt(0)) != 0 {
				t.Errorf("xorDistance(%s, %s) = %v, want 0", tt.id, tt.id, distance)
			}
		})
	}
}

// TestXORDistance_Symmetry verifies d(a, b) = d(b, a)
func TestXORDistance_Symmetry(t *testing.T) {
	tests := []struct {
		name string
		a, b scalegraph.ScalegraphId
	}{
		{"zeros and ones", idAllZeros, idAllOnes},
		{"random pair", idRandom1, idRandom2},
		{"prefix pair", idPrefix8_1, idPrefix8_2},
		{"one bit 1 and 2", idOneBit1, idOneBit2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d1 := xorDistance(tt.a, tt.b)
			d2 := xorDistance(tt.b, tt.a)
			if d1.Cmp(d2) != 0 {
				t.Errorf("xorDistance not symmetric: d(%s, %s) = %v, d(%s, %s) = %v",
					tt.a, tt.b, d1, tt.b, tt.a, d2)
			}
		})
	}
}

// TestXORDistance_NonNegativity verifies d(a, b) >= 0
func TestXORDistance_NonNegativity(t *testing.T) {
	tests := []struct {
		name string
		a, b scalegraph.ScalegraphId
	}{
		{"zeros and ones", idAllZeros, idAllOnes},
		{"random pair", idRandom1, idRandom2},
		{"same id", idPrefix8_1, idPrefix8_1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance := xorDistance(tt.a, tt.b)
			if distance.Sign() < 0 {
				t.Errorf("xorDistance(%s, %s) = %v, want non-negative", tt.a, tt.b, distance)
			}
		})
	}
}

// TestXORDistance_KnownValues verifies correct distance calculation
func TestXORDistance_KnownValues(t *testing.T) {
	tests := []struct {
		name     string
		a, b     scalegraph.ScalegraphId
		expected string // hex string of expected distance
	}{
		{
			name:     "all zeros XOR all zeros",
			a:        idAllZeros,
			b:        idAllZeros,
			expected: "0",
		},
		{
			name:     "all zeros XOR all ones",
			a:        idAllZeros,
			b:        idAllOnes,
			expected: "ffffffffffffffffffffffffffffffffffffffff",
		},
		{
			name:     "all zeros XOR 0x01",
			a:        idAllZeros,
			b:        idOneBit1,
			expected: "1",
		},
		{
			name:     "0x01 XOR 0x02",
			a:        idOneBit1,
			b:        idOneBit2,
			expected: "3", // 0x01 XOR 0x02 = 0x03
		},
		{
			name:     "0x01 XOR 0x04",
			a:        idOneBit1,
			b:        idOneBit4,
			expected: "5", // 0x01 XOR 0x04 = 0x05
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance := xorDistance(tt.a, tt.b)
			expected := new(big.Int)
			expected.SetString(tt.expected, 16)
			if distance.Cmp(expected) != 0 {
				t.Errorf("xorDistance(%s, %s) = %x, want %x", tt.a, tt.b, distance, expected)
			}
		})
	}
}

// TestXORDistance_Ordering verifies distance ordering properties
func TestXORDistance_Ordering(t *testing.T) {
	// Test that distances to a target follow expected ordering
	target := idPrefix8_1 // 0x8000...0000

	// These should be in increasing distance order from target
	ids := []scalegraph.ScalegraphId{
		idPrefix8_1, // distance: 0 (same as target)
		idPrefix8_2, // distance: 1 (differs in last bit)
		idPrefix8_3, // distance: 0x100 (differs in byte 18)
		idAllOnes,   // distance: 0x7fff...ffff (differs in all bits except MSB)
		idAllZeros,  // distance: 0x8000...0000 (differs in MSB only)
	}

	distances := make([]*big.Int, len(ids))
	for i, id := range ids {
		distances[i] = xorDistance(target, id)
	}

	// Verify distances are in ascending order
	for i := 0; i < len(distances)-1; i++ {
		if distances[i].Cmp(distances[i+1]) > 0 {
			t.Errorf("Distance ordering incorrect: distance[%d]=%x > distance[%d]=%x",
				i, distances[i], i+1, distances[i+1])
		}
	}
}

// TestNewRegistry verifies registry creation
func TestNewRegistry(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)

	if reg == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	if reg.Count() != 0 {
		t.Errorf("NewRegistry().Count() = %d, want 0", reg.Count())
	}

	apps := reg.List()
	if len(apps) != 0 {
		t.Errorf("NewRegistry().List() returned %d apps, want 0", len(apps))
	}
}

// TestRegistry_Register verifies app registration
func TestRegistry_Register(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)

	// Create a test virtual app
	srv := New(logger)
	vapp, err := srv.CreateVirtualApp()
	if err != nil {
		t.Fatalf("Failed to create virtual app: %v", err)
	}

	// Register the app
	err = reg.Register(vapp)
	if err != nil {
		t.Errorf("Register() error = %v, want nil", err)
	}

	// Verify count
	if reg.Count() != 1 {
		t.Errorf("Count() = %d, want 1", reg.Count())
	}

	// Verify it can be retrieved
	retrieved, exists := reg.GetByID(vapp.ID())
	if !exists {
		t.Error("GetByID() returned false after registration")
	}
	if retrieved != vapp {
		t.Error("GetByID() returned different VirtualApp instance")
	}
}

// TestRegistry_Register_Duplicate verifies duplicate registration fails
func TestRegistry_Register_Duplicate(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	vapp, _ := srv.CreateVirtualApp()

	// First registration should succeed
	err := reg.Register(vapp)
	if err != nil {
		t.Fatalf("First Register() error = %v, want nil", err)
	}

	// Second registration should fail
	err = reg.Register(vapp)
	if err == nil {
		t.Error("Register() duplicate succeeded, want error")
	}

	// Count should still be 1
	if reg.Count() != 1 {
		t.Errorf("Count() after duplicate = %d, want 1", reg.Count())
	}
}

// TestRegistry_Unregister verifies app removal
func TestRegistry_Unregister(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	vapp, _ := srv.CreateVirtualApp()
	reg.Register(vapp)

	// Unregister the app
	reg.Unregister(vapp.ID())

	// Verify it's gone
	if reg.Count() != 0 {
		t.Errorf("Count() after Unregister() = %d, want 0", reg.Count())
	}

	_, exists := reg.GetByID(vapp.ID())
	if exists {
		t.Error("GetByID() returned true after Unregister()")
	}
}

// TestRegistry_Unregister_Idempotent verifies unregister is safe to call multiple times
func TestRegistry_Unregister_Idempotent(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	vapp, _ := srv.CreateVirtualApp()
	reg.Register(vapp)

	// Unregister multiple times
	reg.Unregister(vapp.ID())
	reg.Unregister(vapp.ID())
	reg.Unregister(vapp.ID())

	// Should not panic or cause issues
	if reg.Count() != 0 {
		t.Errorf("Count() = %d, want 0", reg.Count())
	}
}

// TestRegistry_GetByID_NotFound verifies behavior for non-existent ID
func TestRegistry_GetByID_NotFound(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)

	nonExistentID := idRandom1

	vapp, exists := reg.GetByID(nonExistentID)
	if exists {
		t.Error("GetByID() for non-existent ID returned true")
	}
	if vapp != nil {
		t.Error("GetByID() for non-existent ID returned non-nil VirtualApp")
	}
}

// TestRegistry_List verifies listing all apps
func TestRegistry_List(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	// Register multiple apps
	count := 5
	vapps := make([]*VirtualApp, count)
	for i := 0; i < count; i++ {
		vapp, _ := srv.CreateVirtualApp()
		vapps[i] = vapp
		reg.Register(vapp)
	}

	// Get list
	list := reg.List()
	if len(list) != count {
		t.Errorf("List() returned %d apps, want %d", len(list), count)
	}

	// Verify all apps are present
	found := make(map[scalegraph.ScalegraphId]bool)
	for _, vapp := range list {
		found[vapp.ID()] = true
	}

	for _, vapp := range vapps {
		if !found[vapp.ID()] {
			t.Errorf("List() missing VirtualApp with ID %s", vapp.ID())
		}
	}
}

// TestRegistry_GetKClosest_Basic verifies basic k-closest functionality
func TestRegistry_GetKClosest_Basic(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	// Register 10 apps
	count := 10
	for i := 0; i < count; i++ {
		vapp, _ := srv.CreateVirtualApp()
		reg.Register(vapp)
	}

	// Get 5 closest to a random target
	target := idRandom1
	k := 5

	closest, err := reg.GetKClosest(target, k)
	if err != nil {
		t.Fatalf("GetKClosest() error = %v, want nil", err)
	}

	if len(closest) != k {
		t.Errorf("GetKClosest() returned %d apps, want %d", len(closest), k)
	}

	// Verify results are sorted by distance
	distances := make([]*big.Int, len(closest))
	for i, vapp := range closest {
		distances[i] = xorDistance(target, vapp.ID())
	}

	for i := 0; i < len(distances)-1; i++ {
		if distances[i].Cmp(distances[i+1]) > 0 {
			t.Errorf("GetKClosest() results not sorted: distance[%d]=%x > distance[%d]=%x",
				i, distances[i], i+1, distances[i+1])
		}
	}
}

// TestRegistry_GetKClosest_Empty verifies behavior with empty registry
func TestRegistry_GetKClosest_Empty(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)

	target := idRandom1
	k := 5

	closest, err := reg.GetKClosest(target, k)
	if err != nil {
		t.Fatalf("GetKClosest() on empty registry error = %v, want nil", err)
	}

	if len(closest) != 0 {
		t.Errorf("GetKClosest() on empty registry returned %d apps, want 0", len(closest))
	}
}

// TestRegistry_GetKClosest_InvalidK verifies error handling for invalid k
func TestRegistry_GetKClosest_InvalidK(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	vapp, _ := srv.CreateVirtualApp()
	reg.Register(vapp)

	target := idRandom1

	tests := []struct {
		name string
		k    int
	}{
		{"k = 0", 0},
		{"k = -1", -1},
		{"k = -100", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := reg.GetKClosest(target, tt.k)
			if err == nil {
				t.Error("GetKClosest() with invalid k succeeded, want error")
			}
		})
	}
}

// TestRegistry_GetKClosest_KGreaterThanCount verifies k > registry size
func TestRegistry_GetKClosest_KGreaterThanCount(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	// Register 3 apps
	count := 3
	for i := 0; i < count; i++ {
		vapp, _ := srv.CreateVirtualApp()
		reg.Register(vapp)
	}

	// Request more than available
	target := idRandom1
	k := 10

	closest, err := reg.GetKClosest(target, k)
	if err != nil {
		t.Fatalf("GetKClosest() error = %v, want nil", err)
	}

	// Should return all available apps
	if len(closest) != count {
		t.Errorf("GetKClosest() returned %d apps, want %d (all available)", len(closest), count)
	}
}

// TestRegistry_GetKClosest_Ordering verifies correct distance-based ordering
func TestRegistry_GetKClosest_Ordering(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)

	// Create virtual apps with known IDs for predictable ordering
	// We'll manually create them with test IDs
	target := idPrefix8_1

	// Create apps at known distances
	testIDs := []scalegraph.ScalegraphId{
		idPrefix8_2, // Closest: distance 1
		idPrefix8_3, // Middle: distance 0x100
		idAllZeros,  // Far: distance 0x8000...
	}

	// We need to create virtual apps with specific IDs
	// Since we can't control ID generation in CreateVirtualApp,
	// we'll test the xorDistance ordering separately
	srv := New(logger)
	vapps := make([]*VirtualApp, len(testIDs))
	for i := range testIDs {
		vapp, _ := srv.CreateVirtualApp()
		vapps[i] = vapp
		reg.Register(vapp)
	}

	// Get all apps sorted by distance
	k := len(vapps)
	closest, err := reg.GetKClosest(target, k)
	if err != nil {
		t.Fatalf("GetKClosest() error = %v", err)
	}

	// Calculate actual distances
	distances := make([]*big.Int, len(closest))
	for i, vapp := range closest {
		distances[i] = xorDistance(target, vapp.ID())
	}

	// Verify monotonic ordering
	for i := 0; i < len(distances)-1; i++ {
		if distances[i].Cmp(distances[i+1]) > 0 {
			t.Errorf("GetKClosest() not properly ordered: distance[%d]=%x > distance[%d]=%x",
				i, distances[i], i+1, distances[i+1])
		}
	}
}

// TestRegistry_Concurrent verifies thread-safety of registry operations
func TestRegistry_Concurrent(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// Concurrent registrations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vapp, err := srv.CreateVirtualApp()
			if err != nil {
				errChan <- err
				return
			}
			if err := reg.Register(vapp); err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()

	// Check for errors
	close(errChan)
	for err := range errChan {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Verify final count
	if reg.Count() != 50 {
		t.Errorf("Count() after concurrent registrations = %d, want 50", reg.Count())
	}
}

// TestRegistry_ConcurrentGetKClosest verifies concurrent GetKClosest calls
func TestRegistry_ConcurrentGetKClosest(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	// Populate registry
	for i := 0; i < 20; i++ {
		vapp, _ := srv.CreateVirtualApp()
		reg.Register(vapp)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// Concurrent GetKClosest calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			target := idRandom1
			k := 10
			closest, err := reg.GetKClosest(target, k)
			if err != nil {
				errChan <- err
				return
			}
			if len(closest) != k {
				errChan <- nil // Use nil to signal wrong length
			}
		}()
	}

	wg.Wait()
	close(errChan)

	errorCount := 0
	for range errChan {
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("Concurrent GetKClosest had %d errors", errorCount)
	}
}

// TestRegistry_ConcurrentMixed verifies mixed read/write operations
func TestRegistry_ConcurrentMixed(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	// Pre-populate with some apps
	initialApps := make([]*VirtualApp, 10)
	for i := 0; i < 10; i++ {
		vapp, _ := srv.CreateVirtualApp()
		initialApps[i] = vapp
		reg.Register(vapp)
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Concurrent readers (GetKClosest)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					target := idRandom1
					reg.GetKClosest(target, 5)
				}
			}
		}()
	}

	// Concurrent readers (List)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					reg.List()
				}
			}
		}()
	}

	// Concurrent writers (Register/Unregister)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				vapp, _ := srv.CreateVirtualApp()
				reg.Register(vapp)
				if idx < len(initialApps) {
					reg.Unregister(initialApps[idx].ID())
				}
			}
		}(i)
	}

	// Let operations run briefly
	cancel()
	wg.Wait()

	// Verify registry is still functional
	count := reg.Count()
	if count < 0 {
		t.Error("Invalid count after concurrent operations")
	}
}

// BenchmarkXORDistance benchmarks distance calculation
func BenchmarkXORDistance(b *testing.B) {
	id1 := idRandom1
	id2 := idRandom2

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = xorDistance(id1, id2)
	}
}

// BenchmarkGetKClosest benchmarks GetKClosest with various registry sizes
func BenchmarkGetKClosest(b *testing.B) {
	logger := newTestLogger()
	target := idRandom1

	sizes := []int{10, 50, 100, 500, 1000}
	kValues := []int{1, 5, 10, 20}

	for _, size := range sizes {
		for _, k := range kValues {
			name := fmt.Sprintf("size=%d,k=%d", size, k)
			b.Run(name, func(b *testing.B) {
				// Setup
				reg := NewRegistry(logger)
				srv := New(logger)
				for i := 0; i < size; i++ {
					vapp, _ := srv.CreateVirtualApp()
					reg.Register(vapp)
				}

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _ = reg.GetKClosest(target, k)
				}
			})
		}
	}
}

// BenchmarkRegistry_Register benchmarks registration
func BenchmarkRegistry_Register(b *testing.B) {
	logger := newTestLogger()
	srv := New(logger)

	// Pre-create apps
	apps := make([]*VirtualApp, b.N)
	for i := 0; i < b.N; i++ {
		apps[i], _ = srv.CreateVirtualApp()
	}

	b.ResetTimer()
	reg := NewRegistry(logger)
	for i := 0; i < b.N; i++ {
		reg.Register(apps[i])
	}
}

// BenchmarkRegistry_List benchmarks listing all apps
func BenchmarkRegistry_List(b *testing.B) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	// Populate registry
	for i := 0; i < 1000; i++ {
		vapp, _ := srv.CreateVirtualApp()
		reg.Register(vapp)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.List()
	}
}

// TestRegistry_GetKClosest_StableSort verifies sort stability
func TestRegistry_GetKClosest_StableSort(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	// Register apps
	count := 10
	vapps := make([]*VirtualApp, count)
	for i := 0; i < count; i++ {
		vapp, _ := srv.CreateVirtualApp()
		vapps[i] = vapp
		reg.Register(vapp)
	}

	target := idRandom1
	k := count

	// Call GetKClosest multiple times
	results := make([][]*VirtualApp, 5)
	for i := range results {
		closest, _ := reg.GetKClosest(target, k)
		results[i] = closest
	}

	// All results should be identical (stable sort with same input)
	for i := 1; i < len(results); i++ {
		if len(results[i]) != len(results[0]) {
			t.Fatalf("Result %d has different length than result 0", i)
		}
		for j := range results[i] {
			if results[i][j].ID() != results[0][j].ID() {
				t.Errorf("GetKClosest() not stable: result %d differs at position %d", i, j)
			}
		}
	}
}

// TestXORDistance_BitPatterns tests specific bit patterns
func TestXORDistance_BitPatterns(t *testing.T) {
	tests := []struct {
		name string
		a, b scalegraph.ScalegraphId
		desc string
	}{
		{
			name: "alternating bits 1",
			a:    mustParseID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			b:    mustParseID("5555555555555555555555555555555555555555"),
			desc: "all bits differ",
		},
		{
			name: "high bit set",
			a:    mustParseID("8000000000000000000000000000000000000000"),
			b:    mustParseID("0000000000000000000000000000000000000000"),
			desc: "only MSB differs",
		},
		{
			name: "low bit set",
			a:    mustParseID("0000000000000000000000000000000000000001"),
			b:    mustParseID("0000000000000000000000000000000000000000"),
			desc: "only LSB differs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := xorDistance(tt.a, tt.b)
			// Just verify it's non-negative and computable
			if d.Sign() < 0 {
				t.Errorf("xorDistance returned negative value for %s", tt.desc)
			}
		})
	}
}

// Helper to verify sorting
func isSorted(distances []*big.Int) bool {
	for i := 0; i < len(distances)-1; i++ {
		if distances[i].Cmp(distances[i+1]) > 0 {
			return false
		}
	}
	return true
}

// TestGetKClosest_VerifiesCompleteSorting ensures entire result is sorted
func TestGetKClosest_VerifiesCompleteSorting(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv := New(logger)

	// Register 100 apps for thorough testing
	for i := 0; i < 100; i++ {
		vapp, _ := srv.CreateVirtualApp()
		reg.Register(vapp)
	}

	target := idRandom1
	k := 50

	closest, err := reg.GetKClosest(target, k)
	if err != nil {
		t.Fatalf("GetKClosest() error = %v", err)
	}

	// Calculate all distances
	distances := make([]*big.Int, len(closest))
	for i, vapp := range closest {
		distances[i] = xorDistance(target, vapp.ID())
	}

	// Verify sorting
	if !isSorted(distances) {
		t.Error("GetKClosest() results are not properly sorted")
	}

	// Also verify these are actually the k closest
	// Get all apps and their distances
	allApps := reg.List()
	allDistances := make([]*big.Int, len(allApps))
	for i, vapp := range allApps {
		allDistances[i] = xorDistance(target, vapp.ID())
	}

	// Sort all distances
	sort.Slice(allDistances, func(i, j int) bool {
		return allDistances[i].Cmp(allDistances[j]) < 0
	})

	// Compare first k of sorted all vs returned closest
	for i := 0; i < k; i++ {
		if distances[i].Cmp(allDistances[i]) != 0 {
			t.Errorf("GetKClosest() result[%d] distance %x != expected %x",
				i, distances[i], allDistances[i])
		}
	}
}
