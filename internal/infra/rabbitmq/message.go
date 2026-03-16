// Package rabbitmq 提供RabbitMQ消息相关功能
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *Message) error
}

// parseDeliveryMessage 解析投递消息
func parseDeliveryMessage(delivery amqp.Delivery) (*Message, error) {
	msg := &Message{
		ID:        delivery.MessageId,
		Type:      delivery.Type,
		Timestamp: delivery.Timestamp.Unix(),
		Priority:  delivery.Priority,
	}

	// 解析消息体为 Payload
	if len(delivery.Body) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(delivery.Body, &payload); err != nil {
			return nil, fmt.Errorf("解析消息体失败: %w", err)
		}
		msg.Payload = payload
	}

	// 从Headers中获取重试信息
	if delivery.Headers != nil {
		if retryCount, ok := delivery.Headers["retry_count"].(int32); ok {
			msg.RetryCount = int(retryCount)
		}
		if maxRetries, ok := delivery.Headers["max_retries"].(int32); ok {
			msg.MaxRetries = int(maxRetries)
		}
	}

	// 设置默认值
	if msg.MaxRetries == 0 {
		msg.MaxRetries = 3
	}

	return msg, nil
}
