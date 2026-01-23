package trace

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := "test-trace-123"

	newCtx := WithTraceID(ctx, traceID)

	got := GetTraceID(newCtx)
	if got != traceID {
		t.Errorf("WithTraceID() = %v, want %v", got, traceID)
	}
}

func TestWithTraceID_EmptyString(t *testing.T) {
	ctx := context.Background()
	traceID := ""

	newCtx := WithTraceID(ctx, traceID)

	got := GetTraceID(newCtx)
	if got != traceID {
		t.Errorf("WithTraceID() with empty string = %v, want %v", got, traceID)
	}
}

func TestNewTraceID(t *testing.T) {
	ctx := context.Background()

	newCtx := NewTraceID(ctx)

	got := GetTraceID(newCtx)
	if got == "" {
		t.Error("NewTraceID() returned empty trace ID")
	}

	// Verify it's a valid UUID
	_, err := uuid.Parse(got)
	if err != nil {
		t.Errorf("NewTraceID() did not generate valid UUID: %v", err)
	}
}

func TestNewTraceID_Uniqueness(t *testing.T) {
	ctx := context.Background()
	ids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		newCtx := NewTraceID(ctx)
		id := GetTraceID(newCtx)
		if ids[id] {
			t.Errorf("NewTraceID() generated duplicate ID: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("Expected %d unique IDs, got %d", count, len(ids))
	}
}

func TestGetTraceID_NotFound(t *testing.T) {
	ctx := context.Background()

	got := GetTraceID(ctx)
	if got != "" {
		t.Errorf("GetTraceID() on context without trace ID = %v, want empty string", got)
	}
}

func TestGetTraceID_WrongType(t *testing.T) {
	ctx := context.Background()
	// Store a non-string value with the trace ID key
	ctx = context.WithValue(ctx, traceIDKey, 12345)

	got := GetTraceID(ctx)
	if got != "" {
		t.Errorf("GetTraceID() with wrong type = %v, want empty string", got)
	}
}

func TestTraceID_ContextChain(t *testing.T) {
	// Test that trace IDs propagate correctly through context chains
	ctx := context.Background()
	traceID1 := "trace-1"
	traceID2 := "trace-2"

	ctx1 := WithTraceID(ctx, traceID1)
	ctx2 := WithTraceID(ctx1, traceID2)

	// ctx2 should have traceID2 (overwrites)
	got2 := GetTraceID(ctx2)
	if got2 != traceID2 {
		t.Errorf("GetTraceID(ctx2) = %v, want %v", got2, traceID2)
	}

	// ctx1 should still have traceID1
	got1 := GetTraceID(ctx1)
	if got1 != traceID1 {
		t.Errorf("GetTraceID(ctx1) = %v, want %v", got1, traceID1)
	}

	// Original ctx should have no trace ID
	got := GetTraceID(ctx)
	if got != "" {
		t.Errorf("GetTraceID(ctx) = %v, want empty string", got)
	}
}

func TestTraceID_Concurrent(t *testing.T) {
	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Test concurrent WithTraceID calls
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			traceID := uuid.New().String()
			newCtx := WithTraceID(ctx, traceID)
			got := GetTraceID(newCtx)
			if got != traceID {
				errors <- nil
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	if len(errors) > 0 {
		t.Error("Concurrent WithTraceID operations failed")
	}
}

func TestNewTraceID_Concurrent(t *testing.T) {
	ctx := context.Background()
	var wg sync.WaitGroup
	ids := make(chan string, 100)

	// Test concurrent NewTraceID calls
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newCtx := NewTraceID(ctx)
			id := GetTraceID(newCtx)
			ids <- id
		}()
	}

	wg.Wait()
	close(ids)

	// Check all IDs are unique
	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Errorf("Duplicate trace ID generated: %s", id)
		}
		seen[id] = true
	}

	if len(seen) != 100 {
		t.Errorf("Expected 100 unique trace IDs, got %d", len(seen))
	}
}

func TestTraceID_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	traceID := "test-trace"

	newCtx := WithTraceID(ctx, traceID)
	cancel()

	// Trace ID should still be accessible even after cancellation
	got := GetTraceID(newCtx)
	if got != traceID {
		t.Errorf("GetTraceID() after cancel = %v, want %v", got, traceID)
	}
}

func TestTraceID_MultipleGoroutinesSharedContext(t *testing.T) {
	// Test that multiple goroutines can safely read from the same context
	ctx := context.Background()
	traceID := "shared-trace"
	ctx = WithTraceID(ctx, traceID)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				got := GetTraceID(ctx)
				if got != traceID {
					t.Errorf("GetTraceID() in goroutine = %v, want %v", got, traceID)
					return
				}
			}
		}()
	}

	wg.Wait()
}

// Benchmark tests
func BenchmarkWithTraceID(b *testing.B) {
	ctx := context.Background()
	traceID := "bench-trace-id"

	for b.Loop() {
		_ = WithTraceID(ctx, traceID)
	}
}

func BenchmarkNewTraceID(b *testing.B) {
	ctx := context.Background()

	for b.Loop() {
		_ = NewTraceID(ctx)
	}
}

func BenchmarkGetTraceID(b *testing.B) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "bench-trace")

	for b.Loop() {
		_ = GetTraceID(ctx)
	}
}

func BenchmarkGetTraceID_NotFound(b *testing.B) {
	ctx := context.Background()

	for b.Loop() {
		_ = GetTraceID(ctx)
	}
}
