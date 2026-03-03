package scalegraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScalegraphId(t *testing.T) {
	id, err := NewScalegraphId()
	require.NoError(t, err)

	// Test that ID is not zero value
	zeroID := ScalegraphId{}
	assert.NotEqual(t, zeroID, id, "generated ID should not be zero value")

	// Test that two IDs are different
	id2, err := NewScalegraphId()
	require.NoError(t, err)
	assert.NotEqual(t, id, id2, "two generated IDs should be different")
}

func TestScalegraphIdString(t *testing.T) {
	id, _ := NewScalegraphId()
	str := id.String()

	// Test that string is hex encoded (20 bytes * 2 hex chars = 40)
	assert.Len(t, str, 40, "hex string should be 40 characters")

	// Test that string contains only hex characters
	for _, c := range str {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		assert.True(t, isHex, "string should only contain hex characters, got: %c", c)
	}
}

func TestScalegraphIdFromString(t *testing.T) {
	// Create an ID and convert to string
	original, _ := NewScalegraphId()
	str := original.String()

	// Parse back from string
	parsed, err := ScalegraphIdFromString(str)
	require.NoError(t, err)
	assert.Equal(t, original, parsed, "parsed ID should match original")

	// Test parsing invalid hex
	_, err = ScalegraphIdFromString("not-hex")
	assert.Error(t, err, "should error for invalid hex")

	// Test parsing wrong length
	_, err = ScalegraphIdFromString("abcd1234")
	assert.Error(t, err, "should error for wrong length")

	// Test empty string
	_, err = ScalegraphIdFromString("")
	assert.Error(t, err, "should error for empty string")
}

func TestScalegraphIdRoundTrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		original, _ := NewScalegraphId()
		str := original.String()
		parsed, err := ScalegraphIdFromString(str)
		require.NoError(t, err, "round trip %d failed", i)
		assert.Equal(t, original, parsed, "round trip %d: IDs should match", i)
	}
}

func TestScalegraphIdConsistency(t *testing.T) {
	id, _ := NewScalegraphId()

	str1 := id.String()
	str2 := id.String()
	assert.Equal(t, str1, str2, "String() should return consistent values")
}

func TestScalegraphIdSize(t *testing.T) {
	id := ScalegraphId{}
	assert.Len(t, id, 20, "ScalegraphId should be exactly 20 bytes")
}

func TestScalegraphIdFromPublicKey(t *testing.T) {
	pubKey, _, _ := testKeyPairAndCert(t)

	id := ScalegraphIdFromPublicKey(pubKey)

	// Should not be zero
	zeroID := ScalegraphId{}
	assert.NotEqual(t, zeroID, id, "ID from public key should not be zero")

	// Should be deterministic
	id2 := ScalegraphIdFromPublicKey(pubKey)
	assert.Equal(t, id, id2, "same public key should produce same ID")

	// Different keys should produce different IDs
	pubKey2, _, _ := testKeyPairAndCert(t)
	id3 := ScalegraphIdFromPublicKey(pubKey2)
	assert.NotEqual(t, id, id3, "different public keys should produce different IDs")
}
