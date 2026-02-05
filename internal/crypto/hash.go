package crypto

import (
	"crypto/ed25519"
	"crypto/sha1"
)

// DeriveAccountID derives a 160-bit account ID from an Ed25519 public key
// using SHA-1 hash. This produces a 20-byte output that matches the
// ScalegraphId size.
func DeriveAccountID(pubKey ed25519.PublicKey) [20]byte {
	hash := sha1.Sum(pubKey)
	return hash
}
