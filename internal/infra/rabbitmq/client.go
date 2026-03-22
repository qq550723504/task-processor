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
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Payload    map[string]any `json:"payload"`
	Priority   uint8          `json:"priority"`
	Timestamp  int64          `json:"timestamp"`
	RetryCount int            `json:"retry_count"`
	MaxRetries int            `json:"max_retries"`
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
// 每次使用独立 channel，避免 AMQP 协议下 channel 出错后影响后续操作
func (c *Client) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error {
	channel, err := c.connManager.CreateChannel()
	if err != nil {
		return fmt.Errorf("创建通道失败: %w", err)
	}
	defer channel.Close()

	_, err = channel.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
	if err != nil {
		return fmt.Errorf("声明队列 %s 失败: %w", name, err)
	}

	c.logger.Infof("队列 %s 声明成功", name)
	return nil
}

// DeclareExchange 声明交换机
func (c *Client) DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	channel, err := c.connManager.CreateChannel()
	if err != nil {
		return fmt.Errorf("创建通道失败: %w", err)
	}
	defer channel.Close()

	err = channel.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
	if err != nil {
		return fmt.Errorf("声明交换机 %s 失败: %w", name, err)
	}

	c.logger.Infof("交换机 %s 声明成功", name)
	return nil
}

// DeleteQueue 删除队列
func (c *Client) DeleteQueue(name string, ifUnused, ifEmpty, noWait bool) error {
	channel, err := c.connManager.CreateChannel()
	if err != nil {
		return fmt.Errorf("创建通道失败: %w", err)
	}
	defer channel.Close()

	_, err = channel.QueueDelete(name, ifUnused, ifEmpty, noWait)
	if err != nil {
		return fmt.Errorf("删除队列 %s 失败: %w", name, err)
	}

	c.logger.Infof("队列 %s 删除成功", name)
	return nil
}

// BindQueue 绑定队列到交换机
func (c *Client) BindQueue(queueName, routingKey, exchangeName string, noWait bool, args amqp.Table) error {
	channel, err := c.connManager.CreateChannel()
	if err != nil {
		return fmt.Errorf("创建通道失败: %w", err)
	}
	defer channel.Close()

	err = channel.QueueBind(queueName, routingKey, exchangeName, noWait, args)
	if err != nil {
		return fmt.Errorf("绑定队列 %s 到交换机 %s 失败: %w", queueName, exchangeName, err)
	}

	c.logger.Infof("队列 %s 绑定到交换机 %s 成功，路由键: %s", queueName, exchangeName, routingKey)
	return nil
}

// Publish 发布消息
func (c *Client) Publish(ctx context.Context, msg *Message, opts PublishOptions) error {
	// 每次发布使用独立 channel，避免并发写共享 channel 导致 UNEXPECTED_FRAME
	channel, err := c.connManager.CreateChannel()
	if err != nil {
		return fmt.Errorf("创建发布通道失败: %w", err)
	}
	defer channel.Close()

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
	// 消费者使用独立 channel，避免与发布操作共享 channel
	channel, err := c.connManager.CreateChannel()
	if err != nil {
		return nil, fmt.Errorf("创建消费通道失败: %w", err)
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
		channel.Close()
		return nil, fmt.Errorf("开始消费队列 %s 失败: %w", opts.Queue, err)
	}

	c.logger.Infof("开始消费队列: %s, 消费者: %s", opts.Queue, opts.Consumer)
	return deliveries, nil
}

// ParseMessage 解析消息（基础设施层）
// 只负责从 amqp.Delivery 解析为基础的 Message 结构
// 业务逻辑的转换由领域层的 MessageAdapter 处理
func (c *Client) ParseMessage(delivery amqp.Delivery) (*Message, error) {
	// 尝试解析为标准格式
	var msg Message
	err := json.Unmarshal(delivery.Body, &msg)

	// 如果解析成功且包含 payload 字段，说明是标准嵌套格式
	if err == nil && msg.Payload != nil && len(msg.Payload) > 0 {
		return &msg, nil
	}

	// 否则，尝试解析为扁平格式（整个消息体作为 payload）
	var flatMsg map[string]any
	err = json.Unmarshal(delivery.Body, &flatMsg)
	if err != nil {
		return nil, fmt.Errorf("解析消息失败: %w", err)
	}

	// 构建标准格式的消息
	msg = Message{
		ID:         delivery.MessageId,
		Type:       delivery.Type,
		Payload:    flatMsg,
		Priority:   delivery.Priority,
		Timestamp:  delivery.Timestamp.Unix(),
		RetryCount: 0,
		MaxRetries: 3,
	}

	// 如果消息ID为空，生成一个
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
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

// GetConnectionManager 获取连接管理器
func (c *Client) GetConnectionManager() *ConnectionManager {
	return c.connManager
}

// Close 关闭客户端
func (c *Client) Close() error {
	return c.connManager.Close()
}
