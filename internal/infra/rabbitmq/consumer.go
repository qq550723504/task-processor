// Package rabbitmq 提供RabbitMQ消息消费者功能
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

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *Message) error
}

// MessageConsumer 消息消费者
type MessageConsumer struct {
	client    *Client
	logger    *logrus.Logger
	handlers  map[string]MessageHandler // queue -> handler
	consumers map[string]*QueueConsumer // queue -> consumer
	mutex     sync.RWMutex

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 配置
	config       ConsumerConfig
	queueConfigs []ConsumerQueueConfig // 多队列配置
}

// ConsumerConfig 消费者配置
type ConsumerConfig struct {
	PrefetchCount int           `yaml:"prefetch_count"`
	PrefetchSize  int           `yaml:"prefetch_size"`
	RetryDelay    time.Duration `yaml:"retry_delay"`
	MaxRetries    int           `yaml:"max_retries"`
}

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
	priority    int // 队列优先级
	prefetch    int // 预取数量
}

// NewMessageConsumer 创建消息消费者
func NewMessageConsumer(client *Client, config ConsumerConfig, logger *logrus.Logger) *MessageConsumer {
	if config.PrefetchCount == 0 {
		config.PrefetchCount = 1
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 5 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	return &MessageConsumer{
		client:       client,
		logger:       logger,
		handlers:     make(map[string]MessageHandler),
		consumers:    make(map[string]*QueueConsumer),
		config:       config,
		queueConfigs: []ConsumerQueueConfig{}, // 初始化为空，后续通过SetQueueConfigs设置
	}
}

// SetQueueConfigs 设置队列配置
func (mc *MessageConsumer) SetQueueConfigs(configs []ConsumerQueueConfig) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.queueConfigs = configs
	mc.logger.Infof("设置队列配置: %d个队列", len(configs))
}

// RegisterHandler 注册消息处理器
func (mc *MessageConsumer) RegisterHandler(queueName string, handler MessageHandler) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.handlers[queueName] = handler
	mc.logger.Infof("注册消息处理器: 队列=%s", queueName)
}

// Start 启动消费者
func (mc *MessageConsumer) Start(ctx context.Context) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.ctx, mc.cancel = context.WithCancel(ctx)

	// 启动所有队列的消费者
	for queueName, handler := range mc.handlers {
		if err := mc.startQueueConsumer(queueName, handler); err != nil {
			mc.logger.Errorf("启动队列 %s 消费者失败: %v", queueName, err)
			return fmt.Errorf("启动队列 %s 消费者失败: %w", queueName, err)
		}
	}

	mc.logger.Info("消息消费者启动完成")
	return nil
}

// startQueueConsumer 启动队列消费者
func (mc *MessageConsumer) startQueueConsumer(queueName string, handler MessageHandler) error {
	// 查找队列配置
	var queueConfig *ConsumerQueueConfig
	for i := range mc.queueConfigs {
		if mc.queueConfigs[i].Name == queueName {
			queueConfig = &mc.queueConfigs[i]
			break
		}
	}

	// 如果没有找到配置，使用默认值
	prefetch := mc.config.PrefetchCount
	priority := 5 // 默认中等优先级
	if queueConfig != nil {
		prefetch = queueConfig.Prefetch
		priority = queueConfig.Priority
		mc.logger.Infof("队列 %s 使用配置: priority=%d, prefetch=%d", queueName, priority, prefetch)
	} else {
		mc.logger.Warnf("队列 %s 未找到配置，使用默认值: priority=%d, prefetch=%d", queueName, priority, prefetch)
	}

	// 设置QoS
	channel, err := mc.client.connManager.GetChannel()
	if err != nil {
		return fmt.Errorf("获取通道失败: %w", err)
	}

	err = channel.Qos(
		prefetch,               // prefetch count（使用队列配置的值）
		mc.config.PrefetchSize, // prefetch size
		false,                  // global
	)
	if err != nil {
		return fmt.Errorf("设置QoS失败: %w", err)
	}

	// 开始消费
	consumerTag := fmt.Sprintf("%s-consumer-%d", queueName, time.Now().Unix())
	deliveries, err := mc.client.Consume(mc.ctx, ConsumeOptions{
		Queue:     queueName,
		Consumer:  consumerTag,
		AutoAck:   false, // 手动确认
		Exclusive: false,
		NoLocal:   false,
		NoWait:    false,
		Args:      nil,
	})
	if err != nil {
		return fmt.Errorf("开始消费队列 %s 失败: %w", queueName, err)
	}

	// 创建队列消费者
	queueCtx, queueCancel := context.WithCancel(mc.ctx)
	queueConsumer := &QueueConsumer{
		queueName:   queueName,
		consumerTag: consumerTag,
		handler:     handler,
		deliveries:  deliveries,
		ctx:         queueCtx,
		cancel:      queueCancel,
		logger:      mc.logger,
		config:      mc.config,
		priority:    priority,
		prefetch:    prefetch,
	}

	mc.consumers[queueName] = queueConsumer

	// 启动消费goroutine
	mc.wg.Add(1)
	go func() {
		defer mc.wg.Done()
		queueConsumer.consume()
	}()

	mc.logger.Infof("队列 %s 消费者启动成功，消费者标签: %s, priority=%d, prefetch=%d",
		queueName, consumerTag, priority, prefetch)
	return nil
}

// consume 消费消息
func (qc *QueueConsumer) consume() {
	defer func() {
		if r := recover(); r != nil {
			qc.logger.Errorf("队列 %s 消费者发生panic: %v", qc.queueName, r)
		}
	}()

	qc.logger.Infof("开始消费队列: %s", qc.queueName)

	for {
		select {
		case <-qc.ctx.Done():
			qc.logger.Infof("队列 %s 消费者停止", qc.queueName)
			return
		case delivery, ok := <-qc.deliveries:
			if !ok {
				qc.logger.Warnf("队列 %s 消费通道已关闭", qc.queueName)
				return
			}

			qc.processMessage(delivery)
		}
	}
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

		// 检查是否需要重试
		if qc.shouldRetry(msg) {
			qc.logger.Infof("消息将重新排队重试: ID=%s, RetryCount=%d", msg.ID, msg.RetryCount)
			if nackErr := delivery.Nack(false, true); nackErr != nil {
				qc.logger.Errorf("重新排队消息失败: %v", nackErr)
			}
		} else {
			qc.logger.Warnf("消息已达到最大重试次数，发送到死信队列: ID=%s", msg.ID)
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

// parseDeliveryMessage 解析投递消息
func parseDeliveryMessage(delivery amqp.Delivery) (*Message, error) {
	msg := &Message{
		ID:        delivery.MessageId,
		Type:      delivery.Type,
		Timestamp: delivery.Timestamp.Unix(),
	}

	// 解析消息体为 Payload
	if len(delivery.Body) > 0 {
		var payload map[string]interface{}
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

// Stop 停止消费者
func (mc *MessageConsumer) Stop(ctx context.Context) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.logger.Info("开始停止消息消费者...")

	// 取消所有消费者
	for queueName, consumer := range mc.consumers {
		mc.logger.Infof("停止队列 %s 消费者", queueName)
		consumer.cancel()
	}

	// 取消主上下文
	if mc.cancel != nil {
		mc.cancel()
	}

	// 等待所有goroutine完成
	done := make(chan struct{})
	go func() {
		defer close(done)
		mc.wg.Wait()
	}()

	select {
	case <-done:
		mc.logger.Info("所有消费者goroutine已停止")
	case <-ctx.Done():
		mc.logger.Warn("等待消费者停止超时")
		return fmt.Errorf("停止消费者超时")
	}

	mc.logger.Info("消息消费者停止完成")
	return nil
}

// GetQueueStats 获取队列统计信息
func (mc *MessageConsumer) GetQueueStats() map[string]interface{} {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["total_queues"] = len(mc.consumers)
	stats["active_consumers"] = len(mc.consumers)

	queueStats := make(map[string]interface{})
	for queueName := range mc.consumers {
		queueStats[queueName] = map[string]interface{}{
			"status": "active",
		}
	}
	stats["queues"] = queueStats

	return stats
}
