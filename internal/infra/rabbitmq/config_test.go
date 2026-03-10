package rabbitmq

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "有效配置",
			config: Config{
				Connection: ConnectionConfig{
					URL:               "amqp://localhost:5672",
					ReconnectInterval: 5 * time.Second,
					MaxReconnectTries: 10,
				},
				Consumer: ConsumerConfig{
					PrefetchCount: 10,
					PrefetchSize:  0,
					RetryDelay:    5 * time.Second,
					MaxRetries:    3,
				},
				Queues: []QueueConfig{
					{Name: "test.queue", Priority: 5, Prefetch: 10},
				},
			},
			wantErr: false,
		},
		{
			name: "连接URL为空",
			config: Config{
				Connection: ConnectionConfig{
					URL: "",
				},
				Consumer: ConsumerConfig{
					PrefetchCount: 10,
				},
			},
			wantErr: true,
		},
		{
			name: "PrefetchCount为0",
			config: Config{
				Connection: ConnectionConfig{
					URL: "amqp://localhost:5672",
				},
				Consumer: ConsumerConfig{
					PrefetchCount: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "队列优先级超出范围",
			config: Config{
				Connection: ConnectionConfig{
					URL: "amqp://localhost:5672",
				},
				Consumer: ConsumerConfig{
					PrefetchCount: 10,
				},
				Queues: []QueueConfig{
					{Name: "test.queue", Priority: 11, Prefetch: 10},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	config := Config{
		Connection: ConnectionConfig{
			URL: "amqp://localhost:5672",
		},
		Consumer: ConsumerConfig{},
		Queues: []QueueConfig{
			{Name: "test.queue"},
		},
	}

	config.SetDefaults()

	// 检查连接配置默认值
	if config.Connection.ReconnectInterval != 5*time.Second {
		t.Errorf("ReconnectInterval = %v, want %v", config.Connection.ReconnectInterval, 5*time.Second)
	}
	if config.Connection.MaxReconnectTries != 10 {
		t.Errorf("MaxReconnectTries = %v, want %v", config.Connection.MaxReconnectTries, 10)
	}

	// 检查消费者配置默认值
	if config.Consumer.PrefetchCount != 1 {
		t.Errorf("PrefetchCount = %v, want %v", config.Consumer.PrefetchCount, 1)
	}
	if config.Consumer.RetryDelay != 5*time.Second {
		t.Errorf("RetryDelay = %v, want %v", config.Consumer.RetryDelay, 5*time.Second)
	}
	if config.Consumer.MaxRetries != 3 {
		t.Errorf("MaxRetries = %v, want %v", config.Consumer.MaxRetries, 3)
	}

	// 检查队列配置默认值
	if config.Queues[0].Priority != 5 {
		t.Errorf("Queue Priority = %v, want %v", config.Queues[0].Priority, 5)
	}
	if config.Queues[0].Prefetch != 1 {
		t.Errorf("Queue Prefetch = %v, want %v", config.Queues[0].Prefetch, 1)
	}
}

func TestConnectionConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ConnectionConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			config: ConnectionConfig{
				URL:               "amqp://localhost:5672",
				ReconnectInterval: 5 * time.Second,
				MaxReconnectTries: 10,
			},
			wantErr: false,
		},
		{
			name: "URL为空",
			config: ConnectionConfig{
				URL: "",
			},
			wantErr: true,
		},
		{
			name: "负数重连间隔",
			config: ConnectionConfig{
				URL:               "amqp://localhost:5672",
				ReconnectInterval: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "负数最大重连次数",
			config: ConnectionConfig{
				URL:               "amqp://localhost:5672",
				MaxReconnectTries: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectionConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConsumerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ConsumerConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			config: ConsumerConfig{
				PrefetchCount: 10,
				PrefetchSize:  0,
				RetryDelay:    5 * time.Second,
				MaxRetries:    3,
			},
			wantErr: false,
		},
		{
			name: "PrefetchCount为0",
			config: ConsumerConfig{
				PrefetchCount: 0,
			},
			wantErr: true,
		},
		{
			name: "负数PrefetchSize",
			config: ConsumerConfig{
				PrefetchCount: 10,
				PrefetchSize:  -1,
			},
			wantErr: true,
		},
		{
			name: "负数RetryDelay",
			config: ConsumerConfig{
				PrefetchCount: 10,
				RetryDelay:    -1 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ConsumerConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueueConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  QueueConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			config: QueueConfig{
				Name:     "test.queue",
				Priority: 5,
				Prefetch: 10,
			},
			wantErr: false,
		},
		{
			name: "队列名为空",
			config: QueueConfig{
				Name:     "",
				Priority: 5,
				Prefetch: 10,
			},
			wantErr: true,
		},
		{
			name: "优先级小于0",
			config: QueueConfig{
				Name:     "test.queue",
				Priority: -1,
				Prefetch: 10,
			},
			wantErr: true,
		},
		{
			name: "优先级大于10",
			config: QueueConfig{
				Name:     "test.queue",
				Priority: 11,
				Prefetch: 10,
			},
			wantErr: true,
		},
		{
			name: "Prefetch为0",
			config: QueueConfig{
				Name:     "test.queue",
				Priority: 5,
				Prefetch: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("QueueConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
