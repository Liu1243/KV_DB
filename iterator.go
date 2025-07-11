package bitcask_go

import (
	"bitcask-go/index"
	"bytes"
)

type Iterator struct {
	indexIter index.Iterator
	db        *DB
	options   IteratorOptions
}

func (db *DB) NewIterator(opts IteratorOptions) *Iterator {
	indexIter := db.index.Iterator(opts.Reverse)

	return &Iterator{
		db:        db,
		indexIter: indexIter,
		options:   opts,
	}
}

func (it *Iterator) Rewind() {
	it.indexIter.Rewind()
	it.skipToNext()
}
func (it *Iterator) Seek(key []byte) {
	it.indexIter.Seek(key)
	it.skipToNext()
}
func (it *Iterator) Next() {
	it.indexIter.Next()
	it.skipToNext()
}

func (it *Iterator) Valid() bool {
	return it.indexIter.Valid()
}
func (it *Iterator) Key() []byte {
	return it.indexIter.Key()
}

// Value 获取当前遍历位置的Value数据，index中存的是pos数据，需要在从db中读取
func (it *Iterator) Value() ([]byte, error) {
	logRecordPos := it.indexIter.Value()
	it.db.mu.RLock()
	defer it.db.mu.RUnlock()
	return it.db.getValueByPosition(logRecordPos)
}
func (it *Iterator) Close() {
	it.indexIter.Close()
}

func (it *Iterator) skipToNext() {
	prefixLen := len(it.options.Prefix)
	if prefixLen == 0 {
		return
	}

	for ; it.indexIter.Valid(); it.indexIter.Next() {
		key := it.indexIter.Key()
		if prefixLen <= len(key) && bytes.Compare(it.options.Prefix, key[:prefixLen]) == 0 {
			break
		}
	}
}
