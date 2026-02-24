package crypto

import "errors"

// Signature verification errors
var (
	// ErrNoVerifier indicates the server was not configured with a CA for signature verification
	ErrNoVerifier = errors.New("server not configured with certificate authority for signature verification")
	// ErrMissingSignature indicates a required signature was not provided
	ErrMissingSignature = errors.New("signature required but not provided")
	// ErrInvalidSignature indicates the signature verification failed
	ErrInvalidSignature = errors.New("signature verification failed")
	// ErrSignerMismatch indicates the signer ID does not match the expected account
	ErrSignerMismatch = errors.New("signer ID does not match expected account")
	// ErrPayloadMismatch indicates the payload data does not match the signed data
	ErrPayloadMismatch = errors.New("payload data does not match signed data")
)
