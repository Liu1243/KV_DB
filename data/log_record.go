package data

import (
	"encoding/binary"
	"hash/crc32"
)

type LogRecordType = byte

const (
	LogRecordNormal LogRecordType = iota
	LogRecordDeleted
	LogRecordTxnFinished
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
	keySize    uint32
	valueSize  uint32
}

// LogRecordPos 数据内存索引 描述数据在磁盘上的位置
type LogRecordPos struct {
	Fid    uint32 // 文件id
	Offset int64  // 偏移量
	Size   uint32 // 标识数据在磁盘上的大小
}

// TransactionRecord 暂存的事务相关的数据
type TransactionRecord struct {
	Record *LogRecord
	Pos    *LogRecordPos
}

// EncodeLogRecord 对 LogRecord 进行编码，返回字节数组以及长度
//
//	+-------------+-------------+-------------+--------------+-------------+--------------+
//	| crc 校验值  |  type 类型   |    key size |   value size |      key    |      value   |
//	+-------------+-------------+-------------+--------------+-------------+--------------+
//	    4字节          1字节        变长（最大5）   变长（最大5）     变长           变长
func EncodeLogRecord(LogRecord *LogRecord) ([]byte, int64) {
	// 初始化一个header部分的字节数组
	header := make([]byte, maxLogRecordHeaderSize)

	// 第五个字节存储Type
	header[4] = byte(LogRecord.Type)
	var index = 5
	// key value使用变长类型 节省空间
	index += binary.PutVarint(header[index:], int64(len(LogRecord.Key)))
	index += binary.PutVarint(header[index:], int64(len(LogRecord.Value)))

	var size = index + len(LogRecord.Key) + len(LogRecord.Value)
	encBytes := make([]byte, size)

	//将header和key value写入encBytes
	copy(encBytes[:index], header[:index])
	copy(encBytes[index:], LogRecord.Key)
	copy(encBytes[index+len(LogRecord.Key):], LogRecord.Value)

	// 对整个LogRecord做CRC校验
	crc := crc32.ChecksumIEEE(encBytes[4:])
	binary.LittleEndian.PutUint32(encBytes[:4], crc)

	return encBytes, int64(size)
}

// EncodeLogRecordPos 对位置信息进行编码
func EncodeLogRecordPos(pos *LogRecordPos) []byte {
	buf := make([]byte, binary.MaxVarintLen64+binary.MaxVarintLen32*2)
	index := 0
	index += binary.PutVarint(buf[index:], int64(pos.Fid))
	index += binary.PutVarint(buf[index:], pos.Offset)
	index += binary.PutVarint(buf[index:], int64(pos.Size))
	return buf[:index]
}

// DecodeLogRecordPos 解码LogRecordPos
func DecodeLogRecordPos(buf []byte) *LogRecordPos {
	var index = 0
	fileId, n := binary.Varint(buf[index:])
	index += n
	offset, n := binary.Varint(buf[index:])
	index += n
	size, _ := binary.Varint(buf[index:])
	return &LogRecordPos{
		Fid:    uint32(fileId),
		Offset: offset,
		Size:   uint32(size),
	}
}

func DecodeLogRecordHeader(buf []byte) (*LogRecordHeader, int64) {
	if len(buf) < 4 {
		return nil, 0
	}

	header := &LogRecordHeader{
		crc:        binary.LittleEndian.Uint32(buf[:4]),
		recordType: buf[4],
	}

	var index = 5
	// 取出实际的keySize
	keySize, n := binary.Varint(buf[index:])
	header.keySize = uint32(keySize)
	index += n

	// 取出实际的valueSize
	valueSize, n := binary.Varint(buf[index:])
	header.valueSize = uint32(valueSize)
	index += n

	return header, int64(index)
}

func GetRecordCRC(record *LogRecord, header []byte) uint32 {
	if record == nil {
		return 0
	}

	crc := crc32.ChecksumIEEE(header)
	crc = crc32.Update(crc, crc32.IEEETable, record.Key)
	crc = crc32.Update(crc, crc32.IEEETable, record.Value)

	return crc
}
