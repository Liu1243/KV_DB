package bitcask_go

import "errors"

var (
	ErrKeyIsEmpty             = errors.New("key is empty")
	ErrIndexUpdateFailed      = errors.New("index update failed")
	ErrKeyNotFound            = errors.New("key not found")
	ErrDataFileNotFound       = errors.New("data file not found")
	ErrDirPathIsEmpty         = errors.New("dir path is empty")
	ErrDataFileSizeInvalid    = errors.New("data file size must be greater than 0")
	ErrDataDirectoryCorrupted = errors.New("data directory corrupted")
)
