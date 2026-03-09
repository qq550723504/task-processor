// Package rabbitmq 提供RabbitMQ队列消费者功能
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// QueueConsumer 队列消费者
type QueueConsumer struct {
	queueName   string
	consumerTag string
	handler     MessageHandler
	deliveries  <-chan amqp.Delivery
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	logger      *logrus.Logger
	config      ConsumerConfig
	priority    int           // 队列优先级
	prefetch    int           // 预取数量
	client      *Client       // client字段用于重新发布消息
	channel     *amqp.Channel // 独立通道（用于避免QoS冲突）
}

// consume 消费消息（并发处理）
func (qc *QueueConsumer) consume() {
	defer func() {
		if r := recover(); r != nil {
			qc.logger.Errorf("队列 %s 消费者发生panic: %v", qc.queueName, r)
		}
	}()

	qc.logger.Infof("开始消费队列: %s (并发度: %d)", qc.queueName, qc.prefetch)

	// 创建工作池，根据 prefetch 数量创建对应数量的 worker
	for i := 0; i < qc.prefetch; i++ {
		qc.wg.Add(1)
		go func(workerID int) {
			defer qc.wg.Done()
			qc.logger.Debugf("队列 %s Worker #%d 启动", qc.queueName, workerID)

			for {
				select {
				case <-qc.ctx.Done():
					qc.logger.Debugf("队列 %s Worker #%d 停止", qc.queueName, workerID)
					return
				case delivery, ok := <-qc.deliveries:
					if !ok {
						qc.logger.Debugf("队列 %s Worker #%d 通道已关闭", qc.queueName, workerID)
						return
					}

					qc.logger.Debugf("队列 %s Worker #%d 处理消息: %s", qc.queueName, workerID, delivery.MessageId)
					qc.processMessage(delivery)
				}
			}
		}(i)
	}

	// 等待所有 worker 完成
	qc.wg.Wait()
	qc.logger.Infof("队列 %s 所有 worker 已停止", qc.queueName)
}

// processMessage 处理消息
func (qc *QueueConsumer) processMessage(delivery amqp.Delivery) {
	defer func() {
		if r := recover(); r != nil {
			qc.logger.Errorf("处理消息发生panic: %v", r)
			// Panic时拒绝消息并重新排队
			if err := delivery.Nack(false, true); err != nil {
				qc.logger.Errorf("拒绝消息失败: %v", err)
			}
		}
	}()

	startTime := time.Now()

	// 解析消息
	msg, err := parseDeliveryMessage(delivery)
	if err != nil {
		qc.logger.Errorf("解析消息失败: %v", err)
		// 解析失败，拒绝消息不重新排队
		if ackErr := delivery.Nack(false, false); ackErr != nil {
			qc.logger.Errorf("拒绝消息失败: %v", ackErr)
		}
		return
	}

	qc.logger.Debugf("开始处理消息: ID=%s, Queue=%s", msg.ID, qc.queueName)

	// 处理消息
	err = qc.handler.HandleMessage(qc.ctx, msg)
	if err != nil {
		qc.logger.Errorf("处理消息失败: ID=%s, Error=%v", msg.ID, err)

		// 递增重试计数
		msg.RetryCount++

		// 检查是否需要重试
		if qc.shouldRetry(msg) {
			qc.logger.Infof("消息将重新排队重试: ID=%s, RetryCount=%d/%d", msg.ID, msg.RetryCount, msg.MaxRetries)

			// 更新消息的重试计数后重新发布
			if republishErr := qc.republishWithRetryCount(msg); republishErr != nil {
				qc.logger.Errorf("重新发布消息失败: %v", republishErr)
				// 如果重新发布失败，仍然Nack原消息
				if nackErr := delivery.Nack(false, false); nackErr != nil {
					qc.logger.Errorf("拒绝消息失败: %v", nackErr)
				}
				return
			}

			// 确认原消息（因为已经重新发布了新消息）
			if ackErr := delivery.Ack(false); ackErr != nil {
				qc.logger.Errorf("确认消息失败: %v", ackErr)
			}
		} else {
			qc.logger.Warnf("消息已达到最大重试次数，发送到死信队列: ID=%s, RetryCount=%d/%d",
				msg.ID, msg.RetryCount, msg.MaxRetries)
			if nackErr := delivery.Nack(false, false); nackErr != nil {
				qc.logger.Errorf("发送消息到死信队列失败: %v", nackErr)
			}
		}
		return
	}

	// 处理成功，确认消息
	if ackErr := delivery.Ack(false); ackErr != nil {
		qc.logger.Errorf("确认消息失败: ID=%s, Error=%v", msg.ID, ackErr)
		return
	}

	processingTime := time.Since(startTime)
	qc.logger.Infof("消息处理成功: ID=%s, Queue=%s, Duration=%v",
		msg.ID, qc.queueName, processingTime)
}

// shouldRetry 判断是否应该重试
func (qc *QueueConsumer) shouldRetry(msg *Message) bool {
	return msg.RetryCount < msg.MaxRetries
}

// republishWithRetryCount 重新发布消息并更新重试计数
func (qc *QueueConsumer) republishWithRetryCount(msg *Message) error {
	// 序列化Payload（保持原始消息格式）
	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return fmt.Errorf("序列化Payload失败: %w", err)
	}

	// 获取channel
	channel, err := qc.client.GetConnectionManager().GetChannel()
	if err != nil {
		return fmt.Errorf("获取channel失败: %w", err)
	}

	// 构建Headers，包含重试信息
	headers := amqp.Table{
		"retry_count": int32(msg.RetryCount),
		"max_retries": int32(msg.MaxRetries),
	}

	// 重新发布到同一个队列（使用原始格式：Body是Payload，重试信息在Headers中）
	err = channel.Publish(
		"",           // exchange
		qc.queueName, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         payloadBytes,
			DeliveryMode: amqp.Persistent,
			Priority:     uint8(msg.Priority),
			MessageId:    msg.ID,
			Type:         msg.Type,
			Headers:      headers,
		},
	)

	if err != nil {
		return fmt.Errorf("发布消息失败: %w", err)
	}

	qc.logger.Debugf("重新发布消息成功: ID=%s, RetryCount=%d", msg.ID, msg.RetryCount)
	return nil
}
