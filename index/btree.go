package index

import (
	"bitcask-go/data"
	"github.com/google/btree"
	"sync"
)

// BTree 索引 封装了BTree
type BTree struct {
	tree *btree.BTree
	lock *sync.RWMutex
}

func NewBTree() *BTree {
	return &BTree{
		tree: btree.New(32),
		lock: new(sync.RWMutex),
	}
}

func (B BTree) Put(key []byte, pos *data.LogRecordPos) bool {
	it := &Item{Key: key, pos: pos}
	B.lock.Lock()
	defer B.lock.Unlock()
	B.tree.ReplaceOrInsert(it)

	return true
}

func (B BTree) Get(key []byte) *data.LogRecordPos {
	it := &Item{Key: key}
	btreeItem := B.tree.Get(it)
	if btreeItem == nil {
		return nil
	}
	return btreeItem.(*Item).pos
}

func (B BTree) Delete(key []byte) bool {
	it := &Item{Key: key}
	B.lock.Lock()
	defer B.lock.Unlock()
	oldItem := B.tree.Delete(it)
	if oldItem == nil {
		return false
	}
	return true
}
