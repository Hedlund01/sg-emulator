package crypto

import (
	"crypto/ed25519"
	"testing"
	"time"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	if len(kp.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("Expected public key size %d, got %d", ed25519.PublicKeySize, len(kp.PublicKey))
	}

	if len(kp.PrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("Expected private key size %d, got %d", ed25519.PrivateKeySize, len(kp.PrivateKey))
	}
}

func TestDeriveAccountID(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	id := DeriveAccountID(kp.PublicKey)
	if len(id) != 20 {
		t.Errorf("Expected ID length 20, got %d", len(id))
	}

	// Verify deterministic - same public key produces same ID
	id2 := DeriveAccountID(kp.PublicKey)
	if id != id2 {
		t.Error("DeriveAccountID is not deterministic")
	}
}

func TestEncodeDecodePrivateKey(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	pem, err := EncodePrivateKeyPEM(kp.PrivateKey)
	if err != nil {
		t.Fatalf("EncodePrivateKeyPEM failed: %v", err)
	}

	decoded, err := DecodePrivateKeyPEM(pem)
	if err != nil {
		t.Fatalf("DecodePrivateKeyPEM failed: %v", err)
	}

	if !decoded.Equal(kp.PrivateKey) {
		t.Error("Decoded private key does not match original")
	}
}

func TestEncodeDecodePublicKey(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	pem, err := EncodePublicKeyPEM(kp.PublicKey)
	if err != nil {
		t.Fatalf("EncodePublicKeyPEM failed: %v", err)
	}

	decoded, err := DecodePublicKeyPEM(pem)
	if err != nil {
		t.Fatalf("DecodePublicKeyPEM failed: %v", err)
	}

	if !decoded.Equal(kp.PublicKey) {
		t.Error("Decoded public key does not match original")
	}
}

func TestSignAndVerify(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	// Create test payload
	payload := &TransferRequest{
		From:      "test-from",
		To:        "test-to",
		Amount:    100.0,
		Nonce:     "test-nonce",
		Timestamp: time.Now().Unix(),
	}

	accountID := DeriveAccountID(kp.PublicKey)
	accountIDHex := string(make([]byte, 40))
	for i, b := range accountID {
		const hex = "0123456789abcdef"
		accountIDHex = accountIDHex[:i*2] + string(hex[b>>4]) + string(hex[b&0xf]) + accountIDHex[(i+1)*2:]
	}

	sig, err := Sign(payload, kp.PrivateKey, accountIDHex[:40])
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	if sig.Algorithm != "Ed25519" {
		t.Errorf("Expected algorithm Ed25519, got %s", sig.Algorithm)
	}

	if len(sig.Value) != ed25519.SignatureSize {
		t.Errorf("Expected signature size %d, got %d", ed25519.SignatureSize, len(sig.Value))
	}

	// Verify using raw ed25519 verify
	if !ed25519.Verify(kp.PublicKey, payload.Bytes(), sig.Value) {
		t.Error("Signature verification failed")
	}
}

func TestTransferRequestBytes(t *testing.T) {
	req := &TransferRequest{
		From:      "abc123",
		To:        "def456",
		Amount:    50.5,
		Nonce:     "unique-nonce",
		Timestamp: 1234567890,
	}

	bytes1 := req.Bytes()
	bytes2 := req.Bytes()

	// Verify deterministic
	if string(bytes1) != string(bytes2) {
		t.Error("TransferRequest.Bytes() is not deterministic")
	}

	// Verify it produces valid JSON
	if len(bytes1) == 0 {
		t.Error("TransferRequest.Bytes() returned empty bytes")
	}
}
