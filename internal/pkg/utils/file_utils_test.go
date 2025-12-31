package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileUtils_SaveJSONToFile(t *testing.T) {
	fileUtils := NewFileUtils()

	// 测试数据
	taskID := "test_task_123"
	jsonData := []byte(`{"test": "data", "number": 123}`)
	subDir := "test"

	// 执行保存
	err := fileUtils.SaveJSONToFile(taskID, jsonData, subDir)
	if err != nil {
		t.Fatalf("保存文件失败: %v", err)
	}

	// 验证文件是否存在
	saveDir := filepath.Join("logs", "json_data", subDir)
	files, err := os.ReadDir(saveDir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}

	found := false
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			found = true
			break
		}
	}

	if !found {
		t.Error("未找到保存的JSON文件")
	}

	// 清理测试文件
	os.RemoveAll("logs")
}

func TestFileUtils_SaveJSONToFile_EmptyTaskID(t *testing.T) {
	fileUtils := NewFileUtils()

	err := fileUtils.SaveJSONToFile("", []byte(`{"test": "data"}`), "test")
	if err == nil {
		t.Error("期望返回错误，但没有返回")
	}
}

func TestFileUtils_SaveJSONToFile_EmptyData(t *testing.T) {
	fileUtils := NewFileUtils()

	err := fileUtils.SaveJSONToFile("test_task", []byte{}, "test")
	if err == nil {
		t.Error("期望返回错误，但没有返回")
	}
}
