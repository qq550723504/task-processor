// Package config 提供配置管理功能
package config

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// ValidationError 配置验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("配置验证失败 [%s]: %s", e.Field, e.Message)
}

// Validate 验证配置
func (c *Config) Validate() []error {
	var errors []error

	// 验证 Worker 配置
	errors = append(errors, c.validateWorkerConfig()...)

	// 验证 Management 配置
	errors = append(errors, c.validateManagementConfig()...)

	// 验证 OpenAI 配置
	errors = append(errors, c.validateOpenAIConfig()...)

	// 验证 Amazon 配置
	errors = append(errors, c.validateAmazonConfig()...)

	return errors
}

// validateWorkerConfig 验证工作池配置
func (c *Config) validateWorkerConfig() []error {
	var errors []error

	if c.Worker.Concurrency <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "worker.concurrency",
			Message: "并发数必须大于 0",
		})
	}

	if c.Worker.Concurrency > 100 {
		errors = append(errors, &ValidationError{
			Field:   "worker.concurrency",
			Message: "并发数不应超过 100",
		})
	}

	if c.Worker.BufferSize <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "worker.bufferSize",
			Message: "缓冲区大小必须大于 0",
		})
	}

	if c.Worker.TaskInterval <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "worker.taskInterval",
			Message: "任务获取间隔必须大于 0",
		})
	}

	return errors
}

// validateManagementConfig 验证管理系统配置
func (c *Config) validateManagementConfig() []error {
	var errors []error

	if c.Management.BaseURL == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.baseURL",
			Message: "管理系统 URL 不能为空",
		})
	}

	if c.Management.ClientID == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.clientID",
			Message: "客户端 ID 不能为空",
		})
	}

	if c.Management.ClientSecret == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.clientSecret",
			Message: "客户端密钥不能为空",
		})
	}

	if c.Management.TenantID == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.tenantID",
			Message: "租户 ID 不能为空",
		})
	}

	return errors
}

// validateOpenAIConfig 验证OpenAI配置
func (c *Config) validateOpenAIConfig() []error {
	var errors []error

	if c.OpenAI.APIKey != "" && c.OpenAI.Model == "" {
		errors = append(errors, &ValidationError{
			Field:   "openai.model",
			Message: "OpenAI 模型不能为空",
		})
	}

	return errors
}

// validateAmazonConfig 验证Amazon配置
func (c *Config) validateAmazonConfig() []error {
	var errors []error

	if !c.Amazon.Enabled {
		return errors
	}

	if c.Amazon.PoolSize <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "amazon.poolSize",
			Message: "Amazon 爬虫池大小必须大于 0",
		})
	}

	if c.Amazon.ViewportWidth <= 0 || c.Amazon.ViewportHeight <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "amazon.viewport",
			Message: "视口尺寸必须大于 0",
		})
	}

	return errors
}

// ValidateAndLog 验证配置并记录错误
func (c *Config) ValidateAndLog(logger *logrus.Logger) bool {
	errors := c.Validate()
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
func (c *Config) ValidateOrPanic() {
	errors := c.Validate()
	if len(errors) == 0 {
		return
	}

	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Error())
	}

	panic(fmt.Sprintf("配置验证失败:\n%s", strings.Join(messages, "\n")))
}
