// Package validators 提供配置验证功能
package config

// ValidateManagementConfig 验证管理系统配置
func ValidateManagementConfig(mgmt *ManagementConfig) []error {
	var errors []error

	if mgmt.BaseURL == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.baseURL",
			Message: "管理系统 URL 不能为空",
		})
	}

	if mgmt.ClientID == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.clientID",
			Message: "客户端 ID 不能为空",
		})
	}

	if mgmt.ClientSecret == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.clientSecret",
			Message: "客户端密钥不能为空",
		})
	}

	if mgmt.TenantID == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.tenantID",
			Message: "租户 ID 不能为空",
		})
	}

	return errors
}
