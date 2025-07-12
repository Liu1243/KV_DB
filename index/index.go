package index

import (
	"bitcask-go/data"
	"bytes"
	"github.com/google/btree"
)

// Indexer 抽象索引接口
type Indexer interface {
	Put(key []byte, pos *data.LogRecordPos) *data.LogRecordPos
	Get(key []byte) *data.LogRecordPos
	Delete(key []byte) (*data.LogRecordPos, bool)
	Size() int
	Iterator(reverse bool) Iterator
	Close() error
}

type IndexType = int8

const (
	// Btree 二叉树索引
	Btree IndexType = iota + 1
	// ART 自适应基数树索引
	ART
	// BPTree BPTee B+ 树索引
	BPTree
)

func NewIndexer(indexType IndexType, dirPath string, syncWrites bool) Indexer {
	switch indexType {
	case Btree:
		return NewBTree()
	case ART:
		return NewART()
	case BPTree:
		return NewBPlusTree(dirPath, syncWrites)
	default:
		panic("index type not support")
	}
}

type Item struct {
	Key []byte
	pos *data.LogRecordPos
}

func (ai *Item) Less(bi btree.Item) bool {
	return bytes.Compare(ai.Key, bi.(*Item).Key) == -1
}

// Iterator 通用索引迭代器
type Iterator interface {
	Rewind()
	// Seek 根据传入的key查找第一个大于（或小于）等于的目标key，从这个key开始遍历
	Seek(key []byte)
	Next()
	// Valid 判断是否遍历完所有key
	Valid() bool
	Key() []byte
	Value() *data.LogRecordPos
	Close()
}
