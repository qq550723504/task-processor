// Package validators 提供配置验证功能
package validators

import "task-processor/internal/core/config/types"

// ValidateAmazonConfig 验证Amazon配置
func ValidateAmazonConfig(amazon *types.AmazonConfig) []error {
	var errors []error

	if !amazon.Enabled {
		return errors
	}

	// 目前Amazon配置没有必填项验证
	return errors
}
