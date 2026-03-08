// Package rabbitmq 提供RabbitMQ配置结构
package rabbitmq

// ConsumerQueueConfig 消费者队列配置（用于多队列消费）
type ConsumerQueueConfig struct {
	Name     string `yaml:"name"`     // 队列名称
	Priority int    `yaml:"priority"` // 队列优先级（10=高，5=中，1=低）
	Prefetch int    `yaml:"prefetch"` // 预取数量
}
