// Package rabbitmq 提供RabbitMQ队列消费者功能
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"task-processor/internal/pkg/recovery"
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

	// Panic 恢复策略
	panicCounts      map[string]int // 消息ID -> panic次数
	panicCountsMutex sync.RWMutex
	maxPanicRetries  int // 最大panic重试次数，默认3次

	// 状态管理和错误收集
	stateManager   *ConsumerStateManager
	errorCollector *ErrorCollector
}

// consume 消费消息（并发处理）
func (qc *QueueConsumer) consume() {
	defer recovery.Recover(fmt.Sprintf("队列消费者: %s", qc.queueName), qc.logger.WithField("queue", qc.queueName))

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
	defer qc.recoverFromPanic(delivery)

	startTime := time.Now()

	// 1. 解析消息
	msg, err := qc.parseMessage(delivery)
	if err != nil {
		qc.handleParseError(delivery, err)
		return
	}

	qc.logger.Debugf("开始处理消息: ID=%s, Queue=%s", msg.ID, qc.queueName)

	// 2. 处理消息
	err = qc.handleMessage(msg)
	if err != nil {
		qc.handleProcessError(delivery, msg, err)
		return
	}

	// 3. 确认消息
	qc.acknowledgeMessage(delivery, msg, time.Since(startTime))
}

// recoverFromPanic 从panic中恢复
func (qc *QueueConsumer) recoverFromPanic(delivery amqp.Delivery) {
	if r := recover(); r != nil {
		msgID := delivery.MessageId
		if msgID == "" {
			msgID = fmt.Sprintf("unknown-%d", time.Now().UnixNano())
		}

		// 记录 panic 次数
		qc.panicCountsMutex.Lock()
		qc.panicCounts[msgID]++
		panicCount := qc.panicCounts[msgID]
		qc.panicCountsMutex.Unlock()

		// 使用recovery包记录panic
		recovery.RecoverWithStack(fmt.Sprintf("处理消息: %s", msgID),
			qc.logger.WithField("message_id", msgID))

		// 收集错误
		panicErr := fmt.Errorf("panic: %v", r)
		qc.errorCollector.Collect(ErrorTypePanic, qc.queueName, msgID, panicErr, fmt.Sprintf("panic第%d次", panicCount))

		// 判断是否超过最大 panic 重试次数
		if panicCount > qc.maxPanicRetries {
			qc.logger.Warnf("消息 %s 已达到最大panic重试次数(%d)，发送到死信队列", msgID, qc.maxPanicRetries)
			// 超过最大次数，不再重试，发送到死信队列
			if err := delivery.Nack(false, false); err != nil {
				qc.logger.Errorf("拒绝消息失败: %v", err)
			}

			// 清理 panic 计数
			qc.panicCountsMutex.Lock()
			delete(qc.panicCounts, msgID)
			qc.panicCountsMutex.Unlock()
		} else {
			qc.logger.Infof("消息 %s 将重新排队 (panic重试 %d/%d)", msgID, panicCount, qc.maxPanicRetries)
			// 还未超过最大次数，重新排队
			if err := delivery.Nack(false, true); err != nil {
				qc.logger.Errorf("拒绝消息失败: %v", err)
			}
		}
	}
}

// parseMessage 解析消息
func (qc *QueueConsumer) parseMessage(delivery amqp.Delivery) (*Message, error) {
	msg, err := parseDeliveryMessage(delivery)
	if err != nil {
		return nil, fmt.Errorf("解析消息失败: %w", err)
	}
	return msg, nil
}

// handleMessage 处理消息业务逻辑
func (qc *QueueConsumer) handleMessage(msg *Message) error {
	return qc.handler.HandleMessage(qc.ctx, msg)
}

// handleParseError 处理解析错误
func (qc *QueueConsumer) handleParseError(delivery amqp.Delivery, err error) {
	qc.logger.Errorf("解析消息失败: %v", err)
	// 收集错误
	qc.errorCollector.Collect(ErrorTypeMessage, qc.queueName, delivery.MessageId, err, "解析消息失败")
	// 更新状态统计
	qc.stateManager.IncrementMessageCount(false)
	// 解析失败，拒绝消息不重新排队
	if ackErr := delivery.Nack(false, false); ackErr != nil {
		qc.logger.Errorf("拒绝消息失败: %v", ackErr)
	}
}

// handleProcessError 处理业务处理错误
func (qc *QueueConsumer) handleProcessError(delivery amqp.Delivery, msg *Message, err error) {
	qc.logger.Errorf("处理消息失败: ID=%s, Error=%v", msg.ID, err)
	// 收集错误
	qc.errorCollector.Collect(ErrorTypeMessage, qc.queueName, msg.ID, err, "处理消息失败")
	// 更新状态统计
	qc.stateManager.IncrementMessageCount(false)

	// 递增重试计数
	msg.RetryCount++

	// 检查是否需要重试
	if qc.shouldRetry(msg) {
		qc.retryMessage(delivery, msg)
	} else {
		qc.sendToDeadLetter(delivery, msg)
	}
}

// retryMessage 重试消息
func (qc *QueueConsumer) retryMessage(delivery amqp.Delivery, msg *Message) {
	qc.logger.Infof("消息将重新排队重试: ID=%s, RetryCount=%d/%d", msg.ID, msg.RetryCount, msg.MaxRetries)

	// 计算重试延迟（指数退避）
	delay := qc.calculateRetryDelay(msg.RetryCount)
	if delay > 0 {
		qc.logger.Debugf("消息 %s 将在 %v 后重试", msg.ID, delay)
		time.Sleep(delay)
	}

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
}

// calculateRetryDelay 计算重试延迟（指数退避）
func (qc *QueueConsumer) calculateRetryDelay(retryCount int) time.Duration {
	if retryCount == 0 {
		return 0 // 第一次重试不延迟
	}

	// 指数退避: 1s, 2s, 4s, 8s, 16s, 最大30s
	baseDelay := 1 * time.Second
	maxDelay := 30 * time.Second

	delay := time.Duration(1<<uint(retryCount-1)) * baseDelay
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// sendToDeadLetter 发送到死信队列
func (qc *QueueConsumer) sendToDeadLetter(delivery amqp.Delivery, msg *Message) {
	qc.logger.Warnf("消息已达到最大重试次数，发送到死信队列: ID=%s, RetryCount=%d/%d",
		msg.ID, msg.RetryCount, msg.MaxRetries)
	if nackErr := delivery.Nack(false, false); nackErr != nil {
		qc.logger.Errorf("发送消息到死信队列失败: %v", nackErr)
	}
}

// acknowledgeMessage 确认消息处理成功
func (qc *QueueConsumer) acknowledgeMessage(delivery amqp.Delivery, msg *Message, processingTime time.Duration) {
	// 处理成功，确认消息
	if ackErr := delivery.Ack(false); ackErr != nil {
		qc.logger.Errorf("确认消息失败: ID=%s, Error=%v", msg.ID, ackErr)
		qc.errorCollector.Collect(ErrorTypeMessage, qc.queueName, msg.ID, ackErr, "确认消息失败")
		return
	}

	// 更新状态统计
	qc.stateManager.IncrementMessageCount(true)

	qc.logger.Infof("消息处理成功: ID=%s, Queue=%s, Duration=%v",
		msg.ID, qc.queueName, processingTime)
}

// shouldRetry 判断是否应该重试
func (qc *QueueConsumer) shouldRetry(msg *Message) bool {
	return msg.RetryCount < msg.MaxRetries
}

// republishWithRetryCount 重新发布消息并更新重试计数
func (qc *QueueConsumer) republishWithRetryCount(msg *Message) error {
	// 序列化完整的 Message 结构，保持消息格式一致
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
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

	// 重新发布到同一个队列（保持完整的消息结构）
	err = channel.Publish(
		"",           // exchange
		qc.queueName, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         msgBytes, // 完整的 Message 结构
			DeliveryMode: amqp.Persistent,
			Priority:     msg.Priority,
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
