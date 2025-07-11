package bitcask_go

type Options struct {
	DirPath            string
	DataFileSize       int64       //数据文件大小
	SyncWrites         bool        // 每次写数据是否持久化
	BytesPerSync       uint        // 累计写到多少字节后进行持久化
	IndexType          IndexerType // 索引类型
	MMapAtStartup      bool        // 启动时是否使用MMap加载数据
	DataFileMergeRatio float32     // 数据文件合并的阈值
}

type IteratorOptions struct {
	Prefix  []byte
	Reverse bool
}

// WriteBatchOptions 批量写配置
type WriteBatchOptions struct {
	// 一个batch可以写的做大数据量
	MaxBatchNum uint
	// 提交时是否sync持久化
	SyncWrites bool
}

type IndexerType = int8

const (
	// BTree 索引
	BTree IndexerType = iota + 1

	// ART Adpative Radix Tree 自适应基数树索引
	ART

	// BPlusTree B+树索引，将索引存储到磁盘上
	BPlusTree
)

var DefaultOptions = Options{
	DirPath:            "./data",
	DataFileSize:       1024 * 1024 * 1024, // 1G
	SyncWrites:         false,
	IndexType:          BTree,
	BytesPerSync:       0,
	MMapAtStartup:      true,
	DataFileMergeRatio: 0.5,
}

var DefaultIteratorOptions = IteratorOptions{
	Prefix:  nil,
	Reverse: false,
}

var DefaultWriteBatchOptions = WriteBatchOptions{
	MaxBatchNum: 10000,
	SyncWrites:  true,
}
