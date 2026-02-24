package scalegraph

type BurnTransaction struct {
	id       ScalegraphId
	sender   *Account
	receiver *Account
	amount   float64
}

func newBurnTransaction(receiver *Account, amount float64) *BurnTransaction {
	txId, _ := NewScalegraphId()
	return &BurnTransaction{
		id:       txId,
		sender:   nil,
		receiver: receiver,
		amount:   amount,
	}
}

func (t *BurnTransaction) ID() ScalegraphId {
	return t.id
}

func (t *BurnTransaction) Type() TransactionType {
	tt := Burn
	return tt
}

func (t *BurnTransaction) Sender() *Account {
	return t.sender
}

func (t *BurnTransaction) Receiver() *Account {
	return t.receiver
}

func (t *BurnTransaction) Amount() float64 {
	return t.amount
}
