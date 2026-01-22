package scalegraph

import "fmt"

// Transaction is immutable after creation - no mutex needed
type Transaction struct {
	id                 ScalegraphId
	sendingAccountID   *Account
	receivingAccountID *Account
	amount             float64
	value              string
}

func newTransaction(senderID, receiverID *Account, amount float64, value string) (*Transaction, error) {
	if value != "" && amount != 0 {
		return nil, fmt.Errorf("transaction cannot have both amount and value")
	}

	id, _ := NewScalegraphId()
	return &Transaction{
		id:                 id,
		sendingAccountID:   senderID,
		receivingAccountID: receiverID,
		amount:             amount,
		value:              value,
	}, nil
}

func (t *Transaction) String() string {
	fromStr := "nil"
	if t.sendingAccountID != nil {
		fromStr = t.sendingAccountID.ID().String()[:8] + "..."
	}
	toStr := "nil"
	if t.receivingAccountID != nil {
		toStr = t.receivingAccountID.ID().String()[:8] + "..."
	}
	return fmt.Sprintf("Transaction(ID: %s, From: %s, To: %s, Amount: %.2f, Value: %s)", t.id, fromStr, toStr, t.amount, t.value)
}

// ID returns the transaction's unique identifier
func (t *Transaction) ID() ScalegraphId {
	return t.id
}

// Sender returns the sending account (nil for mint transactions)
func (t *Transaction) Sender() *Account {
	return t.sendingAccountID
}

// Receiver returns the receiving account
func (t *Transaction) Receiver() *Account {
	return t.receivingAccountID
}

// Amount returns the transaction amount
func (t *Transaction) Amount() float64 {
	return t.amount
}

func (t *Transaction) Value() string {
	return t.value
}
