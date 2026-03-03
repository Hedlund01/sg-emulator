package scalegraph

import (
	"slices"
	"sync"
)

// IBlockchain defines the interface for blockchain operations
type IBlockchain interface {
	append(trx ITransaction) *Block
	removeLatestBlock()
	Head() *Block
	Tail() *Block
	GetBlocks() []*Block
	Len() int
}

// Blockchain represents a chain of blocks using a linked list structure.
// The chain stores only head and tail pointers, allowing O(1) append
// and supporting chains of arbitrary length.
type Blockchain struct {
	mu     sync.RWMutex
	head   *Block
	tail   *Block
	length int // Cached block count (including genesis)
}

// newBlockchain creates a new blockchain with a genesis block
func newBlockchain() *Blockchain {
	genesis := genesisBlock()
	return &Blockchain{
		head:   genesis,
		tail:   genesis,
		length: 1, // Genesis block counts
	}
}

// append adds a new block with the given transaction to the end of the chain
func (bc *Blockchain) append(trx ITransaction) *Block {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	newBlock := bc.tail.newBlock(trx)
	bc.tail = newBlock
	bc.length++
	return newBlock
}

func (bc *Blockchain) removeLatestBlock(){
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.length <= 1 {
		return // Can't remove genesis block
	}
	bc.tail = bc.tail.PrevBlock()
	bc.length--
}

// Len returns the number of blocks in the chain (including genesis)
func (bc *Blockchain) Len() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.length
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
