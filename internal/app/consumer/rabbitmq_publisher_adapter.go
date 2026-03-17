// Package consumer 提供RabbitMQ发布器适配器
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

// RabbitMQPublisherAdapter RabbitMQ发布器适配器
// 实现 processor.RabbitMQPublisher 接口
type RabbitMQPublisherAdapter struct {
	client *rabbitmq.Client
	logger *logrus.Logger
}

// NewRabbitMQPublisherAdapter 创建RabbitMQ发布器适配器
func NewRabbitMQPublisherAdapter(client *rabbitmq.Client, logger *logrus.Logger) *RabbitMQPublisherAdapter {
	return &RabbitMQPublisherAdapter{
		client: client,
		logger: logger,
	}
}

// Publish 发布消息到指定队列
func (a *RabbitMQPublisherAdapter) Publish(ctx context.Context, queueName string, data []byte) error {
	// 将 data 反序列化为 payload
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("反序列化消息数据失败: %w", err)
	}

	// 构建消息
	message := &rabbitmq.Message{
		ID:        fmt.Sprintf("result-%d", time.Now().UnixNano()),
		Type:      "crawler.result",
		Payload:   payload,
		Priority:  5,
		Timestamp: time.Now().Unix(),
	}

	// 发布选项
	publishOpts := rabbitmq.PublishOptions{
		Exchange:   "",        // 使用默认交换机
		RoutingKey: queueName, // 直接路由到队列
		Priority:   5,
		Persistent: true,
		Mandatory:  false,
		Immediate:  false,
	}

	// 发布消息
	if err := a.client.Publish(ctx, message, publishOpts); err != nil {
		return fmt.Errorf("发布消息到队列 %s 失败: %w", queueName, err)
	}

	a.logger.Debugf("消息已发布到队列: %s", queueName)
	return nil
}
