// Package config 提供配置管理功能
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"task-processor/internal/pkg/watermark"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config 主配置结构体
type Config struct {
	Logging    LoggingConfig     `yaml:"logging"`
	Processor  ProcessorConfig   `yaml:"processor"`
	Worker     WorkerConfig      `yaml:"worker"`
	OpenAI     OpenAIConfig      `yaml:"openai"`
	Management ManagementConfig  `yaml:"management"`
	Browser    BrowserConfig     `yaml:"browser"`
	Amazon     AmazonConfig      `yaml:"amazon"`
	RabbitMQ   *RabbitMQConfig   `yaml:"rabbitmq"`
	Updater    UpdaterConfig     `yaml:"updater"`
	Platforms  PlatformsConfig   `yaml:"platforms"`
	Watermark  *watermark.Config `yaml:"watermark"`
	Database   *DatabaseConfig   `yaml:"database"`
	Redis      *RedisConfig      `yaml:"redis"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

// LoadConfig 加载配置
func LoadConfig() *Config {
	env := os.Getenv("TASK_PROCESSOR_ENV")
	if env == "" {
		env = "dev"
	}

	// 加载统一配置文件 config-dev.yaml
	configName := fmt.Sprintf("config-%s", env)
	logrus.Infof("加载配置文件: %s.yaml", configName)

	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")

	// 获取可执行文件所在目录
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		viper.AddConfigPath(filepath.Join(exeDir, "config"))
		viper.AddConfigPath(exeDir)
	}

	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/task-processor/")

	viper.SetEnvPrefix("TASK_PROCESSOR")
	viper.AutomaticEnv()

	// 设置默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		logrus.Warnf("无法读取配置文件: %v", err)
		logrus.Info("使用默认配置和环境变量")
	} else {
		logrus.Infof("成功加载配置文件: %s", viper.ConfigFileUsed())
	}

	return buildConfig()
}

// LoadConfigFromFile 从指定文件加载配置
func LoadConfigFromFile(configFile string) *Config {
	logrus.Infof("加载指定配置文件: %s", configFile)

	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	viper.SetEnvPrefix("TASK_PROCESSOR")
	viper.AutomaticEnv()

	// 设置默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		logrus.Warnf("无法读取配置文件 %s: %v", configFile, err)
		logrus.Info("使用默认配置和环境变量")
	} else {
		logrus.Infof("成功加载配置文件: %s", viper.ConfigFileUsed())
	}

	return buildConfig()
}
