// package fileio 提供文件操作工具
package fileio

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// FileUtil 文件操作工具
type FileUtil struct {
	logger *logrus.Entry
}

// New 创建文件操作工具实例
func New() *FileUtil {
	return &FileUtil{logger: logger.GetGlobalLogger("fileutil")}
}

// SaveJSONToFile 保存JSON数据到文件
func (f *FileUtil) SaveJSONToFile(taskID string, jsonData []byte, subDir string) error {
	if taskID == "" {
		return fmt.Errorf("任务ID不能为空")
	}
	if len(jsonData) == 0 {
		return fmt.Errorf("JSON数据不能为空")
	}

	saveDir := filepath.Join("logs", "json_data", subDir)
	if err := ensureDir(saveDir); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.json", taskID, time.Now().Format("20060102_150405"))
	filePath := filepath.Join(saveDir, filename)

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	f.logger.Infof("JSON数据已保存到文件: %s", filePath)
	return nil
}

func ensureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}
