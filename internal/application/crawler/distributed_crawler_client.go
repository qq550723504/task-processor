// Package crawler 提供分布式爬虫客户端
package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/domain/model"
	"task-processor/internal/domain/task"
	"task-processor/internal/infra/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// DistributedCrawlerClient 分布式爬虫客户端
// 用于在现有task-processor中调用分布式爬虫服务
type DistributedCrawlerClient struct {
	rabbitmqClient *rabbitmq.Client
	taskAdapter    *task.MessageAdapter
	logger         *logrus.Logger

	// 结果等待管理
	pendingTasks map[string]*PendingTask
	mutex        sync.RWMutex

	// 配置
	timeout time.Duration
}

// PendingTask 等待中的任务
type PendingTask struct {
	TaskID     string
	ResultChan chan *CrawlResult
	CreatedAt  time.Time
	Context    context.Context
	Cancel     context.CancelFunc
}

// CrawlRequest 爬虫请求
type CrawlRequest struct {
	TaskID    int64  `json:"taskId"`
	TenantID  int64  `json:"tenantId"`
	StoreID   int64  `json:"storeId"`
	Platform  string `json:"platform"`
	Region    string `json:"region"`
	ProductID string `json:"productId"`
	URL       string `json:"url"`
	Zipcode   string `json:"zipcode"`
	Priority  int    `json:"priority"`
}

// CrawlResult 爬虫结果
type CrawlResult struct {
	TaskID   int64          `json:"taskId"`
	Success  bool           `json:"success"`
	Product  *model.Product `json:"product,omitempty"`
	Error    string         `json:"error,omitempty"`
	Duration time.Duration  `json:"duration"`
	NodeID   string         `json:"nodeId"`
}

// NewDistributedCrawlerClient 创建分布式爬虫客户端
func NewDistributedCrawlerClient(rabbitmqURL string, logger *logrus.Logger) (*DistributedCrawlerClient, error) {
	// 创建连接配置
	connConfig := rabbitmq.ConnectionConfig{
		URL:               rabbitmqURL,
		ReconnectInterval: 5 * time.Second,
		MaxReconnectTries: 10,
	}

	// 创建连接管理器
	connManager := rabbitmq.NewConnectionManager(connConfig, logger)

	// 创建RabbitMQ客户端
	rabbitmqClient := rabbitmq.NewClient(connManager, logger)

	client := &DistributedCrawlerClient{
		rabbitmqClient: rabbitmqClient,
		taskAdapter:    task.NewMessageAdapter(),
		logger:         logger,
		pendingTasks:   make(map[string]*PendingTask),
		timeout:        5 * time.Minute, // 默认5分钟超时
	}

	// 启动结果监听
	if err := client.startResultListener(); err != nil {
		return nil, fmt.Errorf("启动结果监听失败: %w", err)
	}

	return client, nil
}

// SubmitCrawlTask 提交爬虫任务并等待结果
func (c *DistributedCrawlerClient) SubmitCrawlTask(ctx context.Context, req *CrawlRequest) (*CrawlResult, error) {
	c.logger.Infof("提交爬虫任务: TaskID=%d, ProductID=%s", req.TaskID, req.ProductID)

	// 创建任务对象
	taskModel := &model.Task{
		ID:         req.TaskID,
		TenantID:   req.TenantID,
		StoreID:    req.StoreID,
		Platform:   req.Platform,
		Region:     req.Region,
		ProductID:  req.ProductID,
		Priority:   req.Priority,
		CreateTime: time.Now().Unix(),
		UpdateTime: time.Now().Unix(),
	}

	// 转换为任务消息
	taskMessage, err := c.taskAdapter.TaskToMessage(taskModel)
	if err != nil {
		return nil, fmt.Errorf("转换任务消息失败: %w", err)
	}

	// 添加爬虫特定信息到 TaskMessage
	// 注意：这里需要将 TaskMessage 序列化为 JSON，然后添加额外字段
	messageData := make(map[string]interface{})

	// 将 TaskMessage 转换为 map
	taskBytes, _ := json.Marshal(taskMessage)
	json.Unmarshal(taskBytes, &messageData)

	// 添加爬虫特定字段
	messageData["url"] = req.URL
	messageData["zipcode"] = req.Zipcode

	// 构建路由键和队列名称
	routingKey := c.taskAdapter.BuildRoutingKey(taskModel)
	queueName := c.taskAdapter.GetQueueName(req.Platform)

	// 创建等待任务
	taskCtx, cancel := context.WithTimeout(ctx, c.timeout)
	pendingTask := &PendingTask{
		TaskID:     fmt.Sprintf("%d", req.TaskID),
		ResultChan: make(chan *CrawlResult, 1),
		CreatedAt:  time.Now(),
		Context:    taskCtx,
		Cancel:     cancel,
	}

	// 注册等待任务
	c.mutex.Lock()
	c.pendingTasks[pendingTask.TaskID] = pendingTask
	c.mutex.Unlock()

	// 创建 RabbitMQ 消息
	message := &rabbitmq.Message{
		ID:         fmt.Sprintf("%d", req.TaskID),
		Type:       "task",
		Payload:    messageData,
		Priority:   c.taskAdapter.CalculatePriority(req.Priority),
		Timestamp:  time.Now().Unix(),
		RetryCount: 0,
		MaxRetries: 3,
	}

	// 发送消息到RabbitMQ
	publishOpts := rabbitmq.PublishOptions{
		Exchange:   "", // 使用默认交换机
		RoutingKey: queueName,
		Priority:   message.Priority,
		Persistent: true,
		Mandatory:  false,
		Immediate:  false,
	}

	err = c.rabbitmqClient.Publish(context.Background(), message, publishOpts)
	if err != nil {
		// 清理等待任务
		c.mutex.Lock()
		delete(c.pendingTasks, pendingTask.TaskID)
		c.mutex.Unlock()
		cancel()
		return nil, fmt.Errorf("发送爬虫任务失败: %w", err)
	}

	c.logger.Infof("爬虫任务已发送: TaskID=%d, Queue=%s, RoutingKey=%s",
		req.TaskID, queueName, routingKey)

	// 等待结果
	select {
	case result := <-pendingTask.ResultChan:
		c.logger.Infof("收到爬虫结果: TaskID=%d, Success=%v", req.TaskID, result.Success)
		return result, nil
	case <-taskCtx.Done():
		// 清理等待任务
		c.mutex.Lock()
		delete(c.pendingTasks, pendingTask.TaskID)
		c.mutex.Unlock()
		return nil, fmt.Errorf("爬虫任务超时: TaskID=%d", req.TaskID)
	}
}

// startResultListener 启动结果监听器
func (c *DistributedCrawlerClient) startResultListener() error {
	// 为每个客户端创建唯一的结果队列
	nodeID := fmt.Sprintf("node-%d", time.Now().UnixNano())
	resultQueueName := fmt.Sprintf("crawler.results.%s", nodeID)

	// 声明队列（临时队列，客户端断开后自动删除）
	err := c.rabbitmqClient.DeclareQueue(resultQueueName, false, true, true, false, nil)
	if err != nil {
		return fmt.Errorf("声明结果队列失败: %w", err)
	}

	// 开始消费结果消息
	go c.consumeResults(resultQueueName)

	c.logger.Infof("爬虫结果监听器已启动，队列: %s", resultQueueName)
	return nil
}

// consumeResults 消费结果消息
func (c *DistributedCrawlerClient) consumeResults(queueName string) {
	consumeOpts := rabbitmq.ConsumeOptions{
		Queue:     queueName,
		Consumer:  "",
		AutoAck:   false,
		Exclusive: false,
		NoLocal:   false,
		NoWait:    false,
		Args:      nil,
	}

	msgs, err := c.rabbitmqClient.Consume(context.Background(), consumeOpts)
	if err != nil {
		c.logger.Errorf("开始消费结果消息失败: %v", err)
		return
	}

	for msg := range msgs {
		c.handleResultMessage(msg)
	}
}

// handleResultMessage 处理结果消息
func (c *DistributedCrawlerClient) handleResultMessage(msg amqp.Delivery) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("处理结果消息时发生panic: %v", r)
		}
	}()

	// 解析结果消息
	var result CrawlResult
	if err := json.Unmarshal(msg.Body, &result); err != nil {
		c.logger.Errorf("解析结果消息失败: %v", err)
		c.rabbitmqClient.NackMessage(msg, false) // 拒绝消息，不重新入队
		return
	}

	taskID := fmt.Sprintf("%d", result.TaskID)
	c.logger.Debugf("收到爬虫结果消息: TaskID=%s", taskID)

	// 查找等待的任务
	c.mutex.RLock()
	pendingTask, exists := c.pendingTasks[taskID]
	c.mutex.RUnlock()

	if !exists {
		c.logger.Warnf("未找到等待的任务: TaskID=%s", taskID)
		c.rabbitmqClient.AckMessage(msg) // 确认消息
		return
	}

	// 发送结果到等待的任务
	select {
	case pendingTask.ResultChan <- &result:
		c.logger.Debugf("结果已发送到等待任务: TaskID=%s", taskID)
	case <-pendingTask.Context.Done():
		c.logger.Warnf("等待任务已超时: TaskID=%s", taskID)
	default:
		c.logger.Warnf("结果通道已满: TaskID=%s", taskID)
	}

	// 清理等待任务
	c.mutex.Lock()
	delete(c.pendingTasks, taskID)
	c.mutex.Unlock()
	pendingTask.Cancel()

	// 确认消息
	c.rabbitmqClient.AckMessage(msg)
}

// SetTimeout 设置超时时间
func (c *DistributedCrawlerClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// GetStats 获取统计信息
func (c *DistributedCrawlerClient) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return map[string]interface{}{
		"pending_tasks": len(c.pendingTasks),
		"timeout":       c.timeout.String(),
	}
}

// Close 关闭客户端
func (c *DistributedCrawlerClient) Close() error {
	c.logger.Info("关闭分布式爬虫客户端")

	// 取消所有等待的任务
	c.mutex.Lock()
	for _, pendingTask := range c.pendingTasks {
		pendingTask.Cancel()
	}
	c.pendingTasks = make(map[string]*PendingTask)
	c.mutex.Unlock()

	// 关闭RabbitMQ客户端
	if c.rabbitmqClient != nil {
		return c.rabbitmqClient.Close()
	}

	return nil
}
