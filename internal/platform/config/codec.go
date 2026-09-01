package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadJSON decodes the JSON file at path into dst.
func LoadJSON(path string, dst any) error {
	data, err := readFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("解析JSON配置失败 %s: %w", path, err)
	}
	return nil
}

// LoadYAML decodes the YAML file at path into dst.
func LoadYAML(path string, dst any) error {
	data, err := readFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("解析YAML配置失败 %s: %w", path, err)
	}
	return nil
}

// SaveJSON encodes value as indented JSON at path.
func SaveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化JSON配置失败: %w", err)
	}
	return writeFile(path, data)
}

// SaveYAML encodes value as YAML at path.
func SaveYAML(path string, value any) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化YAML配置失败: %w", err)
	}
	return writeFile(path, data)
}

func readFile(path string) ([]byte, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("配置文件不存在: %s", path)
		}
		return nil, fmt.Errorf("检查配置文件失败 %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败 %s: %w", path, err)
	}
	return data, nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}
