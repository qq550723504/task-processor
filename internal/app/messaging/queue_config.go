// Package messaging 提供RabbitMQ队列配置
package messaging

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// QueueConfig 队列配置
type QueueConfig struct {
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
		{
			Name:       "tasks.exchange",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Args:       nil,
		},
		{
			Name:       "tasks.dlx",
			Type:       "direct",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Args:       nil,
		},
		{
			Name:       "tasks.delay.exchange",
			Type:       "direct",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Args:       nil,
		},
		{
			Name:       "tasks.result.exchange",
			Type:       "direct",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Args:       nil,
		},
	}
}

// GetQueueConfigs 获取所有队列配置
func GetQueueConfigs() []QueueConfig {
	queues := []QueueConfig{}

	// 添加上架任务队列
	queues = append(queues, getTaskQueues()...)

	// 添加爬虫任务队列
	queues = append(queues, getCrawlerQueues()...)

	// 添加系统队列
	queues = append(queues, getSystemQueues()...)

	return queues
}

// getTaskQueues 获取上架任务队列配置
func getTaskQueues() []QueueConfig {
	platforms := []string{"amazon", "temu", "shein"}
	priorities := []string{"high", "normal", "low"}

	queues := make([]QueueConfig, 0, len(platforms)*len(priorities))

	for _, platform := range platforms {
		for _, priority := range priorities {
			queues = append(queues, QueueConfig{
				Name:       platform + ".tasks." + priority,
				Durable:    true,
				AutoDelete: false,
				Exclusive:  false,
				NoWait:     false,
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

// getCrawlerQueues 获取爬虫任务队列配置
func getCrawlerQueues() []QueueConfig {
	crawlers := []string{"amazon", "1688"}

	queues := make([]QueueConfig, 0, len(crawlers))

	for _, crawler := range crawlers {
		queues = append(queues, QueueConfig{
			Name:       crawler + ".crawler.queue",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		})
	}

	return queues
}

// getSystemQueues 获取系统队列配置
func getSystemQueues() []QueueConfig {
	return []QueueConfig{
		// 死信队列
		{
			Name:       "tasks.dlq",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-message-ttl": 86400000, // 24小时TTL
			},
		},
		// 延迟队列
		{
			Name:       "tasks.delay.queue",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-dead-letter-exchange":    "tasks.exchange",
				"x-dead-letter-routing-key": "retry",
			},
		},
		// 结果队列
		{
			Name:       "tasks.result.queue",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args:       nil,
		},
	}
}

// GetBindingConfigs 获取所有绑定配置
func GetBindingConfigs() []BindingConfig {
	bindings := []BindingConfig{}

	// 添加任务队列绑定
	bindings = append(bindings, getTaskQueueBindings()...)

	// 添加系统队列绑定
	bindings = append(bindings, getSystemQueueBindings()...)

	return bindings
}

// getTaskQueueBindings 获取任务队列绑定配置
// 路由键格式: {targetPlatform}.{sourcePlatform}.{priority}.{region}
// 绑定规则: {targetPlatform}.*.{priority}.# (匹配任意来源平台和区域)
func getTaskQueueBindings() []BindingConfig {
	platforms := []string{"amazon", "temu", "shein"}
	priorities := []string{"high", "normal", "low"}

	bindings := make([]BindingConfig, 0, len(platforms)*len(priorities))

	for _, platform := range platforms {
		for _, priority := range priorities {
			bindings = append(bindings, BindingConfig{
				QueueName:    platform + ".tasks." + priority,
				ExchangeName: "tasks.exchange",
				RoutingKey:   platform + ".*." + priority + ".#", // 新格式：匹配任意来源平台
				NoWait:       false,
				Args:         nil,
			})
		}
	}

	return bindings
}

// getSystemQueueBindings 获取系统队列绑定配置
func getSystemQueueBindings() []BindingConfig {
	return []BindingConfig{
		// 死信队列绑定
		{
			QueueName:    "tasks.dlq",
			ExchangeName: "tasks.dlx",
			RoutingKey:   "failed",
			NoWait:       false,
			Args:         nil,
		},
		// 延迟队列绑定
		{
			QueueName:    "tasks.delay.queue",
			ExchangeName: "tasks.delay.exchange",
			RoutingKey:   "retry",
			NoWait:       false,
			Args:         nil,
		},
		// 结果队列绑定
		{
			QueueName:    "tasks.result.queue",
			ExchangeName: "tasks.result.exchange",
			RoutingKey:   "result",
			NoWait:       false,
			Args:         nil,
		},
	}
}
