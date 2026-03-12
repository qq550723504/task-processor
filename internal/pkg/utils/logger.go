// Package utils 提供工具方法
package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

// SetupLogger 设置默认日志记录器
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

	// 解析日志级别
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logger.Warnf("invalid log level '%s', using info level", level)
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	return logger
}
