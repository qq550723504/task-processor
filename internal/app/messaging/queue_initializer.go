// Package messaging 提供RabbitMQ队列初始化功能
package messaging

import (
	"fmt"
	"task-processor/internal/infra/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// QueueInitializer 队列初始化器
type QueueInitializer struct {
	client *rabbitmq.Client
	logger *logrus.Logger
}

// NewQueueInitializer 创建队列初始化器
func NewQueueInitializer(client *rabbitmq.Client, logger *logrus.Logger) *QueueInitializer {
	return &QueueInitializer{
		client: client,
		logger: logger,
	}
}

// InitializeAll 初始化所有队列和交换机
func (qi *QueueInitializer) InitializeAll() error {
	qi.logger.Info("开始初始化RabbitMQ队列和交换机...")

	// 初始化交换机
	if err := qi.initializeExchanges(); err != nil {
		return fmt.Errorf("初始化交换机失败: %w", err)
	}

	// 初始化队列
	if err := qi.initializeQueues(); err != nil {
		return fmt.Errorf("初始化队列失败: %w", err)
	}

	// 初始化绑定
	if err := qi.initializeBindings(); err != nil {
		return fmt.Errorf("初始化绑定失败: %w", err)
	}

	qi.logger.Info("RabbitMQ队列和交换机初始化完成")
	return nil
}

// initializeExchanges 初始化交换机
func (qi *QueueInitializer) initializeExchanges() error {
	exchanges := GetExchangeConfigs()

	for _, exchange := range exchanges {
		err := qi.client.DeclareExchange(
			exchange.Name,
			exchange.Type,
			exchange.Durable,
			exchange.AutoDelete,
			exchange.Internal,
			exchange.NoWait,
			exchange.Args,
		)
		if err != nil {
			return fmt.Errorf("声明交换机 %s 失败: %w", exchange.Name, err)
		}
	}

	return nil
}

// initializeQueues 初始化队列
func (qi *QueueInitializer) initializeQueues() error {
	queues := GetQueueConfigs()

	for _, queue := range queues {
		err := qi.declareQueueWithRetry(queue)
		if err != nil {
			return fmt.Errorf("声明队列 %s 失败: %w", queue.Name, err)
		}
	}

	return nil
}

// declareQueueWithRetry 声明队列，如果参数不匹配则尝试删除后重新创建
func (qi *QueueInitializer) declareQueueWithRetry(queue QueueConfig) error {
	// 首次尝试声明队列
	err := qi.client.DeclareQueue(
		queue.Name,
		queue.Durable,
		queue.AutoDelete,
		queue.Exclusive,
		queue.NoWait,
		queue.Args,
	)

	// 如果成功或者不是参数不匹配错误，直接返回
	if err == nil {
		qi.logger.Infof("✅ 队列 %s 声明成功", queue.Name)
		return nil
	}

	// 检查是否是参数不匹配错误
	if amqpErr, ok := err.(*amqp.Error); ok && amqpErr.Code == 406 {
		qi.logger.Warnf("⚠️  队列 %s 参数不匹配，尝试删除后重新创建", queue.Name)

		// 尝试删除队列
		if deleteErr := qi.client.DeleteQueue(queue.Name, false, false, false); deleteErr != nil {
			qi.logger.Warnf("删除队列 %s 失败: %v，继续尝试重新声明", queue.Name, deleteErr)
		} else {
			qi.logger.Infof("🗑️  队列 %s 删除成功", queue.Name)
		}

		// 重新声明队列
		err = qi.client.DeclareQueue(
			queue.Name,
			queue.Durable,
			queue.AutoDelete,
			queue.Exclusive,
			queue.NoWait,
			queue.Args,
		)

		if err == nil {
			qi.logger.Infof("✅ 队列 %s 重新创建成功", queue.Name)
		}
	}

	return err
}

// initializeBindings 初始化绑定
func (qi *QueueInitializer) initializeBindings() error {
	bindings := GetBindingConfigs()

	for _, binding := range bindings {
		err := qi.client.BindQueue(
			binding.QueueName,
			binding.RoutingKey,
			binding.ExchangeName,
			binding.NoWait,
			binding.Args,
		)
		if err != nil {
			return fmt.Errorf("绑定队列 %s 到交换机 %s 失败: %w",
				binding.QueueName, binding.ExchangeName, err)
		}
	}

	return nil
}
