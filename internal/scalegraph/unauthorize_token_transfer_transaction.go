package scalegraph

type UnauthorizeTokenTransferTransaction struct {
	id      ScalegraphId
	account *Account
	tokenId *string
}

func newUnauthorizeTokenTransferTransaction(account *Account, tokenId *string) *UnauthorizeTokenTransferTransaction {
	txId, _ := NewScalegraphId()
	return &UnauthorizeTokenTransferTransaction{
		id:      txId,
		account: account,
		tokenId: tokenId,
	}
}

func (t *UnauthorizeTokenTransferTransaction) ID() ScalegraphId {
	return t.id
}

func (t *UnauthorizeTokenTransferTransaction) Type() TransactionType {
	tt := UnauthorizeTokenTransfer
	return tt
}

func (t *UnauthorizeTokenTransferTransaction) Sender() *Account {
	return t.account
}

func (t *UnauthorizeTokenTransferTransaction) Receiver() *Account {
	return t.account
}

func (t *UnauthorizeTokenTransferTransaction) TokenId() *string {
	return t.tokenId
}
