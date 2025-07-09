package fio

import (
	"golang.org/x/exp/mmap"
	"os"
)

// MMap 内存文件映射 目前仅用于加速DB启动速度
type MMap struct {
	readerAt *mmap.ReaderAt
}

// NewMMapIOManager 初始化 MMap
func NewMMapIOManager(fileName string) (*MMap, error) {
	_, err := os.OpenFile(fileName, os.O_CREATE, DataFilePerm)
	if err != nil {
		return nil, err
	}
	readerAt, err := mmap.Open(fileName)
	if err != nil {
		return nil, err
	}
	return &MMap{readerAt: readerAt}, nil
}

func (mmap *MMap) Read(key []byte, offset int64) (int, error) {
	return mmap.readerAt.ReadAt(key, offset)
}

func (mmap *MMap) Write(data []byte) (int, error) {
	panic("not support")
}

func (mmap *MMap) Sync() error {
	panic("not support")
}

func (mmap *MMap) Close() error {
	return mmap.readerAt.Close()
}

func (mmap *MMap) Size() (int64, error) {
	return int64(mmap.readerAt.Len()), nil
}
