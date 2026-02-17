package scalegraph

type MintTokenTransaction struct {
	id       ScalegraphId
	sender   *Account
	receiver *Account
	token    *Token
}

func newMintTokenTransaction(receiver *Account, token *Token) *MintTokenTransaction {
	txId, _ := NewScalegraphId()
	if receiver == nil || token == nil {
		return nil
	}
	return &MintTokenTransaction{
		id:       txId,
		sender:   receiver,
		receiver: receiver,
		token:    token,
	}
}

func (t *MintTokenTransaction) ID() ScalegraphId {
	return t.id
}

func (t *MintTokenTransaction) Type() TransactionType {
	return MintToken
}

func (t *MintTokenTransaction) Sender() *Account {
	return t.sender
}

func (t *MintTokenTransaction) Receiver() *Account {
	return t.receiver
}

func (t *MintTokenTransaction) Token() *Token {
	return t.token
}
