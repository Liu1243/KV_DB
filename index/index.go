package index

import (
	"bitcask-go/data"
	"bytes"
	"github.com/google/btree"
)

// Indexer 抽象索引接口
type Indexer interface {
	Put(key []byte, pos *data.LogRecord) bool
	Get(key []byte) *data.LogRecord
	Delete(key []byte) bool
}

type Item struct {
	Key []byte
	pos *data.LogRecord
}

func (ai *Item) Less(bi btree.Item) bool {
	return bytes.Compare(ai.Key, bi.(*Item).Key) == -1
}
