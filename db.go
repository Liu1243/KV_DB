package bitcask_go

import (
	"bitcask-go/data"
	"bitcask-go/index"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type DB struct {
	options    Options
	mu         *sync.RWMutex
	activeFile *data.DataFile            // 当前活跃数据文件，可以用于写入
	olderFiles map[uint32]*data.DataFile // 旧的数据文件，只能用于读取
	index      index.Indexer
	fileIds    []int // 文件id，只能在加载索引的时候使用
}

// Open 打开数据库实例
func Open(options Options) (*DB, error) {
	// 对用户传入的配置项进行校验
	if err := checkOptions(options); err != nil {
		return nil, err
	}
	// 判断数据目录是否存在，不存在则创建数据目录
	if _, err := os.Stat(options.DirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(options.DirPath, os.ModePerm); err != nil {
			return nil, err
		}
	}
	// 初始化DB实例结构体
	db := &DB{
		options:    options,
		olderFiles: make(map[uint32]*data.DataFile),
		index:      index.NewIndexer(options.IndexType),
		mu:         new(sync.RWMutex),
	}

	// 加载数据文件
	if err := db.loadDataFiles(); err != nil {
		return nil, err
	}

	// 从数据文件中加载索引
	if err := db.loadIndexFromDataFiles(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) loadIndexFromDataFiles() error {
	// 如果没有文件id，说明数据库是空的，直接返回
	if len(db.fileIds) == 0 {
		return nil
	}

	// 遍历所有文件id，处理文件中的记录
	for _, fileId := range db.fileIds {
		var fileId = uint32(fileId)
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
			var ok bool
			if logRecord.Type == data.LogRecordNormal {
				ok = db.index.Put(logRecord.Key, logRecordPos)
			} else if logRecord.Type == data.LogRecordDeleted {
				ok = db.index.Delete(logRecord.Key)
			}

			if !ok {
				return ErrIndexUpdateFailed
			}

			offset += size
		}

		// 如果当前是活跃文件，更新WriteOff
		if fileId == db.activeFile.FileId {
			db.activeFile.WriteOff = offset
		}
	}
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
		dataFile, err := data.OpenDataFile(db.options.DirPath, uint32(fileId))
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

// Put 写入Key Value，Key不能为空
func (db *DB) Put(key []byte, value []byte) error {
	// 判断key是否有效
	if len(key) == 0 {
		return ErrKeyIsEmpty
	}

	// 构造LogRecord
	logRecord := &data.LogRecord{
		Key:   key,
		Type:  data.LogRecordNormal,
		Value: value,
	}

	// 追加写入到当前活跃数据文件中
	pos, err := db.appendLogRecord(logRecord)
	if err != nil {
		return err
	}

	// 更新内存索引
	if ok := db.index.Put(logRecord.Key, pos); !ok {
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
		return ErrKeyNotFound
	}

	// 构造LogRecord 标识该key被删除
	logRecord := &data.LogRecord{
		Key:  key,
		Type: data.LogRecordDeleted,
	}
	// 写入到数据文件中
	_, err := db.appendLogRecord(logRecord)
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
	db.mu.Lock()
	defer db.mu.Unlock()

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

	// 根据用户配置决定是否需要持久化
	if db.options.SyncWrites {
		if err := db.activeFile.Sync(); err != nil {
			return nil, err
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
	dataFile, err := data.OpenDataFile(db.options.DirPath, initialFileId)
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
