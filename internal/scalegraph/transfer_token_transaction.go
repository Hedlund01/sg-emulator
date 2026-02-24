package scalegraph

type TransferTokenTransaction struct {
	id       ScalegraphId
	sender   *Account
	receiver *Account
	token *Token
}

func newTransferTokenTransaction(sender, receiver *Account, token *Token) *TransferTokenTransaction {
	txId, _ := NewScalegraphId()
	return &TransferTokenTransaction{
		id:       txId,
		sender:   sender,
		receiver: receiver,
		token:    token,
	}
}

func (t *TransferTokenTransaction) ID() ScalegraphId {
	return t.id
}

func (t *TransferTokenTransaction) Type() TransactionType {
	tt := TransferToken
	return tt
}

func (t *TransferTokenTransaction) Sender() *Account {
	return t.sender
}

func (t *TransferTokenTransaction) Receiver() *Account {
	return t.receiver
}

func (t *TransferTokenTransaction) Token() *Token {
	return t.token
}