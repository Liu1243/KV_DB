package index

import (
	"bitcask-go/data"
	"bytes"
	"github.com/google/btree"
	"sort"
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

func (B BTree) Size() int {
	B.lock.RLock()
	defer B.lock.RUnlock()
	return B.tree.Len()
}

func (B BTree) Iterator(reverse bool) Iterator {
	if B.tree == nil {
		return nil
	}
	B.lock.RLock()
	defer B.lock.RUnlock()
	return newBTreeIterator(B.tree, reverse)
}

func newBTreeIterator(tree *btree.BTree, reverse bool) Iterator {
	var idx int
	values := make([]*Item, tree.Len())

	// 将所有数据放到数组中
	saveValues := func(it btree.Item) bool {
		values[idx] = it.(*Item)
		idx++
		return true
	}

	if reverse {
		tree.Descend(saveValues)
	} else {
		tree.Ascend(saveValues)
	}

	return &btreeIterator{
		currIndex: 0,
		reverse:   reverse,
		values:    values,
	}
}

type btreeIterator struct {
	currIndex int     // 当前遍历的下标位置
	reverse   bool    // 是否是反向遍历
	values    []*Item // key以及索引信息
}

func (bti *btreeIterator) Rewind() {
	bti.currIndex = 0
}
func (bti *btreeIterator) Seek(key []byte) {
	if bti.reverse {
		bti.currIndex = sort.Search(len(bti.values), func(i int) bool {
			return bytes.Compare(bti.values[i].Key, key) <= 0
		})
	} else {
		bti.currIndex = sort.Search(len(bti.values), func(i int) bool {
			return bytes.Compare(bti.values[i].Key, key) >= 0
		})
	}
}
func (bti *btreeIterator) Next() {
	bti.currIndex++
}

func (bti *btreeIterator) Valid() bool {
	return bti.currIndex < len(bti.values)
}
func (bti *btreeIterator) Key() []byte {
	return bti.values[bti.currIndex].Key
}
func (bti *btreeIterator) Value() *data.LogRecordPos {
	return bti.values[bti.currIndex].pos
}
func (bti *btreeIterator) Close() {
	bti.values = nil
}
