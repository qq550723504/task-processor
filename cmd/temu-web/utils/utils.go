package utils

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// SetupLogger configures the application logger
func SetupLogger() *logrus.Logger {
	logger := logrus.New()

	logFile, err := os.OpenFile("temu-processor.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logger.Fatal("创建日志文件失败:", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger.SetOutput(multiWriter)
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	return logger
}

// EnsureWorkingDirectory ensures the working directory is set correctly
func EnsureWorkingDirectory(logger *logrus.Logger) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	logger.Printf("当前工作目录: %s", cwd)

	// If currently in cmd/temu-web directory, switch to project root
	if len(cwd) > 12 && cwd[len(cwd)-12:] == "cmd/temu-web" {
		rootDir := cwd[:len(cwd)-12]
		if err := os.Chdir(rootDir); err != nil {
			return err
		}
		logger.Printf("工作目录切换到: %s", rootDir)

		// Verify the config file exists
		if _, err := os.Stat("config/config-temu-dev.yaml"); err != nil {
			logger.Printf("警告: 配置文件不存在: %v", err)
		} else {
			logger.Printf("配置文件存在: config/config-temu-dev.yaml")
		}
	} else {
		// Check if we're already in the project root
		if _, err := os.Stat("config/config-temu-dev.yaml"); err != nil {
			logger.Printf("警告: 当前目录下找不到配置文件: %v", err)
		} else {
			logger.Printf("配置文件存在: config/config-temu-dev.yaml")
		}
	}

	return nil
}

// LogEnvironmentInfo logs environment information
func LogEnvironmentInfo(logger *logrus.Logger, appVersion string) {
	logger.Printf("程序版本: %s", appVersion)

	env := os.Getenv("TASK_PROCESSOR_ENV")
	if env == "" {
		env = "dev"
	}
	logger.Printf("当前环境: %s", env)

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	logger.Printf("实例ID: %s-%d", hostname, os.Getpid())
}
