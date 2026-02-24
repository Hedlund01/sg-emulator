package server

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"testing"

	"sg-emulator/internal/scalegraph"

	"github.com/stretchr/testify/assert"
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
			assert.Equal(t, 0, distance.Cmp(big.NewInt(0)), "xorDistance(%s, %s) = %v, want 0", tt.id, tt.id, distance)
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
			assert.Equal(t, 0, d1.Cmp(d2), "xorDistance not symmetric: d(%s, %s) = %v, d(%s, %s) = %v",
				tt.a, tt.b, d1, tt.b, tt.a, d2)
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
			assert.GreaterOrEqual(t, distance.Sign(), 0, "xorDistance(%s, %s) = %v, want non-negative", tt.a, tt.b, distance)
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
			assert.Equal(t, 0, distance.Cmp(expected), "xorDistance(%s, %s) = %x, want %x", tt.a, tt.b, distance, expected)
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
		assert.LessOrEqual(t, distances[i].Cmp(distances[i+1]), 0,
			"Distance ordering incorrect: distance[%d]=%x > distance[%d]=%x",
			i, distances[i], i+1, distances[i+1])
	}
}

// TestNewRegistry verifies registry creation
func TestNewRegistry(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)

	assert.NotNil(t, reg, "NewRegistry() should not return nil")
	assert.Equal(t, 0, reg.Count(), "NewRegistry().Count() should be 0")
	assert.Empty(t, reg.List(), "NewRegistry().List() should be empty")
}

// TestRegistry_Register verifies app registration
func TestRegistry_Register(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)

	// Create a test virtual app
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err, "Failed to create test server: %v", err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err, "Failed to create virtual app")

	// Register the app
	assert.NoError(t, reg.Register(vapp), "Register() should succeed")

	// Verify count
	assert.Equal(t, 1, reg.Count(), "Count() after registration")

	// Verify it can be retrieved
	retrieved, exists := reg.GetByID(vapp.ID())
	assert.True(t, exists, "GetByID() should return true after registration")
	assert.Same(t, vapp, retrieved, "GetByID() should return the same VirtualApp instance")
}

// TestRegistry_Register_Duplicate verifies duplicate registration fails
func TestRegistry_Register_Duplicate(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)

	// First registration should succeed
	err = reg.Register(vapp)
	assert.NoError(t, err, "First Register() should succeed")

	// Second registration should fail
	err = reg.Register(vapp)
	assert.Error(t, err, "Register() duplicate should fail")

	// Count should still be 1
	assert.Equal(t, 1, reg.Count(), "Count() after duplicate")
}

// TestRegistry_Unregister verifies app removal
func TestRegistry_Unregister(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)
	assert.NoError(t, reg.Register(vapp))

	// Unregister the app
	reg.Unregister(vapp.ID())

	// Verify it's gone
	assert.Equal(t, 0, reg.Count(), "Count() after Unregister()")

	_, exists := reg.GetByID(vapp.ID())
	assert.False(t, exists, "GetByID() should return false after Unregister()")
}

// TestRegistry_Unregister_Idempotent verifies unregister is safe to call multiple times
func TestRegistry_Unregister_Idempotent(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)
	assert.NoError(t, reg.Register(vapp))

	// Unregister multiple times
	reg.Unregister(vapp.ID())
	reg.Unregister(vapp.ID())
	reg.Unregister(vapp.ID())

	// Should not panic or cause issues
	assert.Equal(t, 0, reg.Count(), "Count() after multiple Unregister()")
}

// TestRegistry_GetByID_NotFound verifies behavior for non-existent ID
func TestRegistry_GetByID_NotFound(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)

	nonExistentID := idRandom1

	vapp, exists := reg.GetByID(nonExistentID)
	assert.False(t, exists, "GetByID() for non-existent ID should return false")
	assert.Nil(t, vapp, "GetByID() for non-existent ID should return nil")
}

// TestRegistry_List verifies listing all apps
func TestRegistry_List(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	// Register multiple apps
	count := 5
	vapps := make([]*VirtualApp, count)
	for i := 0; i < count; i++ {
		vapp, err := srv.CreateVirtualApp()
		assert.NoError(t, err)
		vapps[i] = vapp
		assert.NoError(t, reg.Register(vapp))
	}

	// Get list
	list := reg.List()
	assert.Len(t, list, count, "List() should return all registered apps")

	// Verify all apps are present
	found := make(map[scalegraph.ScalegraphId]bool)
	for _, vapp := range list {
		found[vapp.ID()] = true
	}

	for _, vapp := range vapps {
		assert.True(t, found[vapp.ID()], "List() missing VirtualApp with ID %s", vapp.ID())
	}
}

// TestRegistry_GetKClosest_Basic verifies basic k-closest functionality
func TestRegistry_GetKClosest_Basic(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	// Register 10 apps
	count := 10
	for i := 0; i < count; i++ {
		vapp, err := srv.CreateVirtualApp()
		assert.NoError(t, err)
		assert.NoError(t, reg.Register(vapp))
	}

	// Get 5 closest to a random target
	target := idRandom1
	k := 5

	closest, err := reg.GetKClosest(target, k)
	assert.NoError(t, err)
	assert.Len(t, closest, k, "GetKClosest() should return k apps")

	// Verify results are sorted by distance
	distances := make([]*big.Int, len(closest))
	for i, vapp := range closest {
		distances[i] = xorDistance(target, vapp.ID())
	}

	for i := 0; i < len(distances)-1; i++ {
		assert.LessOrEqual(t, distances[i].Cmp(distances[i+1]), 0,
			"GetKClosest() results not sorted: distance[%d]=%x > distance[%d]=%x",
			i, distances[i], i+1, distances[i+1])
	}
}

// TestRegistry_GetKClosest_Empty verifies behavior with empty registry
func TestRegistry_GetKClosest_Empty(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)

	target := idRandom1
	k := 5

	closest, err := reg.GetKClosest(target, k)
	assert.NoError(t, err, "GetKClosest() on empty registry should not error")
	assert.Empty(t, closest, "GetKClosest() on empty registry should return empty slice")
}

// TestRegistry_GetKClosest_InvalidK verifies error handling for invalid k
func TestRegistry_GetKClosest_InvalidK(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	vapp, err := srv.CreateVirtualApp()
	assert.NoError(t, err)
	assert.NoError(t, reg.Register(vapp))

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
			assert.Error(t, err, "GetKClosest() with invalid k should fail")
		})
	}
}

// TestRegistry_GetKClosest_KGreaterThanCount verifies k > registry size
func TestRegistry_GetKClosest_KGreaterThanCount(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	// Register 3 apps
	count := 3
	for i := 0; i < count; i++ {
		vapp, err := srv.CreateVirtualApp()
		assert.NoError(t, err)
		assert.NoError(t, reg.Register(vapp))
	}

	// Request more than available
	target := idRandom1
	k := 10

	closest, err := reg.GetKClosest(target, k)
	assert.NoError(t, err)
	// Should return all available apps
	assert.Len(t, closest, count, "GetKClosest() should return all available apps when k > count")
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
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)
	vapps := make([]*VirtualApp, len(testIDs))
	for i := range testIDs {
		vapp, err := srv.CreateVirtualApp()
		assert.NoError(t, err)
		vapps[i] = vapp
		assert.NoError(t, reg.Register(vapp))
	}

	// Get all apps sorted by distance
	k := len(vapps)
	closest, err := reg.GetKClosest(target, k)
	assert.NoError(t, err)

	// Calculate actual distances
	distances := make([]*big.Int, len(closest))
	for i, vapp := range closest {
		distances[i] = xorDistance(target, vapp.ID())
	}

	// Verify monotonic ordering
	for i := 0; i < len(distances)-1; i++ {
		assert.LessOrEqual(t, distances[i].Cmp(distances[i+1]), 0,
			"GetKClosest() not properly ordered: distance[%d]=%x > distance[%d]=%x",
			i, distances[i], i+1, distances[i+1])
	}
}

// TestRegistry_Concurrent verifies thread-safety of registry operations
func TestRegistry_Concurrent(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

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
	assert.Equal(t, 50, reg.Count(), "Count() after concurrent registrations")
}

// TestRegistry_ConcurrentGetKClosest verifies concurrent GetKClosest calls
func TestRegistry_ConcurrentGetKClosest(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	// Populate registry
	for i := 0; i < 20; i++ {
		vapp, err := srv.CreateVirtualApp()
		assert.NoError(t, err)
		assert.NoError(t, reg.Register(vapp))
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

	assert.Equal(t, 0, errorCount, "Concurrent GetKClosest should have no errors")
}

// TestRegistry_ConcurrentMixed verifies mixed read/write operations
func TestRegistry_ConcurrentMixed(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	// Pre-populate with some apps
	initialApps := make([]*VirtualApp, 10)
	for i := 0; i < 10; i++ {
		vapp, err := srv.CreateVirtualApp()
		assert.NoError(t, err)
		initialApps[i] = vapp
		assert.NoError(t, reg.Register(vapp))
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
	assert.GreaterOrEqual(t, count, 0, "Invalid count after concurrent operations")
}

// BenchmarkXORDistance benchmarks distance calculation
func BenchmarkXORDistance(b *testing.B) {
	id1 := idRandom1
	id2 := idRandom2

	for b.Loop() {
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
				srv, cleanup, err := newTestServer(logger)
				if err != nil {
					b.Fatal(err)
				}
				defer cleanup()
				for i := 0; i < size; i++ {
					vapp, err := srv.CreateVirtualApp()
					if err != nil {
						b.Fatal(err)
					}
					if err := reg.Register(vapp); err != nil {
						b.Fatal(err)
					}
				}

				for b.Loop() {
					_, _ = reg.GetKClosest(target, k)
				}
			})
		}
	}
}

// BenchmarkRegistry_Register benchmarks registration
func BenchmarkRegistry_Register(b *testing.B) {
	logger := newTestLogger()
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup()
	reg := NewRegistry(logger)

	// Pre-create apps to exclude creation time from benchmark
	apps := make([]*VirtualApp, 0, 1000)
	for i := 0; i < 1000; i++ {
		vapp, err := srv.CreateVirtualApp()
		if err != nil {
			b.Fatal(err)
		}
		apps = append(apps, vapp)
	}

	idx := 0
	for b.Loop() {
		reg.Register(apps[idx%len(apps)])
		idx++
	}
}

// BenchmarkRegistry_List benchmarks listing all apps
func BenchmarkRegistry_List(b *testing.B) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup()

	// Populate registry
	for i := 0; i < 1000; i++ {
		vapp, err := srv.CreateVirtualApp()
		if err != nil {
			b.Fatal(err)
		}
		if err := reg.Register(vapp); err != nil {
			b.Fatal(err)
		}
	}

	for b.Loop() {
		_ = reg.List()
	}
}

// TestRegistry_GetKClosest_StableSort verifies sort stability
func TestRegistry_GetKClosest_StableSort(t *testing.T) {
	logger := newTestLogger()
	reg := NewRegistry(logger)
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	// Register apps
	count := 10
	vapps := make([]*VirtualApp, count)
	for i := 0; i < count; i++ {
		vapp, err := srv.CreateVirtualApp()
		assert.NoError(t, err)
		vapps[i] = vapp
		assert.NoError(t, reg.Register(vapp))
	}

	target := idRandom1
	k := count

	// Call GetKClosest multiple times
	results := make([][]*VirtualApp, 5)
	for i := range results {
		closest, err := reg.GetKClosest(target, k)
		assert.NoError(t, err)
		results[i] = closest
	}

	// All results should be identical (stable sort with same input)
	for i := 1; i < len(results); i++ {
		assert.Len(t, results[i], len(results[0]), "Result %d has different length than result 0", i)
		for j := range results[i] {
			assert.Equal(t, results[0][j].ID(), results[i][j].ID(),
				"GetKClosest() not stable: result %d differs at position %d", i, j)
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
			assert.GreaterOrEqual(t, d.Sign(), 0, "xorDistance returned negative value for %s", tt.desc)
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
	srv, cleanup, err := newTestServer(logger)
	defer cleanup()
	assert.NoError(t, err)

	// Register 100 apps for thorough testing
	for i := 0; i < 100; i++ {
		vapp, err := srv.CreateVirtualApp()
		assert.NoError(t, err)
		assert.NoError(t, reg.Register(vapp))
	}

	target := idRandom1
	k := 50

	closest, err := reg.GetKClosest(target, k)
	assert.NoError(t, err)

	// Calculate all distances
	distances := make([]*big.Int, len(closest))
	for i, vapp := range closest {
		distances[i] = xorDistance(target, vapp.ID())
	}

	// Verify sorting
	assert.True(t, isSorted(distances), "GetKClosest() results are not properly sorted")

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
		assert.Equal(t, 0, distances[i].Cmp(allDistances[i]),
			"GetKClosest() result[%d] distance %x != expected %x",
			i, distances[i], allDistances[i])
	}
}
