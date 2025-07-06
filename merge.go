package bitcask_go

import (
	"bitcask-go/data"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
)

const (
	mergeDirName     = "-merge"
	mergeFinishedKey = "merge.finished"
)

// Merge 清理无效数据 生成Hint文件
func (db *DB) Merge() error {
	// 如果数据库为空 直接返回
	if db.activeFile == nil {
		return nil
	}

	db.mu.Lock()
	// 如果merge正在进行中 直接返回
	if db.isMerge {
		db.mu.Unlock()
		return ErrMergeInProgress
	}
	db.isMerge = true
	defer func() {
		db.isMerge = false
	}()

	// 持久化当前活跃文件
	err := db.activeFile.Sync()
	if err != nil {
		db.mu.Unlock()
		return err
	}

	// 将当前活跃文件转换为旧数据文件
	db.olderFiles[db.activeFile.FileId] = db.activeFile
	// 打开新活跃文件
	if err := db.setActiveDataFile(); err != nil {
		db.mu.Unlock()
		return err
	}

	// 记录最新的没有参与merge的文件id 用作后续系统启动时候使用
	nonMergeFileId := db.activeFile.FileId

	// 取出所有需要merge的文件
	var mergeFiles []*data.DataFile
	for _, file := range db.olderFiles {
		mergeFiles = append(mergeFiles, file)
	}
	db.mu.Unlock()

	// 将待merge的文件进行排序
	sort.Slice(mergeFiles, func(i, j int) bool {
		return mergeFiles[i].FileId < mergeFiles[j].FileId
	})
	mergePath := db.getMergePath()
	// 如果目录存在 说明发生过merge 将其删除
	if _, err = os.Stat(mergePath); err == nil {
		if err := os.RemoveAll(mergePath); err != nil {
			return err
		}
	}
	// 新建一个merge path目录
	if err := os.MkdirAll(mergePath, os.ModePerm); err != nil {
		return err
	}

	// 打开一个临时bitcask实例
	mergeoptions := db.options
	mergeoptions.DirPath = mergePath
	mergeoptions.SyncWrites = false
	mergeDB, err := Open(mergeoptions)
	if err != nil {
		return err
	}

	// 打开hint文件存储索引
	hintFile, err := data.OpenHintFile(mergePath)
	if err != nil {
		return err
	}
	// 遍历处理每个数据文件
	for _, dataFile := range mergeFiles {
		var offset int64 = 0
		for {
			logRecord, size, err := dataFile.ReadLogRecord(offset)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			// 解析拿到实际的key
			realKey, _ := parseLogRecordKey(logRecord.Key)
			logRecordPos := db.index.Get(realKey)
			// 和内存中索引位置进行比较，如果有效则重写
			if logRecordPos != nil &&
				logRecordPos.Fid == dataFile.FileId &&
				logRecordPos.Offset == offset {
				// 清除事务标记
				logRecord.Key = logRecordKeyWithSeq(realKey, nonTransactionSeqNo)
				pos, err := mergeDB.appendLogRecord(logRecord)
				if err != nil {
					return err
				}
				// 将当前位置索引写入到Hint文件当中
				if err := hintFile.WriteHintRecord(realKey, pos); err != nil {
					return err
				}
			}

			// 增加offset
			offset += size
		}
	}

	// sync 保证持久化
	if err := hintFile.Sync(); err != nil {
		return err
	}
	if err := mergeDB.Sync(); err != nil {
		return err
	}

	// 写标识merge完成的文件
	mergeFinishedFile, err := data.OpenMergeFinishedFile(mergePath)
	if err != nil {
		return err
	}
	// record中存入最新未merge的活动文件id
	mergeFinRecord := &data.LogRecord{
		Key:   []byte(mergeFinishedKey),
		Value: []byte(strconv.Itoa(int(nonMergeFileId))),
	}
	encRecord, _ := data.EncodeLogRecord(mergeFinRecord)
	if err := mergeFinishedFile.Write(encRecord); err != nil {
		return err
	}
	if err := mergeFinishedFile.Sync(); err != nil {
		return err
	}

	return nil
}

func (db *DB) getMergePath() string {
	dir := path.Dir(path.Clean(db.options.DirPath))
	base := path.Base(path.Clean(db.options.DirPath))
	return filepath.Join(dir, base+mergeDirName)
}

// 加载merge数据目录
func (db *DB) loadMergeFiles() error {
	mergePath := db.getMergePath()
	// merge目录不存在直接返回
	if _, err := os.Stat(mergePath); os.IsNotExist(err) {
		return nil
	}
	defer func() {
		_ = os.RemoveAll(mergePath)
	}()

	dirEntries, err := os.ReadDir(mergePath)
	if err != nil {
		return err
	}

	// 查找标识merge完成的文件，判断merge是否完成
	var mergeFinished bool
	var mergeFileNames []string
	for _, entry := range dirEntries {
		if entry.Name() == data.MergeFinishedFileName {
			mergeFinished = true
		}
		mergeFileNames = append(mergeFileNames, entry.Name())
	}
	// 没有merge完成则直接返回
	if !mergeFinished {
		return nil
	}

	// 获取最新未merge的文件id
	nonMergeFileId, err := db.getNonMergeFileId(mergePath)
	if err != nil {
		return nil
	}

	// 删除旧的数据文件
	var fileId uint32 = 0
	for ; fileId < nonMergeFileId; fileId++ {
		fileName := data.GetDataFileName(db.options.DirPath, fileId)
		if _, err := os.Stat(fileName); err == nil {
			if err := os.Remove(fileName); err != nil {
				return err
			}
		}
	}
	// 将新的数据文件移动到数据目录中
	for _, fileName := range mergeFileNames {
		srcPath := filepath.Join(mergePath, fileName)
		destPath := filepath.Join(db.options.DirPath, fileName)
		if err := os.Rename(srcPath, destPath); err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) getNonMergeFileId(dirPath string) (uint32, error) {
	mergeFinishedFile, err := data.OpenMergeFinishedFile(dirPath)
	if err != nil {
		return 0, err
	}
	record, _, err := mergeFinishedFile.ReadLogRecord(0)
	if err != nil {
		return 0, err
	}
	nonMergeFileId, err := strconv.Atoi(string(record.Value))
	if err != nil {
		return 0, err
	}
	return uint32(nonMergeFileId), nil
}

// 从hint文件中加载索引
func (db *DB) loadIndexFromHintFile() error {
	// 查看hint索引文件是否存在
	hintFileName := filepath.Join(db.options.DirPath, data.HintFileName)
	if _, err := os.Stat(hintFileName); os.IsNotExist(err) {
		return nil
	}

	// 打开hint索引文件
	hintFile, err := data.OpenHintFile(hintFileName)
	if err != nil {
		return err
	}

	// 读取文件中的索引
	var offset int64 = 0
	for {
		record, size, err := hintFile.ReadLogRecord(offset)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		// 解码拿到实际的位置索引
		pos := data.DecodeLogRecordPos(record.Value)
		db.index.Put(record.Key, pos)
		offset += size
	}
	return nil
}
