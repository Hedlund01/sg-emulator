package scalegraph

import "fmt"

type IToken interface {
	ID() ScalegraphId
	Value() string
	Signature() []byte
	ClawbackAddress() *ScalegraphId
}

type Token struct {
	id              ScalegraphId
	value           string
	signature       []byte
	clawbackAddress *ScalegraphId
}

func newToken(value string, signature []byte, clawbackAddress *ScalegraphId) *Token {
	id, _ := NewScalegraphId()
	return &Token{
		id:              id,
		value:           value,
		signature:       signature,
		clawbackAddress: clawbackAddress,
	}
}

func (t *Token) ID() ScalegraphId {
	return t.id
}

func (t *Token) Value() string {
	return t.value
}

func (t *Token) Signature() []byte {
	return t.signature
}

func (t *Token) ClawbackAddress() *ScalegraphId {
	return t.clawbackAddress
}

func (t *Token) Equal(other *Token) bool {
	if t == nil || other == nil {
		return false
	}
	return t.id == other.id &&
		t.value == other.value &&
		string(t.signature) == string(other.signature) &&
		((t.clawbackAddress == nil && other.clawbackAddress == nil) ||
			(t.clawbackAddress != nil && other.clawbackAddress != nil && *t.clawbackAddress == *other.clawbackAddress))
}

func (t *Token) String() string {
	return fmt.Sprintf("Token{ID: %s, Value: %s, ClawbackAddress: %v}", t.id, t.value, t.clawbackAddress)
}
