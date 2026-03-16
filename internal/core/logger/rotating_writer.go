package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// rotatingFileConfig 日志轮转配置
type rotatingFileConfig struct {
	Filename   string // 日志文件路径
	MaxSize    int    // 最大文件大小(MB)
	MaxBackups int    // 保留的旧日志文件数量
	MaxAge     int    // 保留的旧日志文件天数
	Compress   bool   // 是否压缩旧日志文件
}

// rotatingFileWriter 带轮转功能的文件写入器
type rotatingFileWriter struct {
	config *rotatingFileConfig
	file   *os.File
	size   int64
	mutex  sync.Mutex
}

// newRotatingFileWriter 创建新的轮转文件写入器
func newRotatingFileWriter(config *rotatingFileConfig) *rotatingFileWriter {
	if config.MaxSize == 0 {
		config.MaxSize = 100 // 默认100MB
	}
	if config.MaxBackups == 0 {
		config.MaxBackups = 10 // 默认保留10个备份
	}
	if config.MaxAge == 0 {
		config.MaxAge = 30 // 默认保留30天
	}

	w := &rotatingFileWriter{
		config: config,
	}

	// 打开或创建日志文件
	if err := w.openFile(); err != nil {
		fmt.Fprintf(os.Stderr, "打开日志文件失败: %v\n", err)
	}

	return w
}

// Write 实现 io.Writer 接口
func (w *rotatingFileWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// 检查是否需要轮转
	writeLen := int64(len(p))
	if w.size+writeLen > w.maxSizeBytes() {
		if rotateErr := w.rotate(); rotateErr != nil {
			return 0, fmt.Errorf("日志轮转失败: %w", rotateErr)
		}
	}

	// 写入数据
	n, err = w.file.Write(p)
	w.size += int64(n)

	return n, err
}

// Close 关闭文件写入器
func (w *rotatingFileWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// openFile 打开日志文件
func (w *rotatingFileWriter) openFile() error {
	// 获取文件信息
	info, err := os.Stat(w.config.Filename)
	if err == nil {
		w.size = info.Size()
	}

	// 打开文件（追加模式）
	file, err := os.OpenFile(w.config.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}

	w.file = file
	return nil
}

// rotate 执行日志轮转
func (w *rotatingFileWriter) rotate() error {
	// 关闭当前文件
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("关闭当前日志文件失败: %w", err)
		}
	}

	// 生成备份文件名（带时间戳）
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s.%s", w.config.Filename, timestamp)

	// 重命名当前文件
	if err := os.Rename(w.config.Filename, backupName); err != nil {
		return fmt.Errorf("重命名日志文件失败: %w", err)
	}

	// 压缩备份文件（异步）
	if w.config.Compress {
		go w.compressFile(backupName)
	}

	// 清理旧文件
	go w.cleanOldFiles()

	// 重新打开文件
	w.size = 0
	return w.openFile()
}

// compressFile 压缩文件
func (w *rotatingFileWriter) compressFile(filename string) {
	// 打开源文件
	src, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开文件失败: %v\n", err)
		return
	}
	defer src.Close()

	// 创建压缩文件
	gzFilename := filename + ".gz"
	dst, err := os.Create(gzFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建压缩文件失败: %v\n", err)
		return
	}
	defer dst.Close()

	// 创建gzip写入器
	gzWriter := gzip.NewWriter(dst)
	defer gzWriter.Close()

	// 复制数据
	if _, err := io.Copy(gzWriter, src); err != nil {
		fmt.Fprintf(os.Stderr, "压缩文件失败: %v\n", err)
		return
	}

	// 删除原文件
	if err := os.Remove(filename); err != nil {
		fmt.Fprintf(os.Stderr, "删除原文件失败: %v\n", err)
	}
}

// cleanOldFiles 清理旧的日志文件
func (w *rotatingFileWriter) cleanOldFiles() {
	dir := filepath.Dir(w.config.Filename)
	baseName := filepath.Base(w.config.Filename)

	// 获取所有备份文件
	files, err := filepath.Glob(filepath.Join(dir, baseName+".*"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "查找备份文件失败: %v\n", err)
		return
	}

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}

	var fileInfos []fileInfo
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, fileInfo{
			path:    file,
			modTime: info.ModTime(),
		})
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	// 删除超过数量限制的文件
	if w.config.MaxBackups > 0 && len(fileInfos) > w.config.MaxBackups {
		for _, fi := range fileInfos[w.config.MaxBackups:] {
			if err := os.Remove(fi.path); err != nil {
				fmt.Fprintf(os.Stderr, "删除旧日志文件失败: %v\n", err)
			}
		}
	}

	// 删除超过时间限制的文件
	if w.config.MaxAge > 0 {
		cutoff := time.Now().AddDate(0, 0, -w.config.MaxAge)
		for _, fi := range fileInfos {
			if fi.modTime.Before(cutoff) {
				if err := os.Remove(fi.path); err != nil {
					fmt.Fprintf(os.Stderr, "删除过期日志文件失败: %v\n", err)
				}
			}
		}
	}
}

// maxSizeBytes 返回最大文件大小（字节）
func (w *rotatingFileWriter) maxSizeBytes() int64 {
	return int64(w.config.MaxSize) * 1024 * 1024
}

// GetBackupFiles 获取所有备份文件列表
func (w *rotatingFileWriter) GetBackupFiles() ([]string, error) {
	dir := filepath.Dir(w.config.Filename)
	baseName := filepath.Base(w.config.Filename)

	files, err := filepath.Glob(filepath.Join(dir, baseName+".*"))
	if err != nil {
		return nil, err
	}

	// 过滤掉非备份文件
	var backups []string
	for _, file := range files {
		if strings.HasSuffix(file, ".gz") || strings.Contains(file, ".20") {
			backups = append(backups, file)
		}
	}

	return backups, nil
}
