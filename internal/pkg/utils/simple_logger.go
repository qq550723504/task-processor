// Package utils 提供工具方法
package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level  string
	Format string
}

// SetupStructuredLogger 设置结构化日志
func SetupStructuredLogger(config LoggerConfig) *logrus.Logger {
	logger := logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// 设置日志格式
	switch config.Format {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	default:
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// 设置输出
	logger.SetOutput(os.Stdout)

	return logger
}

// GetDefaultLoggerConfig 获取默认日志配置
func GetDefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:  "info",
		Format: "text",
	}
}
