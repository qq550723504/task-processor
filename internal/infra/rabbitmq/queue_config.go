// Package rabbitmq 提供 RabbitMQ 队列声明配置
package rabbitmq

import (
	"fmt"
	"strings"

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

// GetQueueDeclareConfigs 获取所有队列声明配置（系统级固定队列）
func GetQueueDeclareConfigs() []QueueDeclareConfig {
	var queues []QueueDeclareConfig
	queues = append(queues, getCrawlerQueues()...)
	queues = append(queues, getSystemQueues()...)
	return queues
}

// GetStoreQueueDeclareConfigs 获取指定店铺的任务队列声明配置
func GetStoreQueueDeclareConfigs(platform string, storeID int64) []QueueDeclareConfig {
	return []QueueDeclareConfig{
		{
			Name:    buildStoreQueueName(platform, storeID),
			Durable: true,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
	}
}

// GetStoreQueueBindingConfigs 获取指定店铺的队列绑定配置
func GetStoreQueueBindingConfigs(platform string, storeID int64) []BindingConfig {
	queueName := buildStoreQueueName(platform, storeID)
	return []BindingConfig{
		{
			QueueName:    queueName,
			ExchangeName: "tasks.exchange",
			RoutingKey:   queueName, // 精确匹配: {platform}.tasks.store.{storeID}
		},
	}
}

// buildStoreQueueName 构建店铺专属队列名称
// 格式: {platform}.tasks.store.{storeID}
func buildStoreQueueName(platform string, storeID int64) string {
	return fmt.Sprintf("%s.tasks.store.%d", platform, storeID)
}

func getCrawlerQueues() []QueueDeclareConfig {
	crawlers := []string{"amazon", "1688"}
	queues := make([]QueueDeclareConfig, 0, len(crawlers))
	for _, crawler := range crawlers {
		queues = append(queues, QueueDeclareConfig{
			Name:    crawler + ".crawler",
			Durable: true,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		})
	}
	return queues
}

// GetRegionCrawlerQueueDeclareConfigs 获取指定 region 列表的爬虫队列声明配置
// 格式: {platform}.crawler.{region}
func GetRegionCrawlerQueueDeclareConfigs(regions []string) []QueueDeclareConfig {
	crawlers := []string{"amazon", "1688"}
	queues := make([]QueueDeclareConfig, 0, len(crawlers)*len(regions))
	for _, crawler := range crawlers {
		for _, region := range regions {
			queues = append(queues, QueueDeclareConfig{
				Name:    fmt.Sprintf("%s.crawler.%s", crawler, strings.ToLower(region)),
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

// GetBindingConfigs 获取系统级固定队列的绑定配置（爬虫队列 + 系统队列）
func GetBindingConfigs() []BindingConfig {
	return getSystemQueueBindings()
}

func getSystemQueueBindings() []BindingConfig {
	return []BindingConfig{
		{QueueName: "tasks.dlq", ExchangeName: "tasks.dlx", RoutingKey: "failed"},
		{QueueName: "tasks.delay.queue", ExchangeName: "tasks.delay.exchange", RoutingKey: "retry"},
		{QueueName: "tasks.result.queue", ExchangeName: "tasks.result.exchange", RoutingKey: "result"},
	}
}
