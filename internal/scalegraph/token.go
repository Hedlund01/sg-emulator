package scalegraph

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sg-emulator/internal/crypto"
)

type IToken interface {
	ID() string
	Value() string
	Signature() crypto.Signature
	ClawbackAddress() *ScalegraphId
	Nonce() int64
	Equal(other IToken) bool
	String() string
}

// TokenEqual is a generic equality function that works for any IToken implementation.
// Token types can use this in their Equal() method or provide their own implementation.
func TokenEqual(t1, t2 IToken) bool {
	if t1 == nil || t2 == nil {
		return false
	}

	// Compare signatures using bytes.Equal for the value
	sig1 := t1.Signature()
	sig2 := t2.Signature()
	signaturesEqual := sig1.Algorithm == sig2.Algorithm &&
		bytes.Equal(sig1.Value, sig2.Value) &&
		sig1.SignerID == sig2.SignerID &&
		sig1.Timestamp == sig2.Timestamp

	// Compare clawback addresses
	cb1 := t1.ClawbackAddress()
	cb2 := t2.ClawbackAddress()
	clawbackEqual := (cb1 == nil && cb2 == nil) ||
		(cb1 != nil && cb2 != nil && *cb1 == *cb2)

	return t1.ID() == t2.ID() && t1.Value() == t2.Value() && signaturesEqual && clawbackEqual
}

// TokenString is a generic string representation function that works for any IToken implementation.
// Token types can use this in their String() method or provide their own implementation.
func TokenString(t IToken) string {
	if t == nil {
		return "Token{nil}"
	}
	return fmt.Sprintf("Token{ID: %s, Value: %s, Nonce: %d, ClawbackAddress: %v, Signature: %s}",
		t.ID(), t.Value(), t.Nonce(), t.ClawbackAddress(), t.Signature())
}

type Token struct {
	value           string
	signature       crypto.Signature
	clawbackAddress *ScalegraphId
	nonce 		 int64
}

func newToken(value string, signature crypto.Signature, clawbackAddress *ScalegraphId, nonce int64) *Token {
	return &Token{
		value:           value,
		signature:       signature,
		clawbackAddress: clawbackAddress,
		nonce:           nonce,
	}
}

func (t *Token) ID() string {
	return hex.EncodeToString(t.signature.Value) // Using the hex-encoded signature bytes as the unique ID
}

func (t *Token) Value() string {
	return t.value
}

func (t *Token) Signature() crypto.Signature {
	return t.signature
}

func (t *Token) ClawbackAddress() *ScalegraphId {
	return t.clawbackAddress
}

// Equal uses the generic TokenEqual function.
// Can be overridden by specific token types if custom logic is needed.
func (t *Token) Equal(other IToken) bool {
	return TokenEqual(t, other)
}

// String uses the generic TokenString function.
// Can be overridden by specific token types if custom formatting is needed.
func (t *Token) String() string {
	return TokenString(t)
}

func (t *Token) Nonce() int64 {
	return t.nonce
}
