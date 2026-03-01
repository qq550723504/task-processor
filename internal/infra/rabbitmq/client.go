// Package rabbitmq 提供RabbitMQ客户端功能
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// Client RabbitMQ客户端
type Client struct {
	connManager *ConnectionManager
	logger      *logrus.Logger
}

// Message 消息结构
type Message struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Payload    map[string]interface{} `json:"payload"`
	Priority   uint8                  `json:"priority"`
	Timestamp  int64                  `json:"timestamp"`
	RetryCount int                    `json:"retry_count"`
	MaxRetries int                    `json:"max_retries"`
}

// PublishOptions 发布选项
type PublishOptions struct {
	Exchange   string
	RoutingKey string
	Priority   uint8
	Persistent bool
	Mandatory  bool
	Immediate  bool
}

// ConsumeOptions 消费选项
type ConsumeOptions struct {
	Queue     string
	Consumer  string
	AutoAck   bool
	Exclusive bool
	NoLocal   bool
	NoWait    bool
	Args      amqp.Table
}

// NewClient 创建RabbitMQ客户端
func NewClient(connManager *ConnectionManager, logger *logrus.Logger) *Client {
	return &Client{
		connManager: connManager,
		logger:      logger,
	}
}

// DeclareQueue 声明队列
func (c *Client) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error {
	channel, err := c.connManager.GetChannel()
	if err != nil {
		return fmt.Errorf("获取通道失败: %w", err)
	}

	_, err = channel.QueueDeclare(
		name,       // 队列名称
		durable,    // 持久化
		autoDelete, // 自动删除
		exclusive,  // 排他性
		noWait,     // 不等待
		args,       // 参数
	)

	if err != nil {
		return fmt.Errorf("声明队列 %s 失败: %w", name, err)
	}

	c.logger.Infof("队列 %s 声明成功", name)
	return nil
}

// DeclareExchange 声明交换机
func (c *Client) DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	channel, err := c.connManager.GetChannel()
	if err != nil {
		return fmt.Errorf("获取通道失败: %w", err)
	}

	err = channel.ExchangeDeclare(
		name,       // 交换机名称
		kind,       // 交换机类型
		durable,    // 持久化
		autoDelete, // 自动删除
		internal,   // 内部使用
		noWait,     // 不等待
		args,       // 参数
	)

	if err != nil {
		return fmt.Errorf("声明交换机 %s 失败: %w", name, err)
	}

	c.logger.Infof("交换机 %s 声明成功", name)
	return nil
}

// DeleteQueue 删除队列
func (c *Client) DeleteQueue(name string, ifUnused, ifEmpty, noWait bool) error {
	channel, err := c.connManager.GetChannel()
	if err != nil {
		return fmt.Errorf("获取通道失败: %w", err)
	}

	_, err = channel.QueueDelete(
		name,     // 队列名称
		ifUnused, // 仅当未使用时删除
		ifEmpty,  // 仅当为空时删除
		noWait,   // 不等待
	)

	if err != nil {
		return fmt.Errorf("删除队列 %s 失败: %w", name, err)
	}

	c.logger.Infof("队列 %s 删除成功", name)
	return nil
}

// BindQueue 绑定队列到交换机
func (c *Client) BindQueue(queueName, routingKey, exchangeName string, noWait bool, args amqp.Table) error {
	channel, err := c.connManager.GetChannel()
	if err != nil {
		return fmt.Errorf("获取通道失败: %w", err)
	}

	err = channel.QueueBind(
		queueName,    // 队列名称
		routingKey,   // 路由键
		exchangeName, // 交换机名称
		noWait,       // 不等待
		args,         // 参数
	)

	if err != nil {
		return fmt.Errorf("绑定队列 %s 到交换机 %s 失败: %w", queueName, exchangeName, err)
	}

	c.logger.Infof("队列 %s 绑定到交换机 %s 成功，路由键: %s", queueName, exchangeName, routingKey)
	return nil
}

// Publish 发布消息
func (c *Client) Publish(ctx context.Context, msg *Message, opts PublishOptions) error {
	channel, err := c.connManager.GetChannel()
	if err != nil {
		return fmt.Errorf("获取通道失败: %w", err)
	}

	// 序列化消息
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 构建发布消息
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		Priority:     opts.Priority,
		Timestamp:    time.Now(),
		MessageId:    msg.ID,
		DeliveryMode: 1, // 非持久化
	}

	// 如果需要持久化
	if opts.Persistent {
		publishing.DeliveryMode = 2
	}

	// 发布消息
	err = channel.PublishWithContext(
		ctx,
		opts.Exchange,   // 交换机
		opts.RoutingKey, // 路由键
		opts.Mandatory,  // 强制
		opts.Immediate,  // 立即
		publishing,
	)

	if err != nil {
		return fmt.Errorf("发布消息失败: %w", err)
	}

	c.logger.Debugf("消息发布成功: ID=%s, Exchange=%s, RoutingKey=%s",
		msg.ID, opts.Exchange, opts.RoutingKey)
	return nil
}

// Consume 消费消息
func (c *Client) Consume(ctx context.Context, opts ConsumeOptions) (<-chan amqp.Delivery, error) {
	channel, err := c.connManager.GetChannel()
	if err != nil {
		return nil, fmt.Errorf("获取通道失败: %w", err)
	}

	// 设置QoS
	err = channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return nil, fmt.Errorf("设置QoS失败: %w", err)
	}

	// 开始消费
	deliveries, err := channel.Consume(
		opts.Queue,     // 队列名称
		opts.Consumer,  // 消费者标签
		opts.AutoAck,   // 自动确认
		opts.Exclusive, // 排他性
		opts.NoLocal,   // 不接收本地消息
		opts.NoWait,    // 不等待
		opts.Args,      // 参数
	)

	if err != nil {
		return nil, fmt.Errorf("开始消费队列 %s 失败: %w", opts.Queue, err)
	}

	c.logger.Infof("开始消费队列: %s, 消费者: %s", opts.Queue, opts.Consumer)
	return deliveries, nil
}

// ParseMessage 解析消息
func (c *Client) ParseMessage(delivery amqp.Delivery) (*Message, error) {
	var msg Message
	err := json.Unmarshal(delivery.Body, &msg)
	if err != nil {
		return nil, fmt.Errorf("解析消息失败: %w", err)
	}
	return &msg, nil
}

// AckMessage 确认消息
func (c *Client) AckMessage(delivery amqp.Delivery) error {
	err := delivery.Ack(false)
	if err != nil {
		return fmt.Errorf("确认消息失败: %w", err)
	}
	return nil
}

// NackMessage 拒绝消息
func (c *Client) NackMessage(delivery amqp.Delivery, requeue bool) error {
	err := delivery.Nack(false, requeue)
	if err != nil {
		return fmt.Errorf("拒绝消息失败: %w", err)
	}
	return nil
}

// RejectMessage 拒绝消息
func (c *Client) RejectMessage(delivery amqp.Delivery, requeue bool) error {
	err := delivery.Reject(requeue)
	if err != nil {
		return fmt.Errorf("拒绝消息失败: %w", err)
	}
	return nil
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
	return c.connManager.IsConnected()
}

// Close 关闭客户端
func (c *Client) Close() error {
	return c.connManager.Close()
}
