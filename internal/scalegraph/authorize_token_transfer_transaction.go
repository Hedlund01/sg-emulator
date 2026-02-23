package scalegraph

type AuthorizeTokenTransferTransaction struct {
	id      ScalegraphId
	account *Account
	tokenId *string
}

func newAuthorizeTokenTransferTransaction(account *Account, tokenId *string) *AuthorizeTokenTransferTransaction {
	txId, _ := NewScalegraphId()
	return &AuthorizeTokenTransferTransaction{
		id:      txId,
		account: account,
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
	return t.account
}

func (t *AuthorizeTokenTransferTransaction) Receiver() *Account {
	return t.account
}

func (t *AuthorizeTokenTransferTransaction) TokenId() *string {
	return t.tokenId
}
