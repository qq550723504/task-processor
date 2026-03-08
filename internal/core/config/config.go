// Package config 提供配置管理功能
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"task-processor/internal/core/config/types"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config 配置结构体 (包装types.Config以支持方法)
type Config struct {
	*types.Config
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
