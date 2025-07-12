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
	ErrMaxBatchNumExceeded    = errors.New("max batch num exceeded")
	ErrMergeInProgress        = errors.New("merge in progress")
	ErrDatabaseIsUsing        = errors.New("database is using")
	ErrMergeRatioUnreached    = errors.New("the merge ratio do not reach the option")
	ErrNoEnoughSpaceForMerge  = errors.New("no enough disk space for merge")
)
