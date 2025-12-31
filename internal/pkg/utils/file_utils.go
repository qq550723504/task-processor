// Package utils 提供通用工具方法
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// FileUtils 文件操作工具
type FileUtils struct {
	logger *logrus.Entry
}

// NewFileUtils 创建文件操作工具实例
func NewFileUtils() *FileUtils {
	return &FileUtils{
		logger: logrus.WithField("utils", "FileUtils"),
	}
}

// SaveJSONToFile 保存JSON数据到文件
func (f *FileUtils) SaveJSONToFile(taskID string, jsonData []byte, subDir string) error {
	if taskID == "" {
		return fmt.Errorf("任务ID不能为空")
	}

	if len(jsonData) == 0 {
		return fmt.Errorf("JSON数据不能为空")
	}

	// 创建保存目录
	saveDir := filepath.Join("logs", "json_data", subDir)
	if err := f.ensureDir(saveDir); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 生成文件名：任务ID_时间戳.json
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.json", taskID, timestamp)
	filePath := filepath.Join(saveDir, filename)

	// 写入文件
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		f.logger.Errorf("写入文件失败: %v", err)
		return fmt.Errorf("写入文件失败: %w", err)
	}

	f.logger.Infof("JSON数据已保存到文件: %s", filePath)
	return nil
}

// ensureDir 确保目录存在
func (f *FileUtils) ensureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		f.logger.Infof("创建目录: %s", dir)
	}
	return nil
}
