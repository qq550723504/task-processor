package config

func ValidateManagementConfig(mgmt *ManagementConfig) []error {
	var errors []error
	if mgmt == nil {
		return errors
	}

	if mgmt.BaseURL == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.baseURL",
			Message: "Management baseURL cannot be empty",
			Hint:    "set management.baseURL in YAML or export TASK_PROCESSOR_MANAGEMENT_BASE_URL",
		})
	}
	if mgmt.ClientID == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.clientID",
			Message: "Management clientID cannot be empty",
			Hint:    "set management.clientID in YAML or export TASK_PROCESSOR_MANAGEMENT_CLIENT_ID",
		})
	}
	if mgmt.ClientSecret == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.clientSecret",
			Message: "Management clientSecret cannot be empty",
			Hint:    "set management.clientSecret in YAML or export TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET",
		})
	}
	if mgmt.TokenURL == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.tokenURL",
			Message: "Management tokenURL cannot be empty",
			Hint:    "set management.tokenURL in YAML or export TASK_PROCESSOR_MANAGEMENT_TOKEN_URL",
		})
	}
	if mgmt.TenantID == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.tenantID",
			Message: "Management tenantID cannot be empty",
			Hint:    "set management.tenantID in YAML or export TASK_PROCESSOR_MANAGEMENT_TENANT_ID",
		})
	}
	if len(mgmt.Scopes) == 0 {
		errors = append(errors, &ValidationError{
			Field:   "management.scopes",
			Message: "Management scopes cannot be empty",
			Hint:    "set at least one OAuth scope in management.scopes",
		})
	}

	return errors
}
