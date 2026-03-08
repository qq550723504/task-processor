// Package config 提供配置辅助函数
package config

import (
	"task-processor/internal/core/config/utils"
)

// LoadJSONConfig 从JSON文件加载配置 (向后兼容)
func LoadJSONConfig(path string, config any) error {
	return utils.LoadJSONConfig(path, config)
}

// LoadYAMLConfig 从YAML文件加载配置 (向后兼容)
func LoadYAMLConfig(path string, config any) error {
	return utils.LoadYAMLConfig(path, config)
}

// SaveJSONConfig 保存配置到JSON文件 (向后兼容)
func SaveJSONConfig(path string, config any) error {
	return utils.SaveJSONConfig(path, config)
}

// SaveYAMLConfig 保存配置到YAML文件 (向后兼容)
func SaveYAMLConfig(path string, config any) error {
	return utils.SaveYAMLConfig(path, config)
}

// ResolveConfigPath 解析配置文件路径 (向后兼容)
func ResolveConfigPath(basePath, configPath string) string {
	return utils.ResolveConfigPath(basePath, configPath)
}

// GetConfigBasePath 获取配置文件基础路径 (向后兼容)
func GetConfigBasePath() (string, error) {
	return utils.GetConfigBasePath()
}

// FileExists 检查文件是否存在 (向后兼容)
func FileExists(path string) bool {
	return utils.FileExists(path)
}

// MergeConfigs 合并两个配置 (向后兼容)
func MergeConfigs(base, override any) error {
	return utils.MergeConfigs(base, override)
}
