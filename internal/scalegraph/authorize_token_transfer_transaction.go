package scalegraph

type AuthorizeTokenTransferTransaction struct {
	id       ScalegraphId
	sender   *Account
	receiver *Account
	tokenId *ScalegraphId
}

func newAuthorizeTokenTransferTransaction(receiver *Account, tokenId *ScalegraphId) *AuthorizeTokenTransferTransaction {
	txId, _ := NewScalegraphId()
	return &AuthorizeTokenTransferTransaction{
		id:       txId,
		sender:   receiver,
		receiver: receiver,
		tokenId: tokenId,
	}
}

func (t *AuthorizeTokenTransferTransaction) ID() ScalegraphId {
	return t.id
}

func (t *AuthorizeTokenTransferTransaction) Type() TransactionType {
	tt := AuthorizeTokenTransfer
	return tt
}

func (t *AuthorizeTokenTransferTransaction) Sender() *Account {
	return t.sender
}

func (t *AuthorizeTokenTransferTransaction) Receiver() *Account {
	return t.receiver
}

func (t *AuthorizeTokenTransferTransaction) TokenId() *ScalegraphId {
	return t.tokenId
}
