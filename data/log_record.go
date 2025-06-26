package data

// LogRecord 数据内存索引 描述数据在磁盘上的位置
type LogRecord struct {
	Fid    uint32 // 文件id
	Offset int64  // 偏移量
}
