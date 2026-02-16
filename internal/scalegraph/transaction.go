package scalegraph

type TransactionType int

const (
	Transfer TransactionType = iota
	Mint
	Burn
	MintToken
	BurnToken
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
	default:
		return "Unknown"
	}
}

func (tt TransactionType) EnumIndex() int {
	return int(tt)
}

type ITransaction interface {
	ID() *ScalegraphId
	Type() *TransactionType
	Sender() *Account
	Receiver() *Account
}
