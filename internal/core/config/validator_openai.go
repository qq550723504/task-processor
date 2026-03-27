package config

func ValidateOpenAIConfig(openai *OpenAIConfig) []error {
	var errors []error
	if openai == nil {
		return errors
	}

	hasDefaultClient := openai.APIKey != "" || openai.Model != "" || openai.BaseURL != "" || openai.Timeout != 0
	if hasDefaultClient {
		errors = append(errors, validateOpenAIClientFields("openai", openai.APIKey, openai.Model, openai.BaseURL, openai.Timeout)...)
	}

	for name, client := range openai.Clients {
		apiKey := client.APIKey
		if apiKey == "" {
			apiKey = openai.APIKey
		}
		baseURL := client.BaseURL
		if baseURL == "" {
			baseURL = openai.BaseURL
		}
		timeout := client.Timeout
		if timeout == 0 {
			timeout = openai.Timeout
		}
		errors = append(errors, validateOpenAIClientFields("openai.clients."+name, apiKey, client.Model, baseURL, timeout)...)
	}

	return errors
}

func validateOpenAIClientFields(prefix, apiKey, model, baseURL string, timeout int) []error {
	var errors []error

	if apiKey == "" {
		errors = append(errors, &ValidationError{
			Field:   prefix + ".apiKey",
			Message: "OpenAI API Key cannot be empty",
			Hint:    openAIHint(prefix+".apiKey", "set a non-empty API key in YAML or export TASK_PROCESSOR_OPENAI_API_KEY"),
		})
	}
	if model == "" {
		errors = append(errors, &ValidationError{
			Field:   prefix + ".model",
			Message: "OpenAI model cannot be empty",
			Hint:    openAIHint(prefix+".model", "set the model name in YAML or export TASK_PROCESSOR_OPENAI_MODEL"),
		})
	}
	if baseURL == "" {
		errors = append(errors, &ValidationError{
			Field:   prefix + ".baseURL",
			Message: "OpenAI baseURL cannot be empty",
			Hint:    openAIHint(prefix+".baseURL", "set the API base URL in YAML or export TASK_PROCESSOR_OPENAI_BASE_URL"),
		})
	}
	if timeout <= 0 {
		errors = append(errors, &ValidationError{
			Field:   prefix + ".timeout",
			Message: "OpenAI timeout must be greater than 0",
			Hint:    "set a positive timeout value in seconds for this client",
		})
	}

	return errors
}

func openAIHint(field, fallback string) string {
	if field == "openai.apiKey" || field == "openai.model" || field == "openai.baseURL" {
		return fallback
	}
	return "set this client field explicitly, or fill the shared openai defaults so named clients can inherit it"
}
