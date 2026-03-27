package config

import "github.com/spf13/viper"

func BuildRabbitMQConfig(v *viper.Viper) *RabbitMQConfig {
	return &RabbitMQConfig{
		Enabled:           v.GetBool("rabbitmq.enabled"),
		URL:               v.GetString("rabbitmq.url"),
		ReconnectInterval: getDuration(v, "rabbitmq.reconnectInterval", 5),
		MaxReconnectTries: v.GetInt("rabbitmq.maxReconnectTries"),
		Consumer: RabbitMQConsumerConfig{
			PrefetchCount: v.GetInt("rabbitmq.consumer.prefetchCount"),
			PrefetchSize:  v.GetInt("rabbitmq.consumer.prefetchSize"),
			RetryDelay:    getDuration(v, "rabbitmq.consumer.retryDelay", 5),
			MaxRetries:    v.GetInt("rabbitmq.consumer.maxRetries"),
			Queues:        buildQueueConfigs(v),
		},
		ResultReporter: ResultReporterConfig{
			ReportURL:  v.GetString("rabbitmq.resultReporter.reportURL"),
			NodeID:     v.GetString("rabbitmq.resultReporter.nodeID"),
			Timeout:    getDuration(v, "rabbitmq.resultReporter.timeout", 30),
			BufferSize: v.GetInt("rabbitmq.resultReporter.bufferSize"),
			Retry:      buildRetryConfig(v, "rabbitmq.resultReporter.retry"),
		},
		LoadMonitor: LoadMonitorConfig{
			UpdateInterval: getDuration(v, "rabbitmq.loadMonitor.updateInterval", 30),
			EnableCPU:      v.GetBool("rabbitmq.loadMonitor.enableCPU"),
			EnableMemory:   v.GetBool("rabbitmq.loadMonitor.enableMemory"),
			EnableTasks:    v.GetBool("rabbitmq.loadMonitor.enableTasks"),
		},
		Node: NodeConfig{
			NodeID:          v.GetString("rabbitmq.node.nodeID"),
			MaxConcurrency:  v.GetInt("rabbitmq.node.maxConcurrency"),
			HealthCheckPort: v.GetInt("rabbitmq.node.healthCheckPort"),
			MetricsPort:     v.GetInt("rabbitmq.node.metricsPort"),
			LogLevel:        v.GetString("rabbitmq.node.logLevel"),
			ShutdownTimeout: getDuration(v, "rabbitmq.node.shutdownTimeout", 30),
			OwnedStores:     getInt64Slice(v, "rabbitmq.node.ownedStores"),
			Regions:         v.GetStringSlice("rabbitmq.node.regions"),
		},
		Deduplicator: DeduplicatorConfig{
			TTL: getDuration(v, "rabbitmq.deduplicator.ttl", 600),
		},
		StoreAPI: StoreAPIConfig{
			BaseURL:  v.GetString("rabbitmq.storeAPI.baseURL"),
			CacheTTL: getDuration(v, "rabbitmq.storeAPI.cacheTTL", 300),
		},
	}
}

func buildQueueConfigs(v *viper.Viper) []QueueConfig {
	var queues []QueueConfig

	queueMaps := v.Get("rabbitmq.consumer.queues")
	if queueMaps == nil {
		return queues
	}

	queueList, ok := queueMaps.([]any)
	if !ok {
		return queues
	}

	for _, item := range queueList {
		queueMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		queue := QueueConfig{
			Name:     getStringFromMap(queueMap, "name"),
			Priority: getIntFromMap(queueMap, "priority"),
			Prefetch: getIntFromMap(queueMap, "prefetch"),
		}
		queues = append(queues, queue)
	}

	return queues
}

func buildRetryConfig(v *viper.Viper, prefix string) *RetryConfig {
	if !v.IsSet(prefix) {
		return DefaultRetryConfig()
	}

	return &RetryConfig{
		MaxRetries:    v.GetInt(prefix + ".maxRetries"),
		InitialDelay:  getDuration(v, prefix+".initialDelay", 1),
		MaxDelay:      getDuration(v, prefix+".maxDelay", 30),
		BackoffFactor: v.GetFloat64(prefix + ".backoffFactor"),
	}
}
