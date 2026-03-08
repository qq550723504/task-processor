// Package config 提供配置管理功能
package config

import (
	"fmt"
	"strings"

	"task-processor/internal/core/config/validators"

	"github.com/sirupsen/logrus"
)

// ValidationError 配置验证错误 (向后兼容)
type ValidationError = validators.ValidationError

// Validate 验证配置
func (c *Config) Validate() []error {
	return ValidateConfig(c)
}

// ValidateConfig 验证配置(函数版本)
func ValidateConfig(c *Config) []error {
	validator := validators.NewValidator(
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

// ValidateOrPanic 验证配置，如果失败则 panic
// Deprecated: Use ValidateWithError() instead and handle the error properly
func (c *Config) ValidateOrPanic() {
	ValidateConfigOrPanic(c)
}

// ValidateConfigOrPanic 验证配置，如果失败则 panic(函数版本)
// Deprecated: Use ValidateConfigWithError() instead and handle the error properly
func ValidateConfigOrPanic(c *Config) {
	if err := ValidateConfigWithError(c); err != nil {
		panic(err)
	}
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
