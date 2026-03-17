// Package validators 提供配置验证功能
package config

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// Validator 配置验证器
type Validator struct {
	processor  *ProcessorConfig
	worker     *WorkerConfig
	openai     *OpenAIConfig
	management *ManagementConfig
	browser    *BrowserConfig
	amazon     *AmazonConfig
	platforms  *PlatformsConfig
}

// NewValidator 创建验证器
func NewValidator(
	processor *ProcessorConfig,
	worker *WorkerConfig,
	openai *OpenAIConfig,
	management *ManagementConfig,
	browser *BrowserConfig,
	amazon *AmazonConfig,
	platforms *PlatformsConfig,
) *Validator {
	return &Validator{
		processor:  processor,
		worker:     worker,
		openai:     openai,
		management: management,
		browser:    browser,
		amazon:     amazon,
		platforms:  platforms,
	}
}

// Validate 验证配置
func (v *Validator) Validate() []error {
	var errors []error

	// 验证各个模块配置
	errors = append(errors, ValidateWorkerConfig(v.worker)...)
	errors = append(errors, ValidateManagementConfig(v.management)...)
	errors = append(errors, ValidateOpenAIConfig(v.openai)...)
	errors = append(errors, ValidateBrowserConfig(v.browser)...)
	errors = append(errors, ValidateAmazonConfig(v.amazon)...)
	errors = append(errors, ValidatePlatformsConfig(v.platforms)...)

	return errors
}

// ValidateAndLog 验证配置并记录错误
func (v *Validator) ValidateAndLog(logger *logrus.Logger) bool {
	errors := v.Validate()
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
func (v *Validator) ValidateWithError() error {
	errors := v.Validate()
	if len(errors) == 0 {
		return nil
	}

	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Error())
	}

	return fmt.Errorf("config validation failed:\n%s", strings.Join(messages, "\n"))
}
