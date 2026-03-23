// Package rabbitmq 提供 RabbitMQ 队列初始化功能
package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// QueueInitializer 队列初始化器
type QueueInitializer struct {
	client *Client
	logger *logrus.Logger
}

// NewQueueInitializer 创建队列初始化器
func NewQueueInitializer(client *Client, logger *logrus.Logger) *QueueInitializer {
	return &QueueInitializer{client: client, logger: logger}
}

// InitializeAll 初始化所有队列和交换机
func (qi *QueueInitializer) InitializeAll() error {
	qi.logger.Info("开始初始化RabbitMQ队列和交换机...")
	if err := qi.initializeExchanges(); err != nil {
		return fmt.Errorf("初始化交换机失败: %w", err)
	}
	if err := qi.initializeQueues(); err != nil {
		return fmt.Errorf("初始化队列失败: %w", err)
	}
	if err := qi.initializeBindings(); err != nil {
		return fmt.Errorf("初始化绑定失败: %w", err)
	}
	qi.logger.Info("RabbitMQ队列和交换机初始化完成")
	return nil
}

func (qi *QueueInitializer) initializeExchanges() error {
	for _, ex := range GetExchangeConfigs() {
		if err := qi.client.DeclareExchange(ex.Name, ex.Type, ex.Durable, ex.AutoDelete, ex.Internal, ex.NoWait, ex.Args); err != nil {
			return fmt.Errorf("声明交换机 %s 失败: %w", ex.Name, err)
		}
	}
	return nil
}

func (qi *QueueInitializer) initializeQueues() error {
	for _, q := range GetQueueDeclareConfigs() {
		if err := qi.declareQueueWithRetry(q); err != nil {
			return fmt.Errorf("声明队列 %s 失败: %w", q.Name, err)
		}
	}
	return nil
}

// InitializeRegionCrawlerQueues 为指定 region 列表初始化爬虫队列
func (qi *QueueInitializer) InitializeRegionCrawlerQueues(regions []string) error {
	for _, q := range GetRegionCrawlerQueueDeclareConfigs(regions) {
		if err := qi.declareQueueWithRetry(q); err != nil {
			return fmt.Errorf("声明 region 爬虫队列 %s 失败: %w", q.Name, err)
		}
	}
	qi.logger.Infof("region 爬虫队列初始化完成: regions=%v", regions)
	return nil
}

// InitializeStoreQueues 为指定平台和店铺列表初始化专属队列和绑定
func (qi *QueueInitializer) InitializeStoreQueues(platform string, storeIDs []int64) error {
	for _, storeID := range storeIDs {
		for _, q := range GetStoreQueueDeclareConfigs(platform, storeID) {
			if err := qi.declareQueueWithRetry(q); err != nil {
				return fmt.Errorf("声明店铺队列 %s 失败: %w", q.Name, err)
			}
		}
		for _, b := range GetStoreQueueBindingConfigs(platform, storeID) {
			if err := qi.client.BindQueue(b.QueueName, b.RoutingKey, b.ExchangeName, b.NoWait, b.Args); err != nil {
				return fmt.Errorf("绑定店铺队列 %s 失败: %w", b.QueueName, err)
			}
			qi.logger.Infof("店铺队列绑定成功: queue=%s, routingKey=%s", b.QueueName, b.RoutingKey)
		}
	}
	return nil
}
func (qi *QueueInitializer) declareQueueWithRetry(q QueueDeclareConfig) error {
	err := qi.client.DeclareQueue(q.Name, q.Durable, q.AutoDelete, q.Exclusive, q.NoWait, q.Args)
	if err == nil {
		qi.logger.Infof("队列 %s 声明成功", q.Name)
		return nil
	}

	amqpErr, ok := err.(*amqp.Error)
	if !ok || amqpErr.Code != 406 {
		return err
	}

	qi.logger.Warnf("队列 %s 参数不匹配，尝试删除后重新创建", q.Name)
	if deleteErr := qi.client.DeleteQueue(q.Name, false, false, false); deleteErr != nil {
		qi.logger.Warnf("删除队列 %s 失败: %v，继续尝试重新声明", q.Name, deleteErr)
	}

	err = qi.client.DeclareQueue(q.Name, q.Durable, q.AutoDelete, q.Exclusive, q.NoWait, q.Args)
	if err == nil {
		qi.logger.Infof("队列 %s 重新创建成功", q.Name)
	}
	return err
}

func (qi *QueueInitializer) initializeBindings() error {
	for _, b := range GetBindingConfigs() {
		if err := qi.client.BindQueue(b.QueueName, b.RoutingKey, b.ExchangeName, b.NoWait, b.Args); err != nil {
			return fmt.Errorf("绑定队列 %s 到交换机 %s 失败: %w", b.QueueName, b.ExchangeName, err)
		}
	}
	return nil
}
