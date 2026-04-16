package scalegraph

type TransactionType int

const (
	Transfer TransactionType = iota
	Mint
	Burn
	MintToken
	BurnToken
	TransferToken
	AuthorizeTokenTransfer
	UnauthorizeTokenTransfer
	ClawbackTokenTransfer
)

func (tt TransactionType) String() string {
	switch tt {
	case Transfer:
		return "Transfer"
	case Mint:
		return "Mint"
	case Burn:
		return "Burn"
	case MintToken:
		return "MintToken"
	case BurnToken:
		return "BurnToken"
	case TransferToken:
		return "TransferToken"
	case AuthorizeTokenTransfer:
		return "AuthorizeTokenTransfer"
	case UnauthorizeTokenTransfer:
		return "UnauthorizeTokenTransfer"
	case ClawbackTokenTransfer:
		return "ClawbackTokenTransfer"
	default:
		return "Unknown"
	}
}

func (tt TransactionType) EnumIndex() int {
	return int(tt)
}

type ITransaction interface {
	ID() ScalegraphId
	Type() TransactionType
	Sender() *Account
	Receiver() *Account
}

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

// Token transactions

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
		sender:   nil,
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

type AuthorizeTokenTransferTransaction struct {
	id         ScalegraphId
	sender     *Account // authorizer (future token receiver)
	receiver   *Account // token owner (current token holder)
	tokenId    *string
}

func newAuthorizeTokenTransferTransaction(authorizer, tokenOwner *Account, tokenId *string) *AuthorizeTokenTransferTransaction {
	txId, _ := NewScalegraphId()
	return &AuthorizeTokenTransferTransaction{
		id:       txId,
		sender:   authorizer,
		receiver: tokenOwner,
		tokenId:  tokenId,
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

func (t *AuthorizeTokenTransferTransaction) TokenId() *string {
	return t.tokenId
}

type TransferTokenTransaction struct {
	id       ScalegraphId
	sender   *Account
	receiver *Account
	token    *Token
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

type UnauthorizeTokenTransferTransaction struct {
	id       ScalegraphId
	sender   *Account // authorizer revoking (future token receiver)
	receiver *Account // token owner (current token holder)
	tokenId  *string
}

func newUnauthorizeTokenTransferTransaction(authorizer, tokenOwner *Account, tokenId *string) *UnauthorizeTokenTransferTransaction {
	txId, _ := NewScalegraphId()
	return &UnauthorizeTokenTransferTransaction{
		id:       txId,
		sender:   authorizer,
		receiver: tokenOwner,
		tokenId:  tokenId,
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
	return t.sender
}

func (t *UnauthorizeTokenTransferTransaction) Receiver() *Account {
	return t.receiver
}

func (t *UnauthorizeTokenTransferTransaction) TokenId() *string {
	return t.tokenId
}

type BurnTokenTransaction struct {
	id        ScalegraphId
	accountID *Account
	tokenID   string
}

func newBurnTokenTransaction(account *Account, tokenID string) *BurnTokenTransaction {
	txId, _ := NewScalegraphId()
	return &BurnTokenTransaction{
		id:        txId,
		accountID: account,
		tokenID:   tokenID,
	}
}

func (t *BurnTokenTransaction) ID() ScalegraphId {
	return t.id
}

func (t *BurnTokenTransaction) Type() TransactionType {
	tt := BurnToken
	return tt
}

func (t *BurnTokenTransaction) Sender() *Account {
	return t.accountID
}

func (t *BurnTokenTransaction) Receiver() *Account {
	return nil
}

func (t *BurnTokenTransaction) TokenID() string {
	return t.tokenID
}

type ClawbackTokenTransaction struct {
	id    ScalegraphId
	from  *Account
	to    *Account
	token Token
}

func newClawbackTokenTransaction(from, to *Account, token Token) *ClawbackTokenTransaction {
	txId, _ := NewScalegraphId()
	return &ClawbackTokenTransaction{
		id:    txId,
		from:  from,
		to:    to,
		token: token,
	}
}

func (t *ClawbackTokenTransaction) ID() ScalegraphId {
	return t.id
}

func (t *ClawbackTokenTransaction) Type() TransactionType {
	tt := ClawbackTokenTransfer
	return tt
}

func (t *ClawbackTokenTransaction) Sender() *Account {
	return t.from
}

func (t *ClawbackTokenTransaction) Receiver() *Account {
	return t.to
}

func (t *ClawbackTokenTransaction) Token() Token {
	return t.token
}
