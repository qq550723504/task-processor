// Package validators 提供配置验证功能
package config

// ValidateAmazonConfig 验证Amazon配置
func ValidateAmazonConfig(amazon *AmazonConfig) []error {
	var errors []error

	if !amazon.Enabled {
		return errors
	}

	// 目前Amazon配置没有必填项验证
	return errors
}
