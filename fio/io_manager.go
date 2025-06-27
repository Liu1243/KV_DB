package fio

const DataFilePerm = 0644

// IOManager 抽象IO管理接口
type IOManager interface {
	Read(key []byte, offset int64) (int, error)

	Write(data []byte) (int, error)

	// Sync 持久化数据
	Sync() error

	Close() error
}
