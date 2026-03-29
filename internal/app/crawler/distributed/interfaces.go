// Package distributed 提供分布式爬虫接口定义
package distributed

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher 消息发布接口（可 mock）
type Publisher interface {
	// Publish 发布消息到指定队列（使用默认交换机）
	Publish(ctx context.Context, queueName string, body []byte, priority uint8) error
}

// QueueDeclarer 队列声明接口（可 mock）
type QueueDeclarer interface {
	// DeclareExclusiveQueue 声明独占临时队列，返回实际队列名
	DeclareExclusiveQueue(name string) (string, error)
	// ConsumeQueue 开始消费队列，返回消息 channel
	ConsumeQueue(queueName, consumerTag string) (<-chan amqp.Delivery, error)
}

// priorityCalculator 优先级计算接口（可 mock）
type priorityCalculator interface {
	CalculatePriority(priority int) uint8
}

// queueNamer 队列命名接口（可 mock）
type queueNamer interface {
	BuildCrawlerQueueName(platform string, priority int) string
	BuildCrawlerQueueNameByRegion(platform, region string, priority int) string
}
