package scalegraph

type TransferTransaction struct {
	id       ScalegraphId
	sender   *Account
	receiver *Account
	amount   float64
}

func newTransferTransaction(sender, receiver *Account, amount float64) *TransferTransaction {
	txId, _ := NewScalegraphId()
	return &TransferTransaction{
		id:       txId,
		sender:   sender,
		receiver: receiver,
		amount:   amount,
	}
}

func (t *TransferTransaction) ID() ScalegraphId {
	return t.id
}

func (t *TransferTransaction) Type() TransactionType {
	tt := Transfer
	return tt
}

func (t *TransferTransaction) Sender() *Account {
	return t.sender
}

func (t *TransferTransaction) Receiver() *Account {
	return t.receiver
}

func (t *TransferTransaction) Amount() float64 {
	return t.amount
}
