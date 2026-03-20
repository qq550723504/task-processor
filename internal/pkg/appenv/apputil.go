// Package apputil 提供应用级通用工具（日志、版本、实例、平台）
package appenv

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// =============================================================================
// 日志
// =============================================================================

// SetupLogger 设置默认日志记录器（info 级别）
func SetupLogger() *logrus.Logger {
	return SetupLoggerWithLevel("info")
}

// SetupLoggerWithLevel 设置指定级别的日志记录器
func SetupLoggerWithLevel(level string) *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logger.Warnf("invalid log level '%s', using info level", level)
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)
	return logger
}

// LoggingConfig 日志文件配置（与 config.LoggingConfig 保持一致，避免循环依赖）
type LoggingConfig struct {
	Level  string
	Format string
	File   string
}

// ApplyLoggingConfig 根据配置应用日志级别、格式和文件输出
// 同时保留 stdout 输出，日志会同时写到终端和文件
func ApplyLoggingConfig(logger *logrus.Logger, cfg LoggingConfig) error {
	// 应用日志级别（配置优先于启动参数）
	if cfg.Level != "" {
		level, err := logrus.ParseLevel(strings.ToLower(cfg.Level))
		if err != nil {
			logger.Warnf("配置中的日志级别无效 '%s'，保持当前级别", cfg.Level)
		} else {
			logger.SetLevel(level)
		}
	}

	// 应用日志格式
	if strings.ToLower(cfg.Format) == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// 应用日志文件输出
	if cfg.File == "" {
		return nil
	}

	// 确保目录存在
	dir := filepath.Dir(cfg.File)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败 %s: %w", dir, err)
	}

	f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败 %s: %w", cfg.File, err)
	}

	// 同时写 stdout 和文件
	logger.SetOutput(io.MultiWriter(os.Stdout, f))
	logger.Infof("日志已同时输出到文件: %s", cfg.File)
	return nil
}

// =============================================================================
// 版本
// =============================================================================

// VersionInfo 版本信息
type VersionInfo struct {
	Version   string
	BuildTime string
}

// PrintVersionInfo 打印版本信息
func PrintVersionInfo(logger *logrus.Logger, v VersionInfo) {
	logger.Infof("========================================")
	logger.Infof("Task Processor 启动")
	logger.Infof("版本: %s", v.Version)
	logger.Infof("构建时间: %s", v.BuildTime)
	logger.Infof("========================================")
}

// =============================================================================
// 实例
// =============================================================================

// InstanceInfo 实例信息
type InstanceInfo struct {
	ID        string
	PodName   string
	Namespace string
	Hostname  string
}

// GetInstanceID 获取实例ID
func GetInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// GetPodName 获取 Pod 名称（Kubernetes 环境）
func GetPodName() string {
	if name := os.Getenv("POD_NAME"); name != "" {
		return name
	}
	return GetInstanceID()
}

// GetNamespace 获取命名空间（Kubernetes 环境）
func GetNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}
	return "default"
}

// GetInstanceInfo 获取完整实例信息
func GetInstanceInfo() InstanceInfo {
	hostname, _ := os.Hostname()
	return InstanceInfo{
		ID:        GetInstanceID(),
		PodName:   GetPodName(),
		Namespace: GetNamespace(),
		Hostname:  hostname,
	}
}

// =============================================================================
// 平台
// =============================================================================

// ParsePlatformList 解析逗号分隔的平台列表字符串
func ParsePlatformList(platformsStr string) []string {
	if platformsStr == "" {
		return []string{}
	}
	parts := strings.Split(platformsStr, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// ContainsPlatform 检查平台列表是否包含指定平台（不区分大小写）
func ContainsPlatform(platforms []string, platform string) bool {
	for _, p := range platforms {
		if strings.EqualFold(p, platform) {
			return true
		}
	}
	return false
}
