// Package distributed 提供 RabbitMQ 适配器，将 infra 层的 Client 适配为本包的接口
package distributed

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/infra/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQAdapter 将 *rabbitmq.Client 适配为 Publisher 和 QueueDeclarer
type RabbitMQAdapter struct {
	client *rabbitmq.Client
}

// NewRabbitMQAdapter 创建适配器
func NewRabbitMQAdapter(client *rabbitmq.Client) *RabbitMQAdapter {
	return &RabbitMQAdapter{client: client}
}

// Publish 实现 Publisher 接口
func (a *RabbitMQAdapter) Publish(ctx context.Context, queueName string, body []byte, priority uint8) error {
	msg := &rabbitmq.Message{
		ID:        fmt.Sprintf("crawl-%d", time.Now().UnixNano()),
		Type:      "task",
		Timestamp: time.Now().Unix(),
	}
	// body 已经是完整的 JSON，直接放到 Payload 里会二次序列化，
	// 所以这里绕过 Message 包装，直接用底层 channel 发布原始 body。
	ch, err := a.client.GetConnectionManager().CreateChannel()
	if err != nil {
		return fmt.Errorf("创建发布通道失败: %w", err)
	}
	defer ch.Close()

	_ = msg // msg 仅用于生成 ID，实际 body 直接发布
	return ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		Priority:     priority,
		DeliveryMode: 2, // 持久化
		Timestamp:    time.Now(),
	})
}

// DeclareExclusiveQueue 实现 QueueDeclarer 接口
// 注意：不使用 exclusive=true，改用 autoDelete=true + durable=false，
// 这样任意连接都可以发布结果到该队列，避免独占限制导致爬虫处理器无法发布。
func (a *RabbitMQAdapter) DeclareExclusiveQueue(name string) (string, error) {
	ch, err := a.client.GetConnectionManager().CreateChannel()
	if err != nil {
		return "", fmt.Errorf("创建通道失败: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		name,  // 队列名
		false, // durable=false（临时队列）
		true,  // autoDelete=true（无消费者时自动删除）
		false, // exclusive=false（允许其他连接发布）
		false, // noWait
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("声明结果队列失败: %w", err)
	}
	return q.Name, nil
}

// ConsumeQueue 实现 QueueDeclarer 接口
func (a *RabbitMQAdapter) ConsumeQueue(queueName, consumerTag string) (<-chan amqp.Delivery, error) {
	ch, err := a.client.GetConnectionManager().CreateChannel()
	if err != nil {
		return nil, fmt.Errorf("创建消费通道失败: %w", err)
	}

	msgs, err := ch.Consume(queueName, consumerTag, false, false, false, false, nil)
	if err != nil {
		ch.Close()
		return nil, fmt.Errorf("开始消费队列 %s 失败: %w", queueName, err)
	}
	return msgs, nil
}
