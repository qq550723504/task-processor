package consumer

import (
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

func applyRabbitMQServiceDefaults(cfg *config.RabbitMQConfig) {
	if cfg.ReconnectInterval == 0 {
		cfg.ReconnectInterval = 5 * time.Second
	}
	if cfg.MaxReconnectTries == 0 {
		cfg.MaxReconnectTries = 10
	}
	if cfg.Consumer.PrefetchCount == 0 {
		cfg.Consumer.PrefetchCount = 1
	}
	if cfg.Consumer.RetryDelay == 0 {
		cfg.Consumer.RetryDelay = 5 * time.Second
	}
	if cfg.Consumer.MaxRetries == 0 {
		cfg.Consumer.MaxRetries = 3
	}
}

func newRabbitMQConnectionManager(cfg *config.RabbitMQConfig, logger *logrus.Logger) *rabbitmq.ConnectionManager {
	return rabbitmq.NewConnectionManager(rabbitmq.ConnectionConfig{
		URL:               cfg.URL,
		ReconnectInterval: cfg.ReconnectInterval,
		MaxReconnectTries: cfg.MaxReconnectTries,
	}, logger)
}

func newRabbitMQConsumer(client *rabbitmq.Client, cfg *config.RabbitMQConfig, logger *logrus.Logger) *rabbitmq.MessageConsumer {
	return rabbitmq.NewMessageConsumer(client, rabbitmq.ConsumerConfig{
		PrefetchCount:  cfg.Consumer.PrefetchCount,
		PrefetchSize:   cfg.Consumer.PrefetchSize,
		RetryDelay:     cfg.Consumer.RetryDelay,
		MaxRetries:     cfg.Consumer.MaxRetries,
		MaxConcurrency: cfg.Node.MaxConcurrency,
	}, logger)
}
