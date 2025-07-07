package index

import (
	"bitcask-go/data"
	"go.etcd.io/bbolt"
	"path/filepath"
)

const bptreeIndexFileName = "bptree-index"

var indexBucketName = []byte("bitcask-index")

// BPlusTree B+树索引
type BPlusTree struct {
	tree *bbolt.DB
}

func NewBPlusTree(dirPath string, syncWrites bool) *BPlusTree {
	opts := bbolt.DefaultOptions
	opts.NoSync = !syncWrites
	bptree, err := bbolt.Open(filepath.Join(dirPath, bptreeIndexFileName), 0644, opts)
	if err != nil {
		panic("failed to open bptree")
	}

	// 创建对应的bucket
	if err := bptree.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(indexBucketName))
		return err
	}); err != nil {
		panic("failed to create bucket")
	}

	return &BPlusTree{tree: bptree}
}

func (bpt *BPlusTree) Put(key []byte, pos *data.LogRecordPos) bool {
	if err := bpt.tree.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(indexBucketName))
		return bucket.Put(key, data.EncodeLogRecordPos(pos))
	}); err != nil {
		panic("failed to put key into bptree")
	}

	return true
}

func (bpt *BPlusTree) Get(key []byte) *data.LogRecordPos {
	var pos *data.LogRecordPos
	if err := bpt.tree.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(indexBucketName))
		value := bucket.Get(key)
		if len(value) != 0 {
			pos = data.DecodeLogRecordPos(value)
		}
		return nil
	}); err != nil {
		panic("failed to get value from bptree")
	}

	return pos
}
func (bpt *BPlusTree) Delete(key []byte) bool {
	var ok bool
	if err := bpt.tree.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(indexBucketName))
		if value := bucket.Get(key); len(value) != 0 {
			ok = true
			return bucket.Delete(key)
		}
		return nil
	}); err != nil {
		panic("failed to delete key from bptree")
	}

	return ok
}

func (bpt *BPlusTree) Size() int {
	var size int
	if err := bpt.tree.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(indexBucketName))
		size = bucket.Stats().KeyN
		return nil
	}); err != nil {
		panic("failed to get size from bptree")
	}

	return size
}
func (bpt *BPlusTree) Iterator(reverse bool) Iterator {
	return newBptreeIterator(bpt.tree, reverse)
}
func (bpt *BPlusTree) Close() error {
	return bpt.tree.Close()
}

// B+树迭代器
type bptreeIterator struct {
	tx        *bbolt.Tx
	cursor    *bbolt.Cursor
	reverse   bool
	currKey   []byte
	currValue []byte
}

func newBptreeIterator(tree *bbolt.DB, reverse bool) *bptreeIterator {
	tx, err := tree.Begin(false)
	if err != nil {
		panic("failed to begin tx")
	}
	bpi := &bptreeIterator{
		tx:      tx,
		cursor:  tx.Bucket([]byte(indexBucketName)).Cursor(),
		reverse: reverse,
	}

	// cursor rewind
	bpi.Rewind()

	return bpi
}

func (bpi *bptreeIterator) Rewind() {
	if bpi.reverse {
		bpi.currKey, bpi.currValue = bpi.cursor.Last()
	} else {
		bpi.currKey, bpi.currValue = bpi.cursor.First()
	}
}
func (bpi *bptreeIterator) Seek(key []byte) {
	bpi.currKey, bpi.currValue = bpi.cursor.Seek(key)
}
func (bpi *bptreeIterator) Next() {
	if bpi.reverse {
		bpi.currKey, bpi.currValue = bpi.cursor.Prev()
	} else {
		bpi.currKey, bpi.currValue = bpi.cursor.Next()
	}
}
func (bpi *bptreeIterator) Valid() bool {
	return len(bpi.currKey) != 0
}
func (bpi *bptreeIterator) Key() []byte {
	return bpi.currKey
}
func (bpi *bptreeIterator) Value() *data.LogRecordPos {
	return data.DecodeLogRecordPos(bpi.currValue)
}
func (bpi *bptreeIterator) Close() {
	_ = bpi.tx.Rollback()
}
