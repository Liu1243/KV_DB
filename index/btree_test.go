package index

import (
	"bitcask-go/data"
	"github.com/google/btree"
	"reflect"
	"sync"
	"testing"
)

func TestBTree_Delete(t *testing.T) {
	type fields struct {
		tree *btree.BTree
		lock *sync.RWMutex
	}
	type args struct {
		key []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test1",
			fields: fields{
				tree: btree.New(2),
				lock: &sync.RWMutex{},
			},
			args: args{
				key: []byte("test"),
			},
			want: true,
		},
		{
			name: "test_nil",
			fields: fields{
				tree: btree.New(2),
				lock: &sync.RWMutex{},
			},
			args: args{
				key: nil,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			B := BTree{
				tree: tt.fields.tree,
				lock: tt.fields.lock,
			}

			B.Put([]byte("test"), &data.LogRecordPos{1, 10})
			B.Put(nil, &data.LogRecordPos{1, 20})

			if got := B.Delete(tt.args.key); got != tt.want {
				t.Errorf("Delete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBTree_Get(t *testing.T) {
	type fields struct {
		tree *btree.BTree
		lock *sync.RWMutex
	}
	type args struct {
		key []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *data.LogRecordPos
	}{
		{
			name: "test1",
			fields: fields{
				tree: btree.New(32),
				lock: &sync.RWMutex{},
			},
			args: args{
				key: []byte("test"),
			},
			want: &data.LogRecordPos{
				Fid:    1,
				Offset: 10,
			},
		},
		{
			name: "test_nil",
			fields: fields{
				tree: btree.New(32),
				lock: &sync.RWMutex{},
			},
			args: args{
				key: nil,
			},
			want: &data.LogRecordPos{
				Fid:    2,
				Offset: 20,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			B := BTree{
				tree: tt.fields.tree,
				lock: tt.fields.lock,
			}

			// 插入实例
			B.Put([]byte("test"), &data.LogRecordPos{1, 10})
			B.Put(nil, &data.LogRecordPos{2, 20})

			if got := B.Get(tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBTree_Put(t *testing.T) {
	type fields struct {
		tree *btree.BTree
		lock *sync.RWMutex
	}
	type args struct {
		key []byte
		pos *data.LogRecordPos
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test1",
			fields: fields{
				tree: btree.New(32),
				lock: &sync.RWMutex{},
			},
			args: args{
				key: []byte("test"),
				pos: &data.LogRecordPos{1, 100},
			},
			want: true,
		},
		{
			name: "test_nil",
			fields: fields{
				tree: btree.New(32),
				lock: &sync.RWMutex{},
			},
			args: args{
				key: nil,
				pos: &data.LogRecordPos{1, 2},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			B := BTree{
				tree: tt.fields.tree,
				lock: tt.fields.lock,
			}
			if got := B.Put(tt.args.key, tt.args.pos); got != tt.want {
				t.Errorf("Put() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newBTree(t *testing.T) {
	tests := []struct {
		name string
		want *BTree
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBTree(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newBTree() = %v, want %v", got, tt.want)
			}
		})
	}
}
