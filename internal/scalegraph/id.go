package scalegraph

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"sg-emulator/internal/crypto"
)

// ScalegraphId represents a unique 160-bit (20-byte) identifier
type ScalegraphId [20]byte

// NewScalegraphId generates a new random 160-bit ScalegraphId
func NewScalegraphId() (ScalegraphId, error) {
	var id ScalegraphId
	_, err := rand.Read(id[:])
	if err != nil {
		return ScalegraphId{}, fmt.Errorf("failed to generate ScalegraphId: %w", err)
	}
	return id, nil
}

// ScalegraphIdFromString parses a hexadecimal string into a ScalegraphId
func ScalegraphIdFromString(s string) (ScalegraphId, error) {
	var id ScalegraphId
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return ScalegraphId{}, fmt.Errorf("invalid hex string: %w", err)
	}
	if len(bytes) != 20 {
		return ScalegraphId{}, fmt.Errorf("invalid ScalegraphId length: expected 20 bytes, got %d", len(bytes))
	}
	copy(id[:], bytes)
	return id, nil
}

// String returns the hexadecimal string representation of the ScalegraphId
func (id ScalegraphId) String() string {
	return hex.EncodeToString(id[:])
}

// ScalegraphIdFromPublicKey derives a ScalegraphId from an Ed25519 public key
// using SHA-1 hash of the public key bytes
func ScalegraphIdFromPublicKey(pubKey ed25519.PublicKey) ScalegraphId {
	hash := crypto.DeriveAccountID(pubKey)
	return ScalegraphId(hash)
}
