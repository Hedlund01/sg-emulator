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
	FreezeToken
	UnfreezeToken
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
	case FreezeToken:
		return "FreezeToken"
	case UnfreezeToken:
		return "UnfreezeToken"
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
	id       ScalegraphId
	sender   *Account // authorizer (future token receiver)
	receiver *Account // token owner (current token holder)
	tokenId  *string
	nonce    uint64
}

func newAuthorizeTokenTransferTransaction(authorizer, tokenOwner *Account, tokenId *string, nonce uint64) *AuthorizeTokenTransferTransaction {
	txId, _ := NewScalegraphId()
	return &AuthorizeTokenTransferTransaction{
		id:       txId,
		sender:   authorizer,
		receiver: tokenOwner,
		tokenId:  tokenId,
		nonce:    nonce,
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

func (t *AuthorizeTokenTransferTransaction) Nonce() uint64 {
	return t.nonce
}

type TransferTokenTransaction struct {
	id       ScalegraphId
	sender   *Account
	receiver *Account
	token    *Token
	nonce    uint64
}

func newTransferTokenTransaction(sender, receiver *Account, token *Token, nonce uint64) *TransferTokenTransaction {
	txId, _ := NewScalegraphId()
	return &TransferTokenTransaction{
		id:       txId,
		sender:   sender,
		receiver: receiver,
		token:    token,
		nonce:    nonce,
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

func (t *TransferTokenTransaction) Nonce() uint64 {
	return t.nonce
}

type UnauthorizeTokenTransferTransaction struct {
	id       ScalegraphId
	sender   *Account // authorizer revoking (future token receiver)
	receiver *Account // token owner (current token holder)
	tokenId  *string
	nonce    uint64
}

func newUnauthorizeTokenTransferTransaction(authorizer, tokenOwner *Account, tokenId *string, nonce uint64) *UnauthorizeTokenTransferTransaction {
	txId, _ := NewScalegraphId()
	return &UnauthorizeTokenTransferTransaction{
		id:       txId,
		sender:   authorizer,
		receiver: tokenOwner,
		tokenId:  tokenId,
		nonce:    nonce,
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

func (t *UnauthorizeTokenTransferTransaction) Nonce() uint64 {
	return t.nonce
}

type BurnTokenTransaction struct {
	id        ScalegraphId
	accountID *Account
	tokenID   string
	nonce     uint64
}

func newBurnTokenTransaction(account *Account, tokenID string, nonce uint64) *BurnTokenTransaction {
	txId, _ := NewScalegraphId()
	return &BurnTokenTransaction{
		id:        txId,
		accountID: account,
		tokenID:   tokenID,
		nonce:     nonce,
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

func (t *BurnTokenTransaction) Nonce() uint64 {
	return t.nonce
}

type ClawbackTokenTransaction struct {
	id    ScalegraphId
	from  *Account
	to    *Account
	token Token
	nonce uint64
}

func newClawbackTokenTransaction(from, to *Account, token Token, nonce uint64) *ClawbackTokenTransaction {
	txId, _ := NewScalegraphId()
	return &ClawbackTokenTransaction{
		id:    txId,
		from:  from,
		to:    to,
		token: token,
		nonce: nonce,
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

func (t *ClawbackTokenTransaction) Nonce() uint64 {
	return t.nonce
}

type FreezeTokenTransaction struct {
	id              ScalegraphId
	freezeAuthority *Account // sender
	tokenHolder     *Account // receiver
	tokenID         string
	nonce           uint64
}

func newFreezeTokenTransaction(authority, holder *Account, tokenID string, nonce uint64) *FreezeTokenTransaction {
	txId, _ := NewScalegraphId()
	return &FreezeTokenTransaction{
		id:              txId,
		freezeAuthority: authority,
		tokenHolder:     holder,
		tokenID:         tokenID,
		nonce:           nonce,
	}
}

func (t *FreezeTokenTransaction) ID() ScalegraphId      { return t.id }
func (t *FreezeTokenTransaction) Type() TransactionType { return FreezeToken }
func (t *FreezeTokenTransaction) Sender() *Account      { return t.freezeAuthority }
func (t *FreezeTokenTransaction) Receiver() *Account    { return t.tokenHolder }
func (t *FreezeTokenTransaction) TokenID() string       { return t.tokenID }
func (t *FreezeTokenTransaction) Nonce() uint64         { return t.nonce }

type UnfreezeTokenTransaction struct {
	id              ScalegraphId
	freezeAuthority *Account // sender
	tokenHolder     *Account // receiver
	tokenID         string
	nonce           uint64
}

func newUnfreezeTokenTransaction(authority, holder *Account, tokenID string, nonce uint64) *UnfreezeTokenTransaction {
	txId, _ := NewScalegraphId()
	return &UnfreezeTokenTransaction{
		id:              txId,
		freezeAuthority: authority,
		tokenHolder:     holder,
		tokenID:         tokenID,
		nonce:           nonce,
	}
}

func (t *UnfreezeTokenTransaction) ID() ScalegraphId      { return t.id }
func (t *UnfreezeTokenTransaction) Type() TransactionType { return UnfreezeToken }
func (t *UnfreezeTokenTransaction) Sender() *Account      { return t.freezeAuthority }
func (t *UnfreezeTokenTransaction) Receiver() *Account    { return t.tokenHolder }
func (t *UnfreezeTokenTransaction) TokenID() string       { return t.tokenID }
func (t *UnfreezeTokenTransaction) Nonce() uint64         { return t.nonce }
