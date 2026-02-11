package crypto

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"time"
)

// SignableData represents data that can be signed
type SignableData interface {
	Bytes() []byte // Canonical byte representation for signing
}

// Signature represents a cryptographic signature
type Signature struct {
	Algorithm string `json:"algorithm"` // "Ed25519"
	Value     []byte `json:"value"`     // Raw signature bytes (64 bytes for Ed25519)
	SignerID  string `json:"signer_id"` // Account ID (hash of public key)
	Timestamp int64  `json:"timestamp"` // Unix timestamp when signed
}

// SignedEnvelope wraps a payload with its signature and certificate
type SignedEnvelope[T SignableData] struct {
	Payload     T         `json:"payload"`
	Signature   Signature `json:"signature"`
	Certificate string    `json:"certificate"` // PEM-encoded X.509 certificate
}

// Sign creates a signature for the given data using the private key
func Sign(data SignableData, privKey ed25519.PrivateKey, signerID string) (*Signature, error) {
	if len(privKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: got %d, want %d", len(privKey), ed25519.PrivateKeySize)
	}

	timestamp := time.Now().Unix()
	bytesToSign := data.Bytes()
	signature := ed25519.Sign(privKey, bytesToSign)

	return &Signature{
		Algorithm: "Ed25519",
		Value:     signature,
		SignerID:  signerID,
		Timestamp: timestamp,
	}, nil
}

// SignWithTimestamp creates a signature with a specific timestamp (useful for testing)
func SignWithTimestamp(data SignableData, privKey ed25519.PrivateKey, signerID string, timestamp int64) (*Signature, error) {
	if len(privKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: got %d, want %d", len(privKey), ed25519.PrivateKeySize)
	}

	bytesToSign := data.Bytes()
	signature := ed25519.Sign(privKey, bytesToSign)

	return &Signature{
		Algorithm: "Ed25519",
		Value:     signature,
		SignerID:  signerID,
		Timestamp: timestamp,
	}, nil
}

// CreateSignedEnvelope creates a signed envelope for the given payload
func CreateSignedEnvelope[T SignableData](payload T, privKey ed25519.PrivateKey, signerID string, certPEM string) (*SignedEnvelope[T], error) {
	sig, err := Sign(payload, privKey, signerID)
	if err != nil {
		return nil, fmt.Errorf("failed to sign payload: %w", err)
	}

	return &SignedEnvelope[T]{
		Payload:     payload,
		Signature:   *sig,
		Certificate: certPEM,
	}, nil
}

// TransferRequest represents a signed transfer request
type TransferRequest struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	Amount    float64 `json:"amount"`
	Nonce     uint64  `json:"nonce"`
	Timestamp int64   `json:"timestamp"`
}

// Bytes returns the canonical byte representation for signing
func (r *TransferRequest) Bytes() []byte {
	// Create a deterministic byte representation
	data, _ := json.Marshal(TransferRequest{
		From:      r.From,
		To:        r.To,
		Amount:    r.Amount,
		Nonce:     r.Nonce,
		Timestamp: r.Timestamp,
	})
	return data
}

type CreateAccountRequest struct {
	InitialBalance float64 `json:"initial_balance"`
}

// Bytes returns the canonical byte representation for signing
func (r *CreateAccountRequest) Bytes() []byte {
	data, _ := json.Marshal(CreateAccountRequest{
		InitialBalance: r.InitialBalance,
	})
	return data
}

type GetAccountRequest struct {
	AccountID string `json:"account_id"`
}

// Bytes returns the canonical byte representation for signing
func (r *GetAccountRequest) Bytes() []byte {
	data, _ := json.Marshal(GetAccountRequest{
		AccountID: r.AccountID,
	})
	return data
}
