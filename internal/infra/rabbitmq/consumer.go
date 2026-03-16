// Package rabbitmq 提供RabbitMQ消息消费者管理功能
package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// MessageConsumer 消息消费者管理器
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
	queueConfigs []QueueConfig // 多队列配置

	// 状态管理和错误收集
	stateManager   map[string]*ConsumerStateManager // queue -> state manager
	errorCollector *ErrorCollector
}

// NewMessageConsumer 创建消息消费者管理器
func NewMessageConsumer(client *Client, config ConsumerConfig, logger *logrus.Logger) *MessageConsumer {
	// 设置默认值
	config.SetDefaults()

	return &MessageConsumer{
		client:         client,
		logger:         logger,
		handlers:       make(map[string]MessageHandler),
		consumers:      make(map[string]*QueueConsumer),
		config:         config,
		queueConfigs:   []QueueConfig{},
		stateManager:   make(map[string]*ConsumerStateManager),
		errorCollector: NewErrorCollector(1000),
	}
}

// SetQueueConfigs 设置队列配置
func (mc *MessageConsumer) SetQueueConfigs(configs []QueueConfig) {
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
	// 为每个队列创建状态管理器
	mc.stateManager[queueName] = NewConsumerStateManager()
}

// Start 启动消费者
func (mc *MessageConsumer) Start(ctx context.Context) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.ctx, mc.cancel = context.WithCancel(ctx)

	// 启动所有队列的消费者
	for queueName, handler := range mc.handlers {
		// 设置状态为启动中
		if sm, exists := mc.stateManager[queueName]; exists {
			sm.SetState(ConsumerStateStarting, queueName)
		}

		if err := mc.startQueueConsumer(queueName, handler); err != nil {
			mc.logger.Errorf("启动队列 %s 消费者失败: %v", queueName, err)
			// 记录错误
			mc.errorCollector.Collect(ErrorTypeConsumer, queueName, "", err, "启动消费者失败")
			// 设置错误状态
			if sm, exists := mc.stateManager[queueName]; exists {
				sm.SetError(err, queueName)
			}
			return fmt.Errorf("启动队列 %s 消费者失败: %w", queueName, err)
		}

		// 设置状态为运行中
		if sm, exists := mc.stateManager[queueName]; exists {
			sm.SetState(ConsumerStateRunning, queueName)
		}
	}

	mc.logger.Info("消息消费者启动完成")
	return nil
}

// Restart 重启所有消费者（用于重连后恢复）
func (mc *MessageConsumer) Restart() error {
	mc.logger.Info("开始重启所有消费者...")

	// 1. 获取旧的消费者列表（在锁内）
	mc.mutex.Lock()
	oldConsumers := mc.consumers
	mc.consumers = make(map[string]*QueueConsumer)
	mc.mutex.Unlock()

	// 2. 在锁外停止旧消费者（避免长时间持锁）
	var wg sync.WaitGroup
	for queueName, consumer := range oldConsumers {
		wg.Add(1)
		go func(name string, c *QueueConsumer) {
			defer wg.Done()
			mc.logger.Infof("停止队列 %s 消费者", name)
			c.cancel()
			// 关闭独立通道
			if c.channel != nil {
				c.channel.Close()
			}
		}(queueName, consumer)
	}
	wg.Wait()

	// 3. 重新启动所有队列的消费者（在锁内）
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	for queueName, handler := range mc.handlers {
		if err := mc.startQueueConsumer(queueName, handler); err != nil {
			mc.logger.Errorf("重启队列 %s 消费者失败: %v", queueName, err)
			// 不返回错误，继续尝试启动其他队列
			continue
		}
	}

	mc.logger.Info("消费者重启完成")
	return nil
}

// startQueueConsumer 启动队列消费者
func (mc *MessageConsumer) startQueueConsumer(queueName string, handler MessageHandler) error {
	// 查找队列配置
	queueConfig := mc.findQueueConfig(queueName)

	// 创建独立通道并设置QoS
	channel, err := mc.createConsumerChannel(queueConfig)
	if err != nil {
		return err
	}

	// 开始消费
	deliveries, consumerTag, err := mc.startConsuming(channel, queueName)
	if err != nil {
		channel.Close()
		return err
	}

	// 创建队列消费者
	consumer := mc.createQueueConsumer(queueName, consumerTag, handler, channel, deliveries, queueConfig)
	mc.consumers[queueName] = consumer

	// 启动消费goroutine
	mc.launchConsumerWorkers(consumer)

	mc.logger.Infof("队列 %s 消费者启动成功，消费者标签: %s, priority=%d, prefetch=%d",
		queueName, consumerTag, queueConfig.Priority, queueConfig.Prefetch)
	return nil
}

// findQueueConfig 查找队列配置
func (mc *MessageConsumer) findQueueConfig(queueName string) *QueueConfig {
	for i := range mc.queueConfigs {
		if mc.queueConfigs[i].Name == queueName {
			config := mc.queueConfigs[i]
			config.SetDefaults()
			return &config
		}
	}

	// 如果没有找到配置，返回默认配置
	mc.logger.Warnf("队列 %s 未找到配置，使用默认值", queueName)
	defaultConfig := QueueConfig{
		Name:     queueName,
		Priority: 5,
		Prefetch: mc.config.PrefetchCount,
	}
	return &defaultConfig
}

// createConsumerChannel 创建消费者独立通道并设置QoS
func (mc *MessageConsumer) createConsumerChannel(queueConfig *QueueConfig) (*amqp.Channel, error) {
	// 为每个消费者创建独立通道（避免QoS冲突）
	channel, err := mc.client.connManager.CreateChannel()
	if err != nil {
		return nil, fmt.Errorf("创建独立通道失败: %w", err)
	}

	// 在独立通道上设置QoS
	err = channel.Qos(
		queueConfig.Prefetch,   // prefetch count
		mc.config.PrefetchSize, // prefetch size
		false,                  // global
	)
	if err != nil {
		channel.Close()
		return nil, fmt.Errorf("设置QoS失败: %w", err)
	}

	return channel, nil
}

// startConsuming 开始消费队列
func (mc *MessageConsumer) startConsuming(channel *amqp.Channel, queueName string) (<-chan amqp.Delivery, string, error) {
	consumerTag := fmt.Sprintf("%s-consumer-%d", queueName, time.Now().Unix())

	deliveries, err := channel.Consume(
		queueName,   // 队列名称
		consumerTag, // 消费者标签
		false,       // autoAck - 手动确认
		false,       // exclusive
		false,       // noLocal
		false,       // noWait
		nil,         // args
	)
	if err != nil {
		return nil, "", fmt.Errorf("开始消费队列 %s 失败: %w", queueName, err)
	}

	return deliveries, consumerTag, nil
}

// createQueueConsumer 创建队列消费者实例
func (mc *MessageConsumer) createQueueConsumer(
	queueName string,
	consumerTag string,
	handler MessageHandler,
	channel *amqp.Channel,
	deliveries <-chan amqp.Delivery,
	queueConfig *QueueConfig,
) *QueueConsumer {
	queueCtx, queueCancel := context.WithCancel(mc.ctx)

	return &QueueConsumer{
		queueName:       queueName,
		consumerTag:     consumerTag,
		handler:         handler,
		deliveries:      deliveries,
		ctx:             queueCtx,
		cancel:          queueCancel,
		logger:          mc.logger,
		config:          mc.config,
		priority:        queueConfig.Priority,
		prefetch:        queueConfig.Prefetch,
		client:          mc.client,
		channel:         channel,
		panicCounts:     make(map[string]int),
		maxPanicRetries: 3,
		stateManager:    mc.stateManager[queueName],
		errorCollector:  mc.errorCollector,
	}
}

// launchConsumerWorkers 启动消费者工作协程
func (mc *MessageConsumer) launchConsumerWorkers(consumer *QueueConsumer) {
	mc.wg.Add(1)
	go func() {
		defer mc.wg.Done()
		consumer.consume()
	}()
}

// Stop 停止消费者
func (mc *MessageConsumer) Stop(ctx context.Context) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.logger.Info("开始停止消息消费者...")

	// 设置所有消费者状态为停止中
	for queueName, sm := range mc.stateManager {
		sm.SetState(ConsumerStateStopping, queueName)
	}

	// 取消所有消费者并关闭通道
	for queueName, consumer := range mc.consumers {
		mc.logger.Infof("停止队列 %s 消费者", queueName)
		consumer.cancel()
		// 关闭独立通道
		if consumer.channel != nil {
			consumer.channel.Close()
		}
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
		// 设置所有消费者状态为已停止
		for queueName, sm := range mc.stateManager {
			sm.SetState(ConsumerStateStopped, queueName)
		}
	case <-ctx.Done():
		mc.logger.Warn("等待消费者停止超时")
		return fmt.Errorf("停止消费者超时")
	}

	mc.logger.Info("消息消费者停止完成")
	return nil
}

// GetQueueStats 获取队列统计信息
func (mc *MessageConsumer) GetQueueStats() map[string]any {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	stats := make(map[string]any)
	stats["total_queues"] = len(mc.consumers)

	activeCount := 0
	queueStats := make(map[string]any)
	for queueName := range mc.consumers {
		sm := mc.stateManager[queueName]
		stateInfo := sm.GetStateInfo()

		if sm.IsRunning() {
			activeCount++
		}

		queueStats[queueName] = map[string]any{
			"status":        stateInfo.State.String(),
			"message_count": stateInfo.MessageCount,
			"success_count": stateInfo.SuccessCount,
			"failure_count": stateInfo.FailureCount,
			"error_count":   stateInfo.ErrorCount,
			"is_healthy":    sm.IsHealthy(),
		}
	}

	stats["active_consumers"] = activeCount
	stats["queues"] = queueStats

	// 添加错误统计
	errorStats := mc.errorCollector.GetErrorStats()
	stats["errors"] = map[string]any{
		"total":    errorStats.Total,
		"by_type":  errorStats.ByType,
		"by_queue": errorStats.ByQueue,
	}

	return stats
}

// GetErrorCollector 获取错误收集器
func (mc *MessageConsumer) GetErrorCollector() *ErrorCollector {
	return mc.errorCollector
}
