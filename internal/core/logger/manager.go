// Package logger 提供统一的日志管理功能
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// LogManager 统一日志管理器
type LogManager struct {
	logger     *logrus.Logger
	level      logrus.Level
	formatter  logrus.Formatter
	outputs    []io.Writer
	config     *LogConfig
	fileWriter *rotatingFileWriter
	mutex      sync.RWMutex
}

// LogConfig 日志配置
type LogConfig struct {
	Level        string            `yaml:"level" json:"level"`                   // 日志级别: debug, info, warn, error
	Format       string            `yaml:"format" json:"format"`                 // 日志格式: json, text
	OutputFile   string            `yaml:"output_file" json:"output_file"`       // 输出文件路径
	MaxSize      int               `yaml:"max_size" json:"max_size"`             // 最大文件大小(MB)
	MaxBackups   int               `yaml:"max_backups" json:"max_backups"`       // 保留的旧日志文件数量
	MaxAge       int               `yaml:"max_age" json:"max_age"`               // 保留的旧日志文件天数
	Compress     bool              `yaml:"compress" json:"compress"`             // 是否压缩旧日志文件
	Console      bool              `yaml:"console" json:"console"`               // 是否输出到控制台
	EnableCaller bool              `yaml:"enable_caller" json:"enable_caller"`   // 是否记录调用者信息
	CallerSkip   int               `yaml:"caller_skip" json:"caller_skip"`       // 调用栈跳过层数
	ReportCaller bool              `yaml:"report_caller" json:"report_caller"`   // 是否在日志中包含文件名和行号
	SplitByLevel []LevelFileConfig `yaml:"split_by_level" json:"split_by_level"` // 按级别分文件输出
}

// DefaultLogConfig 默认日志配置
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:        "info",
		Format:       "json",
		OutputFile:   filepath.Join("tmp", "logs", "app.log"),
		MaxSize:      100,
		MaxBackups:   10,
		MaxAge:       30,
		Compress:     true,
		Console:      true,
		ReportCaller: false,
		CallerSkip:   0,
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
	formatter := createFormatter(config.Format, config.ReportCaller)
	logger.SetFormatter(formatter)

	// 设置是否报告调用者
	logger.SetReportCaller(config.ReportCaller)

	// 创建日志管理器
	lm := &LogManager{
		logger:    logger,
		level:     level,
		formatter: formatter,
		config:    config,
	}

	// 设置输出
	outputs := lm.createOutputs()
	lm.outputs = outputs
	if len(outputs) > 0 {
		logger.SetOutput(io.MultiWriter(outputs...))
	}

	// 按级别分文件输出
	if len(config.SplitByLevel) > 0 {
		hook, err := NewLevelSplitHook(config.SplitByLevel, formatter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "创建级别分文件 Hook 失败: %v\n", err)
		} else {
			logger.AddHook(hook)
		}
	}

	return lm
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

// GetRawLogger 获取底层 *logrus.Logger 实例，供需要直接操作 logger 的场景使用
func (lm *LogManager) GetRawLogger() *logrus.Logger {
	return lm.logger
}

// GetLevel 获取当前日志级别
func (lm *LogManager) GetLevel() string {
	return lm.level.String()
}

// Close 关闭日志管理器
func (lm *LogManager) Close() error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	// 关闭文件写入器
	if lm.fileWriter != nil {
		if err := lm.fileWriter.Close(); err != nil {
			return fmt.Errorf("关闭文件写入器失败: %w", err)
		}
	}

	// 如果有其他输出，确保所有日志都被写入
	for _, output := range lm.outputs {
		if closer, ok := output.(io.Closer); ok {
			// 跳过已经关闭的fileWriter
			if output == lm.fileWriter {
				continue
			}
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
func createFormatter(format string, reportCaller bool) logrus.Formatter {
	switch format {
	case "json":
		formatter := &logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		}
		if reportCaller {
			formatter.FieldMap[logrus.FieldKeyFile] = "caller"
			formatter.FieldMap[logrus.FieldKeyFunc] = "function"
		}
		return formatter
	case "text":
		formatter := &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			ForceColors:     true,
		}
		if reportCaller {
			formatter.CallerPrettyfier = func(f *runtime.Frame) (string, string) {
				return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
			}
		}
		return formatter
	default:
		return &logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		}
	}
}

// createOutputs 创建输出目标
func (lm *LogManager) createOutputs() []io.Writer {
	var outputs []io.Writer

	// 控制台输出
	if lm.config.Console {
		outputs = append(outputs, os.Stdout)
	}

	// 文件输出（带轮转）
	if lm.config.OutputFile != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(lm.config.OutputFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			// 如果创建目录失败，记录到stderr
			fmt.Fprintf(os.Stderr, "创建日志目录失败: %v\n", err)
			return outputs
		}

		// 创建带轮转的文件写入器
		fileWriter := newRotatingFileWriter(&rotatingFileConfig{
			Filename:   lm.config.OutputFile,
			MaxSize:    lm.config.MaxSize,
			MaxBackups: lm.config.MaxBackups,
			MaxAge:     lm.config.MaxAge,
			Compress:   lm.config.Compress,
		})

		lm.fileWriter = fileWriter
		outputs = append(outputs, fileWriter)
	}

	return outputs
}

// 全局日志管理器实例
var globalLogManager *LogManager

// InitGlobalLogger 初始化全局日志管理器
func InitGlobalLogger(config *LogConfig) {
	globalLogManager = NewLogManager(config)
}

// GetGlobalLogManager 获取全局日志管理器实例
func GetGlobalLogManager() *LogManager {
	if globalLogManager == nil {
		InitGlobalLogger(nil)
	}
	return globalLogManager
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
