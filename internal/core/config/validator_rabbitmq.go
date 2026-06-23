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
	if !rabbitmq.Node.HasValidRole() {
		errors = append(errors, &ValidationError{
			Field:   "rabbitmq.node.role",
			Message: "RabbitMQ node role must be one of task, crawler, hybrid",
			Hint:    "set rabbitmq.node.role in YAML or export TASK_PROCESSOR_RABBITMQ_NODE_ROLE",
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
	for _, bucket := range rabbitmq.Node.OwnedBuckets {
		if bucket < 0 || bucket >= 8 {
			errors = append(errors, &ValidationError{
				Field:   "rabbitmq.node.ownedBuckets",
				Message: "RabbitMQ node ownedBuckets must be within [0,7] for SHEIN bucket queues",
				Hint:    "set rabbitmq.node.ownedBuckets to a list like [0,1,2] for explicit bucket sharding",
			})
			break
		}
	}
	if rabbitmq.AutoShard.Enabled {
		if !rabbitmq.AutoShard.HasValidRole() {
			errors = append(errors, &ValidationError{
				Field:   "rabbitmq.autoShard.role",
				Message: "RabbitMQ autoShard role must be one of coordinator, worker, disabled",
				Hint:    "set rabbitmq.autoShard.role in YAML or export TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE",
			})
		}
		if rabbitmq.AutoShard.IsCoordinator() && len(rabbitmq.AutoShard.CandidateNodes) == 0 {
			errors = append(errors, &ValidationError{
				Field:   "rabbitmq.autoShard.candidateNodes",
				Message: "RabbitMQ autoShard candidateNodes cannot be empty when auto sharding coordinator is enabled",
				Hint:    "set rabbitmq.autoShard.candidateNodes for coordinator role to a list like [shein-listing-store-a, shein-listing-store-b]",
			})
		}
		if rabbitmq.AutoShard.Interval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "rabbitmq.autoShard.interval",
				Message: "RabbitMQ autoShard interval must be greater than 0",
				Hint:    "set rabbitmq.autoShard.interval to a positive number of seconds",
			})
		}
		if rabbitmq.AutoShard.LockTTL <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "rabbitmq.autoShard.lockTTL",
				Message: "RabbitMQ autoShard lockTTL must be greater than 0",
				Hint:    "set rabbitmq.autoShard.lockTTL to a positive number of seconds",
			})
		}
	}

	return errors
}
