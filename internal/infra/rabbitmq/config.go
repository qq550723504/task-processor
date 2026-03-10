// Package rabbitmq 提供RabbitMQ配置结构
package rabbitmq

import (
	"fmt"
	"time"
)

// Config RabbitMQ完整配置
type Config struct {
	Connection ConnectionConfig `yaml:"connection"`
	Consumer   ConsumerConfig   `yaml:"consumer"`
	Queues     []QueueConfig    `yaml:"queues"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	URL               string        `yaml:"url"`
	ReconnectInterval time.Duration `yaml:"reconnect_interval"`
	MaxReconnectTries int           `yaml:"max_reconnect_tries"`
}

// ConsumerConfig 消费者配置
type ConsumerConfig struct {
	PrefetchCount int           `yaml:"prefetch_count"`
	PrefetchSize  int           `yaml:"prefetch_size"`
	RetryDelay    time.Duration `yaml:"retry_delay"`
	MaxRetries    int           `yaml:"max_retries"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	Name     string `yaml:"name"`     // 队列名称
	Priority int    `yaml:"priority"` // 队列优先级（10=高，5=中，1=低）
	Prefetch int    `yaml:"prefetch"` // 预取数量
}

// ConsumerQueueConfig 消费者队列配置（用于多队列消费）
// 保留此类型以保持向后兼容
type ConsumerQueueConfig = QueueConfig

// Validate 验证配置
func (c *Config) Validate() error {
	if err := c.Connection.Validate(); err != nil {
		return fmt.Errorf("connection配置错误: %w", err)
	}
	if err := c.Consumer.Validate(); err != nil {
		return fmt.Errorf("consumer配置错误: %w", err)
	}
	for i, queue := range c.Queues {
		if err := queue.Validate(); err != nil {
			return fmt.Errorf("queues[%d]配置错误: %w", i, err)
		}
	}
	return nil
}

// Validate 验证连接配置
func (c *ConnectionConfig) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("url不能为空")
	}
	if c.ReconnectInterval < 0 {
		return fmt.Errorf("reconnect_interval不能为负数")
	}
	if c.MaxReconnectTries < 0 {
		return fmt.Errorf("max_reconnect_tries不能为负数")
	}
	return nil
}

// Validate 验证消费者配置
func (c *ConsumerConfig) Validate() error {
	if c.PrefetchCount <= 0 {
		return fmt.Errorf("prefetch_count必须大于0")
	}
	if c.PrefetchSize < 0 {
		return fmt.Errorf("prefetch_size不能为负数")
	}
	if c.RetryDelay < 0 {
		return fmt.Errorf("retry_delay不能为负数")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries不能为负数")
	}
	return nil
}

// Validate 验证队列配置
func (c *QueueConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name不能为空")
	}
	if c.Priority < 0 || c.Priority > 10 {
		return fmt.Errorf("priority必须在0-10之间")
	}
	if c.Prefetch <= 0 {
		return fmt.Errorf("prefetch必须大于0")
	}
	return nil
}

// SetDefaults 设置默认值
func (c *Config) SetDefaults() {
	c.Connection.SetDefaults()
	c.Consumer.SetDefaults()
	for i := range c.Queues {
		c.Queues[i].SetDefaults()
	}
}

// SetDefaults 设置连接配置默认值
func (c *ConnectionConfig) SetDefaults() {
	if c.ReconnectInterval == 0 {
		c.ReconnectInterval = 5 * time.Second
	}
	if c.MaxReconnectTries == 0 {
		c.MaxReconnectTries = 10
	}
}

// SetDefaults 设置消费者配置默认值
func (c *ConsumerConfig) SetDefaults() {
	if c.PrefetchCount == 0 {
		c.PrefetchCount = 1
	}
	if c.RetryDelay == 0 {
		c.RetryDelay = 5 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
}

// SetDefaults 设置队列配置默认值
func (c *QueueConfig) SetDefaults() {
	if c.Priority == 0 {
		c.Priority = 5 // 默认中等优先级
	}
	if c.Prefetch == 0 {
		c.Prefetch = 1
	}
}
