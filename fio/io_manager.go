package fio

const DataFilePerm = 0644

type FileIOType = byte

const (
	StandardFIO FileIOType = iota
	MemoryMap
)

// IOManager 抽象IO管理接口
type IOManager interface {
	Read(key []byte, offset int64) (int, error)

	Write(data []byte) (int, error)

	// Sync 持久化数据
	Sync() error

	Close() error

	Size() (int64, error)
}

func NewIOManager(fileName string, ioType FileIOType) (IOManager, error) {
	switch ioType {
	case StandardFIO:
		return NewFileIOManager(fileName)
	case MemoryMap:
		return NewMMapIOManager(fileName)
	default:
		panic("unsupported io type")
	}
}
