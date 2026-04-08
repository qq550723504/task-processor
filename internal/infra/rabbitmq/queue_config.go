// Package rabbitmq 提供 RabbitMQ 队列声明配置
package rabbitmq

import (
	"fmt"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

const sheinBucketQueueCount = 8

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

// GetPlatformQueueDeclareConfigs 获取平台共享任务队列声明配置
func GetPlatformQueueDeclareConfigs(platform string) []QueueDeclareConfig {
	queues := []QueueDeclareConfig{
		{
			Name:    buildPlatformQueueName(platform),
			Durable: true,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
	}
	if usesBucketQueues(platform) {
		for bucket := 0; bucket < sheinBucketQueueCount; bucket++ {
			queues = append(queues, QueueDeclareConfig{
				Name:    buildBucketQueueName(platform, bucket),
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

// GetPlatformQueueBindingConfigs 获取平台共享队列绑定配置
// 兼容两种路由方式：
// 1. 新共享模式: {platform}.tasks
// 2. 现有店铺模式: {platform}.tasks.store.{storeId}
func GetPlatformQueueBindingConfigs(platform string) []BindingConfig {
	queueName := buildPlatformQueueName(platform)
	bindings := []BindingConfig{
		{
			QueueName:    queueName,
			ExchangeName: "tasks.exchange",
			RoutingKey:   queueName,
		},
		{
			QueueName:    queueName,
			ExchangeName: "tasks.exchange",
			RoutingKey:   queueName + ".store.*",
		},
	}
	if usesBucketQueues(platform) {
		for bucket := 0; bucket < sheinBucketQueueCount; bucket++ {
			bindings = append(bindings, BindingConfig{
				QueueName:    buildBucketQueueName(platform, bucket),
				ExchangeName: "tasks.exchange",
				RoutingKey:   buildBucketQueueName(platform, bucket),
			})
		}
	}
	return bindings
}

// buildStoreQueueName 构建店铺专属队列名称
// 格式: {platform}.tasks.store.{storeID}
func buildStoreQueueName(platform string, storeID int64) string {
	return fmt.Sprintf("%s.tasks.store.%d", platform, storeID)
}

func buildPlatformQueueName(platform string) string {
	return fmt.Sprintf("%s.tasks", platform)
}

func buildBucketQueueName(platform string, bucket int) string {
	return fmt.Sprintf("%s.tasks.bucket.%d", platform, bucket)
}

func usesBucketQueues(platform string) bool {
	return strings.EqualFold(platform, "shein")
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
