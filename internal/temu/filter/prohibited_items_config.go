package filter

import (
	"fmt"
	"os"
	"regexp"
	"task-processor/internal/pkg/jsonx"

	"github.com/sirupsen/logrus"
)

// ConfigLoader 配置加载器
type ConfigLoader struct {
	logger     *logrus.Entry
	configPath string
}

// NewConfigLoader 创建配置加载器
func NewConfigLoader(logger *logrus.Entry, configPath string) *ConfigLoader {
	return &ConfigLoader{
		logger:     logger,
		configPath: configPath,
	}
}

// LoadConfig 加载违禁品配置
func (cl *ConfigLoader) LoadConfig() (*DetectorConfig, error) {
	data, err := os.ReadFile(cl.configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config ProhibitedItemsConfig
	if err := jsonx.UnmarshalBytes(data, &config, "解析配置文件失败"); err != nil {
		return nil, err
	}

	detectorConfig := &DetectorConfig{
		StaticKeywords:   config.StaticKeywords,
		CategoryKeywords: config.CategoryKeywords,
		DynamicPatterns:  make(map[string][]*regexp.Regexp),
	}

	// 编译动态模式
	for category, patterns := range config.DynamicPatterns {
		detectorConfig.DynamicPatterns[category] = []*regexp.Regexp{}
		for _, pattern := range patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				cl.logger.WithError(err).Warnf("编译正则表达式失败: %s", pattern)
				continue
			}
			detectorConfig.DynamicPatterns[category] = append(detectorConfig.DynamicPatterns[category], re)
		}
	}

	cl.logger.Infof("加载违禁品配置成功: 静态关键词=%d, 动态模式=%d",
		len(detectorConfig.StaticKeywords), len(detectorConfig.DynamicPatterns))

	return detectorConfig, nil
}

// LoadDefaultConfig 加载默认配置
func (cl *ConfigLoader) LoadDefaultConfig() *DetectorConfig {
	cl.logger.Info("使用默认违禁品配置")
	return &DetectorConfig{
		StaticKeywords:   make(map[string][]string),
		CategoryKeywords: make(map[string][]string),
		DynamicPatterns:  make(map[string][]*regexp.Regexp),
	}
}
