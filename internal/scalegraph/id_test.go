package scalegraph

import (
	"testing"
)

func TestNewScalegraphId(t *testing.T) {
	id, err := NewScalegraphId()
	if err != nil {
		t.Fatalf("NewScalegraphId() failed: %v", err)
	}

	// Test that ID is not zero value
	zeroID := ScalegraphId{}
	if id == zeroID {
		t.Error("generated ID is zero value")
	}

	// Test that two IDs are different
	id2, err := NewScalegraphId()
	if err != nil {
		t.Fatalf("NewScalegraphId() failed: %v", err)
	}
	if id == id2 {
		t.Error("two generated IDs are identical")
	}
}

func TestScalegraphIdString(t *testing.T) {
	id, _ := NewScalegraphId()
	str := id.String()

	// Test that string is hex encoded
	if len(str) != 40 { // 20 bytes * 2 hex chars
		t.Errorf("expected string length 40, got %d", len(str))
	}

	// Test that string contains only hex characters
	for _, c := range str {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("string contains non-hex character: %c", c)
		}
	}
}

func TestScalegraphIdFromString(t *testing.T) {
	// Create an ID and convert to string
	original, _ := NewScalegraphId()
	str := original.String()

	// Parse back from string
	parsed, err := ScalegraphIdFromString(str)
	if err != nil {
		t.Fatalf("ScalegraphIdFromString() failed: %v", err)
	}

	// Test that IDs are equal
	if parsed != original {
		t.Error("parsed ID does not match original")
	}

	// Test parsing invalid hex
	_, err = ScalegraphIdFromString("not-hex")
	if err == nil {
		t.Error("expected error for invalid hex, got nil")
	}

	// Test parsing wrong length
	_, err = ScalegraphIdFromString("abcd1234") // Too short
	if err == nil {
		t.Error("expected error for wrong length, got nil")
	}

	// Test empty string
	_, err = ScalegraphIdFromString("")
	if err == nil {
		t.Error("expected error for empty string, got nil")
	}
}

func TestScalegraphIdRoundTrip(t *testing.T) {
	// Test multiple round trips
	for i := 0; i < 10; i++ {
		original, _ := NewScalegraphId()
		str := original.String()
		parsed, err := ScalegraphIdFromString(str)
		if err != nil {
			t.Fatalf("round trip %d failed: %v", i, err)
		}
		if parsed != original {
			t.Errorf("round trip %d: IDs don't match", i)
		}
	}
}

func TestScalegraphIdConsistency(t *testing.T) {
	id, _ := NewScalegraphId()

	// Test that String() is consistent
	str1 := id.String()
	str2 := id.String()
	if str1 != str2 {
		t.Error("String() returned different values")
	}
}

func TestScalegraphIdSize(t *testing.T) {
	id := ScalegraphId{}

	// Test that size is exactly 20 bytes
	if len(id) != 20 {
		t.Errorf("expected size 20 bytes, got %d", len(id))
	}
}
