package scalegraph

import (
	"slices"
	"sync"
)

// Blockchain represents a chain of blocks using a linked list structure.
// The chain stores only head and tail pointers, allowing O(1) append
// and supporting chains of arbitrary length.
type Blockchain struct {
	mu   sync.RWMutex
	head *Block
	tail *Block
}

// newBlockchain creates a new blockchain with a genesis block
func newBlockchain() *Blockchain {
	genesis := genesisBlock()
	return &Blockchain{
		head: genesis,
		tail: genesis,
	}
}

// append adds a new block with the given transaction to the end of the chain
func (bc *Blockchain) append(trx *Transaction) *Block {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	newBlock := bc.tail.newBlock(trx)
	bc.tail = newBlock
	return newBlock
}

// Head returns the first block in the chain (genesis block)
func (bc *Blockchain) Head() *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.head
}

// Tail returns the last block in the chain
func (bc *Blockchain) Tail() *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.tail
}

func (bc *Blockchain) GetBlocks() []*Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	var blocks []*Block
	current := bc.Tail()
	for current != nil {
		blocks = append(blocks, current)
		current = current.PrevBlock()
	}

	slices.Reverse(blocks)

	return blocks
}
