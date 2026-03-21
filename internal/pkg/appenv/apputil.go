// Package apputil 提供应用级通用工具（日志、版本、实例、平台）
package appenv

import (
	"os"
	"strings"

	loggerPkg "task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// =============================================================================
// 日志
// =============================================================================

// SetupLogger 初始化全局日志管理器（info 级别），返回底层 *logrus.Logger。
// 业务代码通过 logger.GetGlobalLogger(component) 获取带组件标识的 Entry。
func SetupLogger() *logrus.Logger {
	return SetupLoggerWithLevel("info")
}

// SetupLoggerWithLevel 以指定级别初始化全局日志管理器，返回底层 *logrus.Logger。
func SetupLoggerWithLevel(level string) *logrus.Logger {
	loggerPkg.InitGlobalLogger(&loggerPkg.LogConfig{
		Level:   level,
		Format:  "text",
		Console: true,
	})
	return loggerPkg.GetGlobalLogManager().GetRawLogger()
}

// LoggingConfig 日志配置（从 config.LoggingConfig 映射而来，避免循环依赖）
type LoggingConfig struct {
	Level        string
	Format       string
	File         string
	SplitByLevel []loggerPkg.LevelFileConfig
}

// ApplyLoggingConfig 用配置文件中的设置重新配置全局日志管理器。
// 必须在 SetupLogger/SetupLoggerWithLevel 之后调用。
func ApplyLoggingConfig(log *logrus.Logger, cfg LoggingConfig) error {
	lc := &loggerPkg.LogConfig{
		Level:        cfg.Level,
		Format:       cfg.Format,
		OutputFile:   cfg.File,
		Console:      true,
		MaxSize:      100,
		MaxBackups:   10,
		MaxAge:       30,
		Compress:     true,
		SplitByLevel: cfg.SplitByLevel,
	}
	loggerPkg.InitGlobalLogger(lc)

	// 让调用方持有的 *logrus.Logger 指针指向同一个实例
	raw := loggerPkg.GetGlobalLogManager().GetRawLogger()
	if log != raw {
		// 同步调用方 logger 的级别与格式，使其行为一致
		log.SetLevel(raw.Level)
		log.SetFormatter(raw.Formatter)
		log.SetOutput(raw.Out)
		for _, h := range raw.Hooks {
			for _, hook := range h {
				log.AddHook(hook)
			}
		}
	}
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
