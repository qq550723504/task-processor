// Package logger 提供统一的日志管理功能
package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// LogManager 统一日志管理器
type LogManager struct {
	logger    *logrus.Logger
	level     logrus.Level
	formatter logrus.Formatter
	outputs   []io.Writer
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level" json:"level"`             // 日志级别: debug, info, warn, error
	Format     string `yaml:"format" json:"format"`           // 日志格式: json, text
	OutputFile string `yaml:"output_file" json:"output_file"` // 输出文件路径
	MaxSize    int    `yaml:"max_size" json:"max_size"`       // 最大文件大小(MB)
	Console    bool   `yaml:"console" json:"console"`         // 是否输出到控制台
}

// DefaultLogConfig 默认日志配置
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:      "info",
		Format:     "json",
		OutputFile: "logs/app.log",
		MaxSize:    100,
		Console:    true,
	}
}

// NewLogManager 创建日志管理器
func NewLogManager(config *LogConfig) *LogManager {
	if config == nil {
		config = DefaultLogConfig()
	}

	logger := logrus.New()

	// 设置日志级别
	level := parseLogLevel(config.Level)
	logger.SetLevel(level)

	// 设置日志格式
	formatter := createFormatter(config.Format)
	logger.SetFormatter(formatter)

	// 设置输出
	outputs := createOutputs(config)
	if len(outputs) > 0 {
		logger.SetOutput(io.MultiWriter(outputs...))
	}

	return &LogManager{
		logger:    logger,
		level:     level,
		formatter: formatter,
		outputs:   outputs,
	}
}

// GetLogger 获取带组件标识的日志记录器
func (lm *LogManager) GetLogger(component string) *logrus.Entry {
	return lm.logger.WithField("component", component)
}

// GetLoggerWithFields 获取带多个字段的日志记录器
func (lm *LogManager) GetLoggerWithFields(fields logrus.Fields) *logrus.Entry {
	return lm.logger.WithFields(fields)
}

// SetLevel 动态设置日志级别
func (lm *LogManager) SetLevel(level string) error {
	logLevel := parseLogLevel(level)
	lm.logger.SetLevel(logLevel)
	lm.level = logLevel
	return nil
}

// GetLevel 获取当前日志级别
func (lm *LogManager) GetLevel() string {
	return lm.level.String()
}

// Close 关闭日志管理器
func (lm *LogManager) Close() error {
	// 如果有文件输出，确保所有日志都被写入
	for _, output := range lm.outputs {
		if closer, ok := output.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseLogLevel 解析日志级别
func parseLogLevel(level string) logrus.Level {
	switch level {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "fatal":
		return logrus.FatalLevel
	case "panic":
		return logrus.PanicLevel
	default:
		return logrus.InfoLevel
	}
}

// createFormatter 创建日志格式化器
func createFormatter(format string) logrus.Formatter {
	switch format {
	case "json":
		return &logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		}
	case "text":
		return &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			ForceColors:     true,
		}
	default:
		return &logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		}
	}
}

// createOutputs 创建输出目标
func createOutputs(config *LogConfig) []io.Writer {
	var outputs []io.Writer

	// 控制台输出
	if config.Console {
		outputs = append(outputs, os.Stdout)
	}

	// 文件输出
	if config.OutputFile != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(config.OutputFile)
		if err := os.MkdirAll(logDir, 0755); err == nil {
			if file, err := os.OpenFile(config.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				outputs = append(outputs, file)
			}
		}
	}

	return outputs
}

// 全局日志管理器实例
var globalLogManager *LogManager

// InitGlobalLogger 初始化全局日志管理器
func InitGlobalLogger(config *LogConfig) {
	globalLogManager = NewLogManager(config)
}

// GetGlobalLogger 获取全局日志记录器
func GetGlobalLogger(component string) *logrus.Entry {
	if globalLogManager == nil {
		InitGlobalLogger(nil) // 使用默认配置
	}
	return globalLogManager.GetLogger(component)
}

// SetGlobalLogLevel 设置全局日志级别
func SetGlobalLogLevel(level string) error {
	if globalLogManager == nil {
		InitGlobalLogger(nil)
	}
	return globalLogManager.SetLevel(level)
}
