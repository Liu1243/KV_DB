package data

import "encoding/binary"

type LogRecordType = byte

const (
	LogRecordNormal LogRecordType = iota
	LogRecordDeleted
)

// crc type keySize valueSize
// 4 +  1  +  5   +   5 = 15
const maxLogRecordHeaderSize = binary.MaxVarintLen32*2 + 5

// LogRecord 写入到数据文件的记录
type LogRecord struct {
	Key   []byte
	Value []byte
	Type  LogRecordType
}

// LogRecordHeader LogRecord头部信息
type LogRecordHeader struct {
	crc        uint32
	recordType LogRecordType
	keySize    int32
	valueSize  int32
}

// LogRecordPos 数据内存索引 描述数据在磁盘上的位置
type LogRecordPos struct {
	Fid    uint32 // 文件id
	Offset int64  // 偏移量
}

// EncodeLogRecord 对 LogRecord 进行编码，返回字节数组以及长度
func EncodeLogRecord(LogRecord *LogRecord) ([]byte, int64) {
	return nil, 0
}

func DecodeLogRecordHeader(buf []byte) (*LogRecordHeader, int64) {
	return nil, 0
}

func GetRecordCRC(record *LogRecord, header []byte) uint32 {
	return 0
}
