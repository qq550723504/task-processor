package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

func (c *Config) Validate() []error {
	return ValidateConfig(c)
}

func ValidateConfig(c *Config) []error {
	validator := NewValidator(
		&c.Processor,
		&c.Worker,
		&c.OpenAI,
		&c.Management,
		&c.Browser,
		&c.Amazon,
		c.RabbitMQ,
		&c.Platforms,
	)
	return validator.Validate()
}

func (c *Config) ValidateAndLog(logger *logrus.Logger) bool {
	return ValidateConfigAndLog(c, logger)
}

func ValidateConfigAndLog(c *Config, logger *logrus.Logger) bool {
	errors := ValidateConfig(c)
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

func (c *Config) ValidateWithError() error {
	return ValidateConfigWithError(c)
}

func ValidateConfigWithError(c *Config) error {
	errors := ValidateConfig(c)
	if len(errors) == 0 {
		return nil
	}

	return fmt.Errorf("config validation failed:\n%s", strings.Join(formatValidationErrors(errors), "\n"))
}

func formatValidationErrors(errors []error) []string {
	grouped := make(map[string][]string)
	var order []string

	for _, err := range errors {
		module, message := classifyValidationError(err)
		if _, exists := grouped[module]; !exists {
			order = append(order, module)
		}
		grouped[module] = append(grouped[module], message)
	}

	sort.SliceStable(order, func(i, j int) bool {
		return validationModuleRank(order[i]) < validationModuleRank(order[j])
	})

	var lines []string
	for _, module := range order {
		lines = append(lines, fmt.Sprintf("[%s]", module))
		for _, message := range grouped[module] {
			lines = append(lines, "  - "+message)
		}
	}

	return lines
}

func classifyValidationError(err error) (string, string) {
	if ve, ok := err.(*ValidationError); ok {
		message := fmt.Sprintf("%s: %s", ve.Field, ve.Message)
		if ve.Hint != "" {
			message = fmt.Sprintf("%s (hint: %s)", message, ve.Hint)
		}
		return moduleFromField(ve.Field), message
	}
	return "General", err.Error()
}

func moduleFromField(field string) string {
	switch {
	case strings.HasPrefix(field, "openai."):
		return "OpenAI"
	case strings.HasPrefix(field, "management."):
		return "Management"
	case strings.HasPrefix(field, "browser."):
		return "Browser"
	case strings.HasPrefix(field, "amazon.spapi."):
		return "Amazon SP-API"
	case strings.HasPrefix(field, "amazon."):
		return "Amazon"
	case strings.HasPrefix(field, "rabbitmq."):
		return "RabbitMQ"
	case strings.HasPrefix(field, "platforms.temu."):
		return "Platforms.TEMU"
	case strings.HasPrefix(field, "platforms.shein."):
		return "Platforms.SHEIN"
	case strings.HasPrefix(field, "platforms.alibaba1688."):
		return "Platforms.1688"
	case strings.HasPrefix(field, "worker."):
		return "Worker"
	case strings.HasPrefix(field, "processor."):
		return "Processor"
	default:
		return "General"
	}
}

func validationModuleRank(module string) int {
	switch module {
	case "Management":
		return 1
	case "OpenAI":
		return 2
	case "Browser":
		return 3
	case "Amazon SP-API":
		return 4
	case "Amazon":
		return 5
	case "RabbitMQ":
		return 6
	case "Platforms.TEMU":
		return 7
	case "Platforms.SHEIN":
		return 8
	case "Platforms.1688":
		return 9
	case "Worker":
		return 10
	case "Processor":
		return 11
	default:
		return 99
	}
}
