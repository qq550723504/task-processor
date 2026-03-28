// Package rabbitmq provides unit tests for RabbitMQ configuration
package rabbitmq

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConfig_Validate tests full configuration validation
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid config",
			config: Config{
				Connection: ConnectionConfig{
					URL:               "amqp://localhost:5672/",
					ReconnectInterval: 5 * time.Second,
					MaxReconnectTries: 10,
				},
				Consumer: ConsumerConfig{
					PrefetchCount: 1,
					PrefetchSize:  0,
					RetryDelay:    5 * time.Second,
					MaxRetries:    3,
				},
				Queues: []QueueConfig{
					{Name: "test-queue", Priority: 5, Prefetch: 1},
				},
			},
			expectError: false,
		},
		{
			name: "empty URL",
			config: Config{
				Connection: ConnectionConfig{
					URL:               "",
					ReconnectInterval: 5 * time.Second,
					MaxReconnectTries: 10,
				},
				Consumer: ConsumerConfig{
					PrefetchCount: 1,
					PrefetchSize:  0,
					RetryDelay:    5 * time.Second,
					MaxRetries:    3,
				},
			},
			expectError: true,
		},
		{
			name: "invalid prefetch count",
			config: Config{
				Connection: ConnectionConfig{
					URL:               "amqp://localhost:5672/",
					ReconnectInterval: 5 * time.Second,
					MaxReconnectTries: 10,
				},
				Consumer: ConsumerConfig{
					PrefetchCount: 0,
					PrefetchSize:  0,
					RetryDelay:    5 * time.Second,
					MaxRetries:    3,
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConnectionConfig_Validate tests connection configuration validation
func TestConnectionConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      ConnectionConfig
		expectError bool
	}{
		{
			name: "valid",
			config: ConnectionConfig{
				URL:               "amqp://admin:password@localhost:5672/",
				ReconnectInterval: 5 * time.Second,
				MaxReconnectTries: 10,
			},
			expectError: false,
		},
		{
			name: "empty URL",
			config: ConnectionConfig{
				URL:               "",
				ReconnectInterval: 5 * time.Second,
				MaxReconnectTries: 10,
			},
			expectError: true,
		},
		{
			name: "negative reconnect interval",
			config: ConnectionConfig{
				URL:               "amqp://localhost:5672/",
				ReconnectInterval: -1 * time.Second,
				MaxReconnectTries: 10,
			},
			expectError: true,
		},
		{
			name: "negative max reconnect tries",
			config: ConnectionConfig{
				URL:               "amqp://localhost:5672/",
				ReconnectInterval: 5 * time.Second,
				MaxReconnectTries: -1,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConsumerConfig_Validate tests consumer configuration validation
func TestConsumerConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      ConsumerConfig
		expectError bool
	}{
		{
			name: "valid",
			config: ConsumerConfig{
				PrefetchCount: 1,
				PrefetchSize:  0,
				RetryDelay:    5 * time.Second,
				MaxRetries:    3,
			},
			expectError: false,
		},
		{
			name: "zero prefetch count",
			config: ConsumerConfig{
				PrefetchCount: 0,
				PrefetchSize:  0,
				RetryDelay:    5 * time.Second,
				MaxRetries:    3,
			},
			expectError: true,
		},
		{
			name: "negative prefetch size",
			config: ConsumerConfig{
				PrefetchCount: 1,
				PrefetchSize:  -1,
				RetryDelay:    5 * time.Second,
				MaxRetries:    3,
			},
			expectError: true,
		},
		{
			name: "negative retry delay",
			config: ConsumerConfig{
				PrefetchCount: 1,
				PrefetchSize:  0,
				RetryDelay:    -1 * time.Second,
				MaxRetries:    3,
			},
			expectError: true,
		},
		{
			name: "negative max retries",
			config: ConsumerConfig{
				PrefetchCount: 1,
				PrefetchSize:  0,
				RetryDelay:    5 * time.Second,
				MaxRetries:    -1,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestQueueConfig_Validate tests queue configuration validation
func TestQueueConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      QueueConfig
		expectError bool
	}{
		{
			name: "valid",
			config: QueueConfig{
				Name:     "test-queue",
				Priority: 5,
				Prefetch: 1,
			},
			expectError: false,
		},
		{
			name: "empty name",
			config: QueueConfig{
				Name:     "",
				Priority: 5,
				Prefetch: 1,
			},
			expectError: true,
		},
		{
			name: "zero priority",
			config: QueueConfig{
				Name:     "test-queue",
				Priority: 0,
				Prefetch: 1,
			},
			expectError: false,
		},
		{
			name: "negative prefetch",
			config: QueueConfig{
				Name:     "test-queue",
				Priority: 5,
				Prefetch: -1,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
