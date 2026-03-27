package config

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

type Validator struct {
	processor  *ProcessorConfig
	worker     *WorkerConfig
	openai     *OpenAIConfig
	management *ManagementConfig
	browser    *BrowserConfig
	amazon     *AmazonConfig
	rabbitmq   *RabbitMQConfig
	platforms  *PlatformsConfig
}

func NewValidator(
	processor *ProcessorConfig,
	worker *WorkerConfig,
	openai *OpenAIConfig,
	management *ManagementConfig,
	browser *BrowserConfig,
	amazon *AmazonConfig,
	rabbitmq *RabbitMQConfig,
	platforms *PlatformsConfig,
) *Validator {
	return &Validator{
		processor:  processor,
		worker:     worker,
		openai:     openai,
		management: management,
		browser:    browser,
		amazon:     amazon,
		rabbitmq:   rabbitmq,
		platforms:  platforms,
	}
}

func (v *Validator) Validate() []error {
	var errors []error

	errors = append(errors, ValidateWorkerConfig(v.worker)...)
	errors = append(errors, ValidateManagementConfig(v.management)...)
	errors = append(errors, ValidateOpenAIConfig(v.openai)...)
	errors = append(errors, ValidateBrowserConfig(v.browser)...)
	errors = append(errors, ValidateAmazonConfig(v.amazon)...)
	errors = append(errors, ValidateRabbitMQConfig(v.rabbitmq)...)
	errors = append(errors, ValidatePlatformsConfig(v.platforms)...)

	return errors
}

func (v *Validator) ValidateAndLog(logger *logrus.Logger) bool {
	errors := v.Validate()
	if len(errors) == 0 {
		logger.Info("configuration validation passed")
		return true
	}

	logger.Error("configuration validation failed:")
	for _, line := range formatValidationErrors(errors) {
		logger.Error(line)
	}

	return false
}

func (v *Validator) ValidateWithError() error {
	errors := v.Validate()
	if len(errors) == 0 {
		return nil
	}

	return fmt.Errorf("config validation failed:\n%s", strings.Join(formatValidationErrors(errors), "\n"))
}
