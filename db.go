package bitcask_go

import (
	"bitcask-go/data"
	"bitcask-go/fio"
	"bitcask-go/index"
	"fmt"
	"github.com/gofrs/flock"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	seqNoKey     = "seq.no"
	fileLockName = "flock"
)

type DB struct {
	options         Options
	mu              *sync.RWMutex
	activeFile      *data.DataFile            // 当前活跃数据文件，可以用于写入
	olderFiles      map[uint32]*data.DataFile // 旧的数据文件，只能用于读取
	index           index.Indexer
	fileIds         []int        // 文件id，只能在加载索引的时候使用
	seqNo           uint64       // 事务序列号，全局递增
	isMerge         bool         // 是否正在合并
	seqNoFileExists bool         // 存储事务序列号的文件是否存在
	isInitial       bool         // 是否是第一次初始化此数据目录
	fileLock        *flock.Flock // 文件锁保证多进程之间的互斥
	bytesWrite      uint         // 累计写了多少字节
}

// Open 打开数据库实例
func Open(options Options) (*DB, error) {
	// 对用户传入的配置项进行校验
	if err := checkOptions(options); err != nil {
		return nil, err
	}
	var isInitial bool
	// 判断数据目录是否存在，不存在则创建数据目录
	if _, err := os.Stat(options.DirPath); os.IsNotExist(err) {
		isInitial = true
		if err := os.MkdirAll(options.DirPath, os.ModePerm); err != nil {
			return nil, err
		}
	}

	// 判断当前数据目录是否正在使用
	fileLock := flock.New(filepath.Join(options.DirPath, fileLockName))
	hold, err := fileLock.TryLock()
	if err != nil {
		return nil, err
	}
	if !hold {
		return nil, ErrDatabaseIsUsing
	}

	entries, err := os.ReadDir(options.DirPath)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		isInitial = true
	}

	// 初始化DB实例结构体
	db := &DB{
		options:    options,
		olderFiles: make(map[uint32]*data.DataFile),
		index:      index.NewIndexer(options.IndexType, options.DirPath, options.SyncWrites),
		mu:         new(sync.RWMutex),
		isInitial:  isInitial,
		fileLock:   fileLock,
	}

	// 加载merge数据目录
	if err := db.loadMergeFiles(); err != nil {
		return nil, err
	}

	// 加载数据文件
	if err := db.loadDataFiles(); err != nil {
		return nil, err
	}

	if options.IndexType != BPlusTree {
		// 从hint索引文件中加载索引
		if err := db.loadIndexFromHintFile(); err != nil {
			return nil, err
		}

		// 从数据文件中加载索引
		if err := db.loadIndexFromDataFiles(); err != nil {
			return nil, err
		}
	}

	// 重置IO类型为标准文件IO
	if db.options.MMapAtStartup {
		if err := db.resetIoType(); err != nil {
			return nil, err
		}
	}

	// 当索引结构为B+树时，需要取出当前事务序列号
	if options.IndexType == BPlusTree {
		if err := db.loadSeqNo(); err != nil {
			return nil, err
		}
		if db.activeFile != nil {
			size, err := db.activeFile.IOManager.Size()
			if err != nil {
				return nil, err
			}
			db.activeFile.WriteOff = size
		}
	}

	return db, nil
}

// Close 关闭数据库
func (db *DB) Close() error {
	defer func() {
		err := db.fileLock.Unlock()
		if err != nil {
			panic(fmt.Sprintf("fail to unlock the directory, %v", err))
		}
	}()

	if db.activeFile == nil {
		return nil
	}
	db.mu.Lock()
	defer db.mu.Unlock()

	// 关闭索引
	if err := db.index.Close(); err != nil {
		return err
	}

	// 保存当前事务序列号
	seqNoFile, err := data.OpenSeqNoFile(db.options.DirPath)
	if err != nil {
		return err
	}
	record := &data.LogRecord{
		Key:   []byte(seqNoKey),
		Value: []byte(strconv.FormatUint(db.seqNo, 10)),
	}

	encRecord, _ := data.EncodeLogRecord(record)
	if err := seqNoFile.Write(encRecord); err != nil {
		return err
	}
	if err := seqNoFile.Sync(); err != nil {
		return err
	}

	// 关闭当前活跃文件
	if err := db.activeFile.Close(); err != nil {
		return err
	}

	// 关闭旧的数据文件
	for _, dataFile := range db.olderFiles {
		if err := dataFile.Close(); err != nil {
			return err
		}
	}

	return nil
}

// Sync 持久化数据文件
func (db *DB) Sync() error {
	if db.activeFile == nil {
		return nil
	}
	db.mu.Lock()
	defer db.mu.Unlock()

	return db.activeFile.Sync()
}

func (db *DB) loadIndexFromDataFiles() error {
	// 如果没有文件id，说明数据库是空的，直接返回
	if len(db.fileIds) == 0 {
		return nil
	}

	// 判断是否发生过merge
	hasMerge, nonMergeFileId := false, uint32(0)
	mergeFinFileName := filepath.Join(db.options.DirPath, data.MergeFinishedFileName)
	if _, err := os.Stat(mergeFinFileName); err == nil {
		fid, err := db.getNonMergeFileId(db.options.DirPath)
		if err != nil {
			return err
		}
		hasMerge = true
		nonMergeFileId = fid
	}

	updateIndex := func(key []byte, typ data.LogRecordType, pos *data.LogRecordPos) {
		var ok bool
		if typ == data.LogRecordDeleted {
			ok = db.index.Delete(key)
		} else {
			ok = db.index.Put(key, pos)
		}
		if !ok {
			panic("failed to update index at startup")
		}
	}

	// 暂存所有事务数据
	transactionRecords := make(map[uint64][]*data.TransactionRecord)
	var currentSeqNo = nonTransactionSeqNo

	// 遍历所有文件id，处理文件中的记录
	for _, fileId := range db.fileIds {
		var fileId = uint32(fileId)

		// 如果比最近未参与merge的fid更小，说明已经从hint文件中加载索引了
		if hasMerge && fileId < nonMergeFileId {
			continue
		}

		var dataFile *data.DataFile

		if fileId == db.activeFile.FileId {
			dataFile = db.activeFile
		} else {
			dataFile = db.olderFiles[fileId]
		}

		var offset int64 = 0
		for {
			logRecord, size, err := dataFile.ReadLogRecord(offset)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			// 构造内存索引并保存
			logRecordPos := &data.LogRecordPos{
				Fid:    fileId,
				Offset: offset,
			}

			// 解析key 拿到事务序列号
			realKey, seqNo := parseLogRecordKey(logRecord.Key)
			if seqNo == nonTransactionSeqNo {
				// 非事务操作 直接更新内存索引
				db.index.Put(realKey, logRecordPos)
			} else {
				// 事务完成，对应的seqNo的数据就可以更新到内存索引中
				if logRecord.Type == data.LogRecordTxnFinished {
					for _, txnRecord := range transactionRecords[seqNo] {
						updateIndex(txnRecord.Record.Key, txnRecord.Record.Type, txnRecord.Pos)
					}
					delete(transactionRecords, seqNo)
				} else {
					logRecord.Key = realKey
					transactionRecords[seqNo] = append(transactionRecords[seqNo], &data.TransactionRecord{
						Record: logRecord,
						Pos:    logRecordPos,
					})
				}
			}

			// 更新事务序列号
			if seqNo > currentSeqNo {
				currentSeqNo = seqNo
			}

			offset += size
		}

		// 如果当前是活跃文件，更新WriteOff
		if fileId == db.activeFile.FileId {
			db.activeFile.WriteOff = offset
		}
	}

	// 更新事务序列号
	db.seqNo = currentSeqNo

	return nil
}

// loadDataFiles 从磁盘中加载数据文件
func (db *DB) loadDataFiles() error {
	// 遍历目录中所有文件，找到所有以.data结尾的数据文件
	files, err := os.ReadDir(db.options.DirPath)
	if err != nil {
		return err
	}

	var fileIds []int

	for _, file := range files {
		if filepath.Ext(file.Name()) != data.DataFileNameSuffix {
			continue
		}
		splitNames := strings.Split(file.Name(), ".")
		fileId, err := strconv.Atoi(splitNames[0])
		// 数据目录损坏
		if err != nil {
			return ErrDataDirectoryCorrupted
		}
		fileIds = append(fileIds, fileId)
	}
	// 对文件id进行排序，从小到大依次加载
	sort.Ints(fileIds)
	db.fileIds = fileIds

	// 遍历每个文件id，打开对应的数据文件
	for i, fileId := range fileIds {
		ioType := fio.StandardFIO
		if db.options.MMapAtStartup {
			ioType = fio.MemoryMap
		}
		dataFile, err := data.OpenDataFile(db.options.DirPath, uint32(fileId), ioType)
		if err != nil {
			return err
		}
		if i == len(fileIds)-1 { // 最后一个，id是最大的，说明是当前活跃文件
			db.activeFile = dataFile
		} else { // 说明是旧的数据文件
			db.olderFiles[uint32(fileId)] = dataFile
		}
	}

	return nil
}

func checkOptions(options Options) error {
	if options.DirPath == "" {
		return ErrDirPathIsEmpty
	}
	if options.DataFileSize <= 0 {
		return ErrDataFileSizeInvalid
	}
	return nil
}

func (db *DB) appendLogRecordWithLock(LogRecord *data.LogRecord) (*data.LogRecordPos, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.appendLogRecord(LogRecord)
}

// Put 写入Key Value，Key不能为空
func (db *DB) Put(key []byte, value []byte) error {
	// 判断key是否有效
	if len(key) == 0 {
		return ErrKeyIsEmpty
	}

	// 构造LogRecord
	logRecord := &data.LogRecord{
		Key:   logRecordKeyWithSeq(key, nonTransactionSeqNo),
		Type:  data.LogRecordNormal,
		Value: value,
	}

	// 追加写入到当前活跃数据文件中
	pos, err := db.appendLogRecordWithLock(logRecord)
	if err != nil {
		return err
	}

	// 更新内存索引
	if ok := db.index.Put(key, pos); !ok {
		return ErrIndexUpdateFailed
	}

	return nil
}

func (db *DB) Delete(key []byte) error {
	// 判断key的有效性
	if len(key) == 0 {
		return ErrKeyIsEmpty
	}

	// 先检查key是否存在 不存在直接返回
	if pos := db.index.Get(key); pos == nil {
		return nil
	}

	// 构造LogRecord 标识该key被删除
	logRecord := &data.LogRecord{
		Key:  logRecordKeyWithSeq(key, nonTransactionSeqNo),
		Type: data.LogRecordDeleted,
	}
	// 写入到数据文件中
	_, err := db.appendLogRecordWithLock(logRecord)
	if err != nil {
		return err
	}
	// 从内存索引中将key删除
	if ok := db.index.Delete(key); !ok {
		return ErrIndexUpdateFailed
	}

	return nil
}

// Put 追加写入到活跃数据文件中
func (db *DB) appendLogRecord(logRecord *data.LogRecord) (*data.LogRecordPos, error) {
	// 判断当前活跃数据文件是否存在（数据库在没有写入的情况下没有文件生成）
	// 如果为空则初始化数据文件
	if db.activeFile == nil {
		err := db.setActiveDataFile()
		if err != nil {
			return nil, err
		}
	}

	// 编码后写入
	encRecord, size := data.EncodeLogRecord(logRecord)
	// 如果写入的长度达到了活跃文件的阈值，关闭活跃文件，打开新的文件
	if db.activeFile.WriteOff+int64(size) >= db.options.DataFileSize {
		// 先持久化数据文件，保证已有的数据持久化到磁盘中
		err := db.activeFile.Sync()
		if err != nil {
			return nil, err
		}

		// 当前活跃文件转换为旧的数据文件
		db.olderFiles[db.activeFile.FileId] = db.activeFile

		// 打开新的数据文件
		err = db.setActiveDataFile()
		if err != nil {
			return nil, err
		}
	}
	writeOff := db.activeFile.WriteOff
	if err := db.activeFile.Write(encRecord); err != nil {
		return nil, err
	}

	db.bytesWrite += uint(size)
	// 根据用户配置决定是否需要持久化
	var needSync = db.options.SyncWrites
	if !needSync && db.options.BytesPerSync > 0 && db.bytesWrite >= db.options.BytesPerSync {
		needSync = true
	}

	if needSync {
		if err := db.activeFile.Sync(); err != nil {
			return nil, err
		}
		// 清空累计值
		if db.bytesWrite > 0 {
			db.bytesWrite = 0
		}
	}

	// 构造内存索引信息
	pos := &data.LogRecordPos{
		Fid:    db.activeFile.FileId,
		Offset: writeOff,
	}

	return pos, nil
}

// 设置当前活跃文件 使用该方法需要持有互斥锁
func (db *DB) setActiveDataFile() error {
	var initialFileId uint32 = 0
	if db.activeFile != nil {
		initialFileId = db.activeFile.FileId + 1
	}

	// 打开新的数据文件
	dataFile, err := data.OpenDataFile(db.options.DirPath, initialFileId, fio.StandardFIO)
	if err != nil {
		return err
	}
	db.activeFile = dataFile
	return nil
}

func (db *DB) Get(key []byte) ([]byte, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// 判断Key的合法性
	if len(key) == 0 {
		return nil, ErrKeyIsEmpty
	}

	// 从内存索引中获取Key对应的索引信息
	logRecordPos := db.index.Get(key)
	// 如果Key不在内存索引中 说明Key不存在
	if logRecordPos == nil {
		return nil, ErrKeyNotFound
	}

	return db.getValueByPosition(logRecordPos)
}

// ListKeys 获取数据库中所有key
func (db *DB) ListKeys() [][]byte {
	iterator := db.index.Iterator(false)
	defer iterator.Close()
	keys := make([][]byte, db.index.Size())

	var idx int
	for iterator.Rewind(); iterator.Valid(); iterator.Next() {
		keys[idx] = iterator.Key()
		idx++
	}

	return keys
}

// Fold 获取所有数据，执行用户指定的fn，函数返回false时终止遍历
func (db *DB) Fold(fn func(key []byte, value []byte) bool) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	iterator := db.index.Iterator(false)
	defer iterator.Close()
	for iterator.Rewind(); iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		value, err := db.getValueByPosition(iterator.Value())
		if err != nil {
			return err
		}
		if !fn(key, value) {
			break
		}
	}

	return nil
}

func (db *DB) getValueByPosition(logRecordPos *data.LogRecordPos) ([]byte, error) {
	// 根据FileId找到对应的数据文件
	var dataFile *data.DataFile
	if db.activeFile.FileId == logRecordPos.Fid {
		dataFile = db.activeFile
	} else {
		dataFile = db.olderFiles[logRecordPos.Fid]
	}
	// 数据文件为空
	if dataFile == nil {
		return nil, ErrDataFileNotFound
	}
	// 根据偏移读取对应的数据
	logRecord, _, err := dataFile.ReadLogRecord(logRecordPos.Offset)
	if err != nil {
		return nil, err
	}

	if logRecord.Type == data.LogRecordDeleted {
		return nil, ErrKeyNotFound
	}

	return logRecord.Value, nil
}

func (db *DB) loadSeqNo() error {
	fileName := filepath.Join(db.options.DirPath, data.SeqNoFileName)
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil
	}

	seqNoFile, err := data.OpenSeqNoFile(db.options.DirPath)
	if err != nil {
		return err
	}
	record, _, err := seqNoFile.ReadLogRecord(0)
	seqNo, err := strconv.ParseUint(string(record.Value), 10, 64)
	if err != nil {
		return err
	}
	db.seqNo = seqNo
	db.seqNoFileExists = true

	// 同样的 在windows下 os.remove会panic
	//return os.Remove(fileName)
	return nil
}

func (db *DB) resetIoType() error {
	if db.activeFile == nil {
		return nil
	}

	if err := db.activeFile.SetIOManager(db.options.DirPath, fio.StandardFIO); err != nil {
		return err
	}

	for _, dataFile := range db.olderFiles {
		if err := dataFile.SetIOManager(db.options.DirPath, fio.StandardFIO); err != nil {
			return err
		}
	}
	return nil
}
