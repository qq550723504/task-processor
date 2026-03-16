// Package config 提供配置管理功能
package config

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// Validate 验证配置
func (c *Config) Validate() []error {
	return ValidateConfig(c)
}

// ValidateConfig 验证配置(函数版本)
func ValidateConfig(c *Config) []error {
	validator := NewValidator(
		&c.Processor,
		&c.Worker,
		&c.OpenAI,
		&c.Management,
		&c.Browser,
		&c.Amazon,
		&c.Platforms,
	)
	return validator.Validate()
}

// ValidateAndLog 验证配置并记录错误
func (c *Config) ValidateAndLog(logger *logrus.Logger) bool {
	return ValidateConfigAndLog(c, logger)
}

// ValidateConfigAndLog 验证配置并记录错误(函数版本)
func ValidateConfigAndLog(c *Config, logger *logrus.Logger) bool {
	errors := ValidateConfig(c)
	if len(errors) == 0 {
		logger.Info("✅ 配置验证通过")
		return true
	}

	logger.Error("❌ 配置验证失败:")
	for _, err := range errors {
		logger.Error("  - " + err.Error())
	}

	return false
}

// ValidateWithError 验证配置并返回错误
func (c *Config) ValidateWithError() error {
	return ValidateConfigWithError(c)
}

// ValidateConfigWithError 验证配置并返回错误(函数版本)
func ValidateConfigWithError(c *Config) error {
	errors := ValidateConfig(c)
	if len(errors) == 0 {
		return nil
	}

	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Error())
	}

	return fmt.Errorf("config validation failed:\n%s", strings.Join(messages, "\n"))
}
