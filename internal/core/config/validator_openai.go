// Package validators 提供配置验证功能
package config

// ValidateOpenAIConfig 验证OpenAI配置
func ValidateOpenAIConfig(openai *OpenAIConfig) []error {
	var errors []error

	if openai.APIKey != "" && openai.Model == "" {
		errors = append(errors, &ValidationError{
			Field:   "openai.model",
			Message: "OpenAI 模型不能为空",
		})
	}

	return errors
}
