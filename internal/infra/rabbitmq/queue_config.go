// Package rabbitmq 提供 RabbitMQ 队列声明配置
package rabbitmq

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// QueueDeclareConfig AMQP 队列声明参数
type QueueDeclareConfig struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       amqp.Table
}

// ExchangeConfig 交换机配置
type ExchangeConfig struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp.Table
}

// BindingConfig 绑定配置
type BindingConfig struct {
	QueueName    string
	ExchangeName string
	RoutingKey   string
	NoWait       bool
	Args         amqp.Table
}

// GetExchangeConfigs 获取所有交换机配置
func GetExchangeConfigs() []ExchangeConfig {
	return []ExchangeConfig{
		{Name: "tasks.exchange", Type: "topic", Durable: true},
		{Name: "tasks.dlx", Type: "direct", Durable: true},
		{Name: "tasks.delay.exchange", Type: "direct", Durable: true},
		{Name: "tasks.result.exchange", Type: "direct", Durable: true},
	}
}

// GetQueueDeclareConfigs 获取所有队列声明配置
func GetQueueDeclareConfigs() []QueueDeclareConfig {
	var queues []QueueDeclareConfig
	queues = append(queues, getTaskQueues()...)
	queues = append(queues, getCrawlerQueues()...)
	queues = append(queues, getSystemQueues()...)
	return queues
}

func getTaskQueues() []QueueDeclareConfig {
	platforms := []string{"amazon", "temu", "shein"}
	priorities := []string{"high", "normal", "low"}
	queues := make([]QueueDeclareConfig, 0, len(platforms)*len(priorities))
	for _, platform := range platforms {
		for _, priority := range priorities {
			queues = append(queues, QueueDeclareConfig{
				Name:    platform + ".tasks." + priority,
				Durable: true,
				Args: amqp.Table{
					"x-max-priority":            10,
					"x-dead-letter-exchange":    "tasks.dlx",
					"x-dead-letter-routing-key": "failed",
				},
			})
		}
	}
	return queues
}

func getCrawlerQueues() []QueueDeclareConfig {
	crawlers := []string{"amazon", "1688"}
	priorities := []string{"high", "normal", "low"}
	queues := make([]QueueDeclareConfig, 0, len(crawlers)*len(priorities))
	for _, crawler := range crawlers {
		for _, priority := range priorities {
			queues = append(queues, QueueDeclareConfig{
				Name:    crawler + ".crawler." + priority,
				Durable: true,
				Args: amqp.Table{
					"x-max-priority":            10,
					"x-dead-letter-exchange":    "tasks.dlx",
					"x-dead-letter-routing-key": "failed",
				},
			})
		}
	}
	return queues
}

func getSystemQueues() []QueueDeclareConfig {
	return []QueueDeclareConfig{
		{
			Name:    "tasks.dlq",
			Durable: true,
			Args:    amqp.Table{"x-message-ttl": 86400000},
		},
		{
			Name:    "tasks.delay.queue",
			Durable: true,
			Args: amqp.Table{
				"x-dead-letter-exchange":    "tasks.exchange",
				"x-dead-letter-routing-key": "retry",
			},
		},
		{
			Name:    "tasks.result.queue",
			Durable: true,
		},
	}
}

// GetBindingConfigs 获取所有绑定配置
func GetBindingConfigs() []BindingConfig {
	var bindings []BindingConfig
	bindings = append(bindings, getTaskQueueBindings()...)
	bindings = append(bindings, getSystemQueueBindings()...)
	return bindings
}

func getTaskQueueBindings() []BindingConfig {
	platforms := []string{"amazon", "temu", "shein"}
	priorities := []string{"high", "normal", "low"}
	bindings := make([]BindingConfig, 0, len(platforms)*len(priorities))
	for _, platform := range platforms {
		for _, priority := range priorities {
			bindings = append(bindings, BindingConfig{
				QueueName:    platform + ".tasks." + priority,
				ExchangeName: "tasks.exchange",
				RoutingKey:   platform + ".*." + priority + ".#",
			})
		}
	}
	crawlers := []string{"amazon", "1688"}
	for _, crawler := range crawlers {
		for _, priority := range priorities {
			bindings = append(bindings, BindingConfig{
				QueueName:    crawler + ".crawler." + priority,
				ExchangeName: "tasks.exchange",
				RoutingKey:   crawler + ".crawler." + priority + ".#",
			})
		}
	}
	return bindings
}

func getSystemQueueBindings() []BindingConfig {
	return []BindingConfig{
		{QueueName: "tasks.dlq", ExchangeName: "tasks.dlx", RoutingKey: "failed"},
		{QueueName: "tasks.delay.queue", ExchangeName: "tasks.delay.exchange", RoutingKey: "retry"},
		{QueueName: "tasks.result.queue", ExchangeName: "tasks.result.exchange", RoutingKey: "result"},
	}
}
