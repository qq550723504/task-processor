// Package loaders 提供配置加载功能
package config

import (

	"github.com/spf13/viper"
)

// BuildRabbitMQConfig 构建RabbitMQ配置
func BuildRabbitMQConfig() *RabbitMQConfig {
	return &RabbitMQConfig{
		Enabled:           viper.GetBool("rabbitmq.enabled"),
		URL:               viper.GetString("rabbitmq.url"),
		ReconnectInterval: getDuration("rabbitmq.reconnectInterval", 5),
		MaxReconnectTries: viper.GetInt("rabbitmq.maxReconnectTries"),
		Consumer: RabbitMQConsumerConfig{
			PrefetchCount: viper.GetInt("rabbitmq.consumer.prefetchCount"),
			PrefetchSize:  viper.GetInt("rabbitmq.consumer.prefetchSize"),
			RetryDelay:    getDuration("rabbitmq.consumer.retryDelay", 5),
			MaxRetries:    viper.GetInt("rabbitmq.consumer.maxRetries"),
			Queues:        buildQueueConfigs(),
		},
		ResultReporter: ResultReporterConfig{
			ReportURL:  viper.GetString("rabbitmq.resultReporter.reportURL"),
			NodeID:     viper.GetString("rabbitmq.resultReporter.nodeID"),
			Timeout:    getDuration("rabbitmq.resultReporter.timeout", 30),
			BufferSize: viper.GetInt("rabbitmq.resultReporter.bufferSize"),
			Retry:      buildRetryConfig("rabbitmq.resultReporter.retry"),
		},
		LoadMonitor: LoadMonitorConfig{
			UpdateInterval: getDuration("rabbitmq.loadMonitor.updateInterval", 30),
			EnableCPU:      viper.GetBool("rabbitmq.loadMonitor.enableCPU"),
			EnableMemory:   viper.GetBool("rabbitmq.loadMonitor.enableMemory"),
			EnableTasks:    viper.GetBool("rabbitmq.loadMonitor.enableTasks"),
		},
		Node: NodeConfig{
			NodeID:          viper.GetString("rabbitmq.node.nodeID"),
			MaxConcurrency:  viper.GetInt("rabbitmq.node.maxConcurrency"),
			HealthCheckPort: viper.GetInt("rabbitmq.node.healthCheckPort"),
			MetricsPort:     viper.GetInt("rabbitmq.node.metricsPort"),
			LogLevel:        viper.GetString("rabbitmq.node.logLevel"),
			ShutdownTimeout: getDuration("rabbitmq.node.shutdownTimeout", 30),
		},
		Deduplicator: DeduplicatorConfig{
			TTL: getDuration("rabbitmq.deduplicator.ttl", 600),
		},
		StoreAPI: StoreAPIConfig{
			BaseURL:  viper.GetString("rabbitmq.storeAPI.baseURL"),
			CacheTTL: getDuration("rabbitmq.storeAPI.cacheTTL", 300),
		},
	}
}

// buildQueueConfigs 构建队列配置列表
func buildQueueConfigs() []QueueConfig {
	var queues []QueueConfig

	// 从配置中读取队列列表
	queueMaps := viper.Get("rabbitmq.consumer.queues")
	if queueMaps == nil {
		return queues
	}

	// 类型断言为 []any
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

// buildRetryConfig 构建重试配置
func buildRetryConfig(prefix string) *RetryConfig {
	if !viper.IsSet(prefix) {
		return DefaultRetryConfig()
	}

	return &RetryConfig{
		MaxRetries:    viper.GetInt(prefix + ".maxRetries"),
		InitialDelay:  getDuration(prefix+".initialDelay", 1),
		MaxDelay:      getDuration(prefix+".maxDelay", 30),
		BackoffFactor: viper.GetFloat64(prefix + ".backoffFactor"),
	}
}
