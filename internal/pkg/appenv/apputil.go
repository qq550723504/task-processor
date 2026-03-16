// Package apputil 提供应用级通用工具（日志、版本、实例、平台）
package appenv

import (
	"os"
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
