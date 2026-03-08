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

// QueueConfig 队列配置
type QueueConfig struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       amqp.Table
}

// ExchangeConfig 交换机配置
type ExchangeConfig struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp.Table
}

// BindingConfig 绑定配置
type BindingConfig struct {
	QueueName    string
	ExchangeName string
	RoutingKey   string
	NoWait       bool
	Args         amqp.Table
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
	exchanges := []ExchangeConfig{
		{
			Name:       "tasks.exchange",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Args:       nil,
		},
		{
			Name:       "tasks.dlx",
			Type:       "direct",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Args:       nil,
		},
		{
			Name:       "tasks.delay.exchange",
			Type:       "direct",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Args:       nil,
		},
		{
			Name:       "tasks.result.exchange",
			Type:       "direct",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Args:       nil,
		},
	}

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
	queues := []QueueConfig{
		// Amazon 优先级队列
		{
			Name:       "amazon.tasks.high",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
		{
			Name:       "amazon.tasks.normal",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
		{
			Name:       "amazon.tasks.low",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
		// TEMU 优先级队列
		{
			Name:       "temu.tasks.high",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
		{
			Name:       "temu.tasks.normal",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
		{
			Name:       "temu.tasks.low",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
		// SHEIN 优先级队列
		{
			Name:       "shein.tasks.high",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
		{
			Name:       "shein.tasks.normal",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
		{
			Name:       "shein.tasks.low",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-max-priority":            10,
				"x-dead-letter-exchange":    "tasks.dlx",
				"x-dead-letter-routing-key": "failed",
			},
		},
		// 死信队列
		{
			Name:       "tasks.dlq",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-message-ttl": 86400000, // 24小时TTL
			},
		},
		// 延迟队列
		{
			Name:       "tasks.delay.queue",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args: amqp.Table{
				"x-dead-letter-exchange":    "tasks.exchange",
				"x-dead-letter-routing-key": "retry",
			},
		},
		// 结果队列
		{
			Name:       "tasks.result.queue",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Args:       nil,
		},
	}

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
	bindings := []BindingConfig{
		// Amazon 任务队列绑定
		{
			QueueName:    "amazon.tasks.high",
			ExchangeName: "tasks.exchange",
			RoutingKey:   "amazon.high.#",
			NoWait:       false,
			Args:         nil,
		},
		{
			QueueName:    "amazon.tasks.normal",
			ExchangeName: "tasks.exchange",
			RoutingKey:   "amazon.normal.#",
			NoWait:       false,
			Args:         nil,
		},
		{
			QueueName:    "amazon.tasks.low",
			ExchangeName: "tasks.exchange",
			RoutingKey:   "amazon.low.#",
			NoWait:       false,
			Args:         nil,
		},
		// TEMU 任务队列绑定
		{
			QueueName:    "temu.tasks.high",
			ExchangeName: "tasks.exchange",
			RoutingKey:   "temu.high.#",
			NoWait:       false,
			Args:         nil,
		},
		{
			QueueName:    "temu.tasks.normal",
			ExchangeName: "tasks.exchange",
			RoutingKey:   "temu.normal.#",
			NoWait:       false,
			Args:         nil,
		},
		{
			QueueName:    "temu.tasks.low",
			ExchangeName: "tasks.exchange",
			RoutingKey:   "temu.low.#",
			NoWait:       false,
			Args:         nil,
		},
		// SHEIN 任务队列绑定
		{
			QueueName:    "shein.tasks.high",
			ExchangeName: "tasks.exchange",
			RoutingKey:   "shein.high.#",
			NoWait:       false,
			Args:         nil,
		},
		{
			QueueName:    "shein.tasks.normal",
			ExchangeName: "tasks.exchange",
			RoutingKey:   "shein.normal.#",
			NoWait:       false,
			Args:         nil,
		},
		{
			QueueName:    "shein.tasks.low",
			ExchangeName: "tasks.exchange",
			RoutingKey:   "shein.low.#",
			NoWait:       false,
			Args:         nil,
		},
		// 死信队列绑定
		{
			QueueName:    "tasks.dlq",
			ExchangeName: "tasks.dlx",
			RoutingKey:   "failed",
			NoWait:       false,
			Args:         nil,
		},
		// 延迟队列绑定
		{
			QueueName:    "tasks.delay.queue",
			ExchangeName: "tasks.delay.exchange",
			RoutingKey:   "retry",
			NoWait:       false,
			Args:         nil,
		},
		// 结果队列绑定
		{
			QueueName:    "tasks.result.queue",
			ExchangeName: "tasks.result.exchange",
			RoutingKey:   "result",
			NoWait:       false,
			Args:         nil,
		},
	}

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
