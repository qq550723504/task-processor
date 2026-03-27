package config

func ValidateRabbitMQConfig(rabbitmq *RabbitMQConfig) []error {
	var errors []error
	if rabbitmq == nil || !rabbitmq.Enabled {
		return errors
	}

	if rabbitmq.URL == "" {
		errors = append(errors, &ValidationError{
			Field:   "rabbitmq.url",
			Message: "RabbitMQ url cannot be empty",
			Hint:    "set rabbitmq.url in YAML or export TASK_PROCESSOR_RABBITMQ_URL",
		})
	}
	if rabbitmq.MaxReconnectTries < 0 {
		errors = append(errors, &ValidationError{
			Field:   "rabbitmq.maxReconnectTries",
			Message: "RabbitMQ maxReconnectTries cannot be negative",
			Hint:    "set rabbitmq.maxReconnectTries to 0 or a positive retry count",
		})
	}
	if rabbitmq.Consumer.PrefetchCount <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "rabbitmq.consumer.prefetchCount",
			Message: "RabbitMQ consumer prefetchCount must be greater than 0",
			Hint:    "set rabbitmq.consumer.prefetchCount to a positive integer",
		})
	}
	if rabbitmq.Consumer.MaxRetries < 0 {
		errors = append(errors, &ValidationError{
			Field:   "rabbitmq.consumer.maxRetries",
			Message: "RabbitMQ consumer maxRetries cannot be negative",
			Hint:    "set rabbitmq.consumer.maxRetries to 0 or a positive retry count",
		})
	}
	if rabbitmq.Node.MaxConcurrency <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "rabbitmq.node.maxConcurrency",
			Message: "RabbitMQ node maxConcurrency must be greater than 0",
			Hint:    "set rabbitmq.node.maxConcurrency in YAML or export TASK_PROCESSOR_RABBITMQ_NODE_MAX_CONCURRENCY",
		})
	}
	if rabbitmq.Node.HealthCheckPort <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "rabbitmq.node.healthCheckPort",
			Message: "RabbitMQ node healthCheckPort must be greater than 0",
			Hint:    "set rabbitmq.node.healthCheckPort in YAML or export TASK_PROCESSOR_RABBITMQ_NODE_HEALTH_CHECK_PORT",
		})
	}
	if rabbitmq.Node.MetricsPort <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "rabbitmq.node.metricsPort",
			Message: "RabbitMQ node metricsPort must be greater than 0",
			Hint:    "set rabbitmq.node.metricsPort in YAML or export TASK_PROCESSOR_RABBITMQ_NODE_METRICS_PORT",
		})
	}

	return errors
}
