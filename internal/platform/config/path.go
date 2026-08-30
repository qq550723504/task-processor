package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolvePath resolves configPath relative to basePath unless it is absolute.
func ResolvePath(basePath, configPath string) string {
	if filepath.IsAbs(configPath) {
		return configPath
	}
	return filepath.Join(basePath, configPath)
}

// ExecutableBasePath returns the directory containing the current executable.
func ExecutableBasePath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	return filepath.Dir(executable), nil
}
