package fileio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileUtil_SaveJSONToFile(t *testing.T) {
	fu := New()

	taskID := "test_task_123"
	jsonData := []byte(`{"test": "data", "number": 123}`)
	subDir := "test"

	err := fu.SaveJSONToFile(taskID, jsonData, subDir)
	if err != nil {
		t.Fatalf("保存文件失败: %v", err)
	}

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

	os.RemoveAll("logs")
}

func TestFileUtil_SaveJSONToFile_EmptyTaskID(t *testing.T) {
	fu := New()
	err := fu.SaveJSONToFile("", []byte(`{"test": "data"}`), "test")
	if err == nil {
		t.Error("期望返回错误，但没有返回")
	}
}

func TestFileUtil_SaveJSONToFile_EmptyData(t *testing.T) {
	fu := New()
	err := fu.SaveJSONToFile("test_task", []byte{}, "test")
	if err == nil {
		t.Error("期望返回错误，但没有返回")
	}
}
