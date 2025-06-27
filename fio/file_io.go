package fio

import (
	"errors"
	"os"
)

// FileIO 标准系统文件IO
type FileIO struct {
	fd *os.File
}

func (f *FileIO) Size() (int64, error) {
	stat, err := f.fd.Stat()
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}

func NewFileIOManager(fileName string) (*FileIO, error) {
	fd, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, DataFilePerm)
	if err != nil {
		return nil, err
	}
	return &FileIO{fd: fd}, nil
}

func (f FileIO) Read(key []byte, offset int64) (int, error) {
	return f.fd.ReadAt(key, offset)
}

// f.fd.Write在windows上也能写入只读文件，所需需要写入前判断
func (f *FileIO) Write(data []byte) (int, error) {
	if f.fd == nil {
		return 0, errors.New("file descriptor is nil")
	}

	// 检查文件是否是以只读方式打开的
	stat, err := f.fd.Stat()
	if err != nil {
		return 0, err
	}
	if stat.Mode()&0200 == 0 {
		return 0, errors.New("file is read-only, write permission denied")
	}

	return f.fd.Write(data)
}

func (f FileIO) Sync() error {
	return f.fd.Sync()
}

func (f FileIO) Close() error {
	return f.fd.Close()
}
