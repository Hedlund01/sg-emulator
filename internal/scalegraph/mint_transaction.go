package scalegraph

type MintTransaction struct {
	id       ScalegraphId
	sender   *Account
	receiver *Account
	amount   float64
}

func newMintTransaction(receiver *Account, amount float64) *MintTransaction {
	txId, _ := NewScalegraphId()
	return &MintTransaction{
		id:       txId,
		sender:   nil,
		receiver: receiver,
		amount:   amount,
	}
}

func (t *MintTransaction) ID() ScalegraphId {
	return t.id
}

func (t *MintTransaction) Type() TransactionType {
	tt := Mint
	return tt
}

func (t *MintTransaction) Sender() *Account {
	return t.sender
}

func (t *MintTransaction) Receiver() *Account {
	return t.receiver
}

func (t *MintTransaction) Amount() float64 {
	return t.amount
}
