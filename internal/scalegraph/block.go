package scalegraph

import "fmt"

// Block is immutable after creation - no mutex needed
type Block struct {
	id          ScalegraphId
	prevBlock   *Block
	transaction ITransaction
}

func genesisBlock() *Block {
	genesisId, _ := NewScalegraphId()

	return &Block{
		id:          genesisId,
		prevBlock:   nil,
		transaction: nil,
	}
}

func (b *Block) newBlock(trx ITransaction) *Block {
	newId, _ := NewScalegraphId()

	return &Block{
		id:          newId,
		prevBlock:   b,
		transaction: trx,
	}
}

func (b *Block) ID() ScalegraphId {
	return b.id
}

func (b *Block) PrevBlock() *Block {
	return b.prevBlock
}

// Transaction returns the transaction in this block
func (b *Block) Transaction() ITransaction {
	return b.transaction
}

func (b *Block) String() string {
	return fmt.Sprintf("Block(ID: %s, PrevBlock: %s, Transaction: %s)", b.id, b.prevBlock, b.transaction)
}
