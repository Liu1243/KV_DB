package bitcask_go

type Options struct {
	DirPath      string
	DataFileSize int64       //数据文件大小
	SyncWrites   bool        // 每次写数据是否持久化
	IndexType    IndexerType // 索引类型
}

type IndexerType = int8

const (
	// BTree 索引
	BTree IndexerType = iota + 1

	// ART Adpative Radix Tree 自适应基数树索引
	ART
)

var DefaultOptions = Options{
	DirPath:      "./data",
	DataFileSize: 1024 * 1024 * 1024, // 1G
	SyncWrites:   false,
	IndexType:    BTree,
}
