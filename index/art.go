package index

import (
	"bitcask-go/data"
	"bytes"
	goart "github.com/plar/go-adaptive-radix-tree"
	"sort"
	"sync"
)

// AdaptiveRadixTree 自适应基数树索引
type AdaptiveRadixTree struct {
	tree goart.Tree
	lock *sync.RWMutex
}

// NewART 初始化ART索引
func NewART() *AdaptiveRadixTree {
	return &AdaptiveRadixTree{
		tree: goart.New(),
		lock: new(sync.RWMutex),
	}
}

func (art *AdaptiveRadixTree) Put(key []byte, pos *data.LogRecordPos) *data.LogRecordPos {
	art.lock.Lock()
	oldValue, _ := art.tree.Insert(key, pos)
	art.lock.Unlock()

	if oldValue == nil {
		return nil
	}

	return oldValue.(*data.LogRecordPos)
}
func (art *AdaptiveRadixTree) Get(key []byte) *data.LogRecordPos {
	art.lock.RLock()
	defer art.lock.RUnlock()
	value, found := art.tree.Search(key)
	if !found {
		return nil
	}
	return value.(*data.LogRecordPos)
}
func (art *AdaptiveRadixTree) Delete(key []byte) (*data.LogRecordPos, bool) {
	art.lock.Lock()
	oldValue, deleted := art.tree.Delete(key)
	art.lock.Unlock()

	if oldValue == nil {
		return nil, false
	}

	return oldValue.(*data.LogRecordPos), deleted
}
func (art *AdaptiveRadixTree) Size() int {
	art.lock.RLock()
	defer art.lock.RUnlock()
	return art.tree.Size()
}
func (art *AdaptiveRadixTree) Iterator(reverse bool) Iterator {
	art.lock.RLock()
	defer art.lock.RUnlock()
	return newARTIterator(art.tree, reverse)
}

func (art *AdaptiveRadixTree) Close() error {
	return nil
}

// Art 索引迭代器
type artIterator struct {
	currentIndex int
	reverse      bool
	values       []*Item
}

func newARTIterator(tree goart.Tree, reverse bool) *artIterator {
	var idx int
	if reverse {
		idx = tree.Size() - 1
	}
	values := make([]*Item, tree.Size())
	saveValues := func(node goart.Node) bool {
		item := &Item{
			Key: node.Key(),
			pos: node.Value().(*data.LogRecordPos),
		}
		values[idx] = item
		if reverse {
			idx--
		} else {
			idx++
		}
		return true
	}

	tree.ForEach(saveValues)

	return &artIterator{
		currentIndex: 0,
		reverse:      reverse,
		values:       values,
	}
}

func (ai *artIterator) Rewind() {
	ai.currentIndex = 0
}
func (ai *artIterator) Seek(key []byte) {
	if ai.reverse {
		ai.currentIndex = sort.Search(len(ai.values), func(i int) bool {
			return bytes.Compare(ai.values[i].Key, key) <= 0
		})
	} else {
		ai.currentIndex = sort.Search(len(ai.values), func(i int) bool {
			return bytes.Compare(ai.values[i].Key, key) >= 0
		})
	}
}
func (ai *artIterator) Next() {
	ai.currentIndex += 1
}
func (ai *artIterator) Valid() bool {
	return ai.currentIndex < len(ai.values)
}
func (ai *artIterator) Key() []byte {
	return ai.values[ai.currentIndex].Key
}
func (ai *artIterator) Value() *data.LogRecordPos {
	return ai.values[ai.currentIndex].pos
}
func (ai *artIterator) Close() {
	ai.values = nil
}
