package utils

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// SetupLogger 设置日志系统（配置全局logger）
// 支持通过环境变量或参数指定日志级别
func SetupLogger() *logrus.Logger {
	return SetupLoggerWithLevel("")
}

// SetupLoggerWithLevel 设置日志系统并指定日志级别
// levelStr: 日志级别字符串（debug, info, warn, error），为空则从环境变量读取
func SetupLoggerWithLevel(levelStr string) *logrus.Logger {
	// 直接使用全局logger，统一日志输出
	logger := logrus.StandardLogger()

	// 确定日志级别：优先使用参数，其次环境变量，最后默认INFO
	if levelStr == "" {
		levelStr = os.Getenv("LOG_LEVEL")
	}

	var level logrus.Level
	switch levelStr {
	case "DEBUG", "debug":
		level = logrus.DebugLevel
	case "INFO", "info", "":
		level = logrus.InfoLevel
	case "WARN", "warn", "WARNING", "warning":
		level = logrus.WarnLevel
	case "ERROR", "error":
		level = logrus.ErrorLevel
	default:
		level = logrus.InfoLevel
		logrus.Warnf("未知的日志级别: %s，使用默认级别 INFO", levelStr)
	}
	logrus.SetLevel(level)

	// 设置日志格式 - 控制台使用带颜色的格式
	logFormat := os.Getenv("LOG_FORMAT")
	var formatter logrus.Formatter
	if logFormat == "json" {
		formatter = &logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		}
	} else {
		// 默认使用文本格式，控制台带颜色
		formatter = &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			ForceColors:     true,
		}
	}
	logrus.SetFormatter(formatter)

	// 设置输出 - 默认输出到文件
	logFile := os.Getenv("LOG_FILE")
	if logFile == "" {
		// 如果没有设置LOG_FILE环境变量，使用默认路径
		logFile = "logs/app.log"
	}

	// 设置控制台输出
	logrus.SetOutput(os.Stdout)

	// 添加文件日志hook（使用hook方式，避免颜色代码写入文件）
	if err := setupFileLoggerWithHook(logger, logFile); err != nil {
		logrus.Warnf("设置文件日志失败: %v，将只输出到控制台", err)
	}

	logrus.Infof("日志系统初始化完成 (级别: %s, 输出: 控制台+%s)", level, logFile)
	logrus.Info("提示: 代码中统一使用 logrus.Info/Warn/Error 等全局方法记录日志")

	return logger
}

// setupFileLoggerWithHook 使用hook方式设置文件日志（文件不包含颜色代码）
func setupFileLoggerWithHook(logger *logrus.Logger, logFile string) error {
	// 确保日志目录存在
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 打开日志文件
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// 添加文件hook
	logger.AddHook(&FileHook{
		file: file,
		formatter: &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			DisableColors:   true, // 文件输出不使用颜色
		},
	})

	return nil
}

// FileHook 文件日志hook
type FileHook struct {
	file      *os.File
	formatter logrus.Formatter
}

// Levels 返回hook处理的日志级别
func (hook *FileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire 写入日志到文件
func (hook *FileHook) Fire(entry *logrus.Entry) error {
	line, err := hook.formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = hook.file.Write(line)
	return err
}

// WithFields 创建带字段的日志条目
func WithFields(logger *logrus.Logger, fields map[string]interface{}) *logrus.Entry {
	return logger.WithFields(logrus.Fields(fields))
}

// WithTaskContext 创建带任务上下文的日志条目
func WithTaskContext(logger *logrus.Logger, taskID, productID, platform string) *logrus.Entry {
	return logger.WithFields(logrus.Fields{
		"task_id":    taskID,
		"product_id": productID,
		"platform":   platform,
	})
}

// WithProcessorContext 创建带处理器上下文的日志条目
func WithProcessorContext(logger *logrus.Logger, processor, component string) *logrus.Entry {
	return logger.WithFields(logrus.Fields{
		"processor": processor,
		"component": component,
	})
}
