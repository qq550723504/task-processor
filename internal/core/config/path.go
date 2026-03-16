// Package helpers 提供配置辅助工具
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveConfigPath 解析配置文件路径(支持相对路径和绝对路径)
func ResolveConfigPath(basePath, configPath string) string {
	// 如果是绝对路径,直接返回
	if filepath.IsAbs(configPath) {
		return configPath
	}

	// 相对于基础路径解析
	return filepath.Join(basePath, configPath)
}

// GetConfigBasePath 获取配置文件基础路径
func GetConfigBasePath() (string, error) {
	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 返回可执行文件所在目录
	return filepath.Dir(exePath), nil
}
