// Package helpers 提供配置辅助工具
package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadJSONConfig 从JSON文件加载配置
func LoadJSONConfig(path string, config any) error {
	// 检查文件是否存在
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("配置文件不存在: %s", path)
		}
		return fmt.Errorf("检查配置文件失败 %s: %w", path, err)
	}

	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败 %s: %w", path, err)
	}

	// 解析JSON
	if err := json.Unmarshal(data, config); err != nil {
		return fmt.Errorf("解析JSON配置失败 %s: %w", path, err)
	}

	return nil
}

// LoadYAMLConfig 从YAML文件加载配置
func LoadYAMLConfig(path string, config any) error {
	// 检查文件是否存在
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("配置文件不存在: %s", path)
		}
		return fmt.Errorf("检查配置文件失败 %s: %w", path, err)
	}

	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败 %s: %w", path, err)
	}

	// 解析YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("解析YAML配置失败 %s: %w", path, err)
	}

	return nil
}

// SaveJSONConfig 保存配置到JSON文件
func SaveJSONConfig(path string, config any) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化JSON配置失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// SaveYAMLConfig 保存配置到YAML文件
func SaveYAMLConfig(path string, config any) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 序列化为YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化YAML配置失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
