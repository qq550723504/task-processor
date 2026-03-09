// Package rabbitmq 提供RabbitMQ消息消费者管理功能
package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	queueConfigs []ConsumerQueueConfig // 多队列配置
}

// ConsumerConfig 消费者配置
type ConsumerConfig struct {
	PrefetchCount int           `yaml:"prefetch_count"`
	PrefetchSize  int           `yaml:"prefetch_size"`
	RetryDelay    time.Duration `yaml:"retry_delay"`
	MaxRetries    int           `yaml:"max_retries"`
}

// NewMessageConsumer 创建消息消费者管理器
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

// Restart 重启所有消费者（用于重连后恢复）
func (mc *MessageConsumer) Restart() error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.logger.Info("开始重启所有消费者...")

	// 停止现有消费者并关闭通道
	for queueName, consumer := range mc.consumers {
		mc.logger.Infof("停止队列 %s 消费者", queueName)
		consumer.cancel()
		// 关闭独立通道
		if consumer.channel != nil {
			consumer.channel.Close()
		}
	}

	// 清空消费者列表
	mc.consumers = make(map[string]*QueueConsumer)

	// 重新启动所有队列的消费者
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

	// 为每个消费者创建独立通道（避免QoS冲突）
	channel, err := mc.client.connManager.CreateChannel()
	if err != nil {
		return fmt.Errorf("创建独立通道失败: %w", err)
	}

	// 在独立通道上设置QoS
	err = channel.Qos(
		prefetch,               // prefetch count（使用队列配置的值）
		mc.config.PrefetchSize, // prefetch size
		false,                  // global
	)
	if err != nil {
		channel.Close() // 关闭通道
		return fmt.Errorf("设置QoS失败: %w", err)
	}

	// 在独立通道上开始消费
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
		channel.Close() // 关闭通道
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
		client:      mc.client,
		channel:     channel, // 保存独立通道
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

// Stop 停止消费者
func (mc *MessageConsumer) Stop(ctx context.Context) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.logger.Info("开始停止消息消费者...")

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
