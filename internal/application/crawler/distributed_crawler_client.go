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

// NewDistributedCrawlerClient 创建分布式爬虫客户端（使用已存在的RabbitMQ客户端）
func NewDistributedCrawlerClient(rabbitmqClient *rabbitmq.Client, logger *logrus.Logger) (*DistributedCrawlerClient, error) {
	if rabbitmqClient == nil {
		return nil, fmt.Errorf("RabbitMQ客户端不能为空")
	}

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
	messageData := c.buildMessageData(taskMessage, req)

	// 构建路由键和队列名称
	// 注意：爬虫任务使用专门的爬虫队列，不是上架队列
	routingKey := c.taskAdapter.BuildRoutingKey(taskModel)
	queueName := c.taskAdapter.GetQueueName(req.Platform + ".crawler")

	// 创建等待任务
	pendingTask := c.createPendingTask(ctx, req.TaskID)

	// 创建并发送 RabbitMQ 消息
	if err := c.publishTask(taskModel, messageData, queueName, routingKey, pendingTask); err != nil {
		return nil, err
	}

	// 等待结果
	return c.waitForResult(pendingTask, req.TaskID)
}

// buildMessageData 构建消息数据
func (c *DistributedCrawlerClient) buildMessageData(taskMessage interface{}, req *CrawlRequest) map[string]interface{} {
	messageData := make(map[string]interface{})

	// 将 TaskMessage 转换为 map
	taskBytes, _ := json.Marshal(taskMessage)
	json.Unmarshal(taskBytes, &messageData)

	// 添加爬虫特定字段
	messageData["url"] = req.URL
	messageData["zipcode"] = req.Zipcode

	return messageData
}

// createPendingTask 创建等待任务
func (c *DistributedCrawlerClient) createPendingTask(ctx context.Context, taskID int64) *PendingTask {
	taskCtx, cancel := context.WithTimeout(ctx, c.timeout)
	pendingTask := &PendingTask{
		TaskID:     fmt.Sprintf("%d", taskID),
		ResultChan: make(chan *CrawlResult, 1),
		CreatedAt:  time.Now(),
		Context:    taskCtx,
		Cancel:     cancel,
	}

	// 注册等待任务
	c.mutex.Lock()
	c.pendingTasks[pendingTask.TaskID] = pendingTask
	c.mutex.Unlock()

	return pendingTask
}

// publishTask 发布任务到RabbitMQ
func (c *DistributedCrawlerClient) publishTask(
	taskModel *model.Task,
	messageData map[string]interface{},
	queueName, routingKey string,
	pendingTask *PendingTask,
) error {
	// 创建 RabbitMQ 消息
	message := &rabbitmq.Message{
		ID:         fmt.Sprintf("%d", taskModel.ID),
		Type:       "task",
		Payload:    messageData,
		Priority:   c.taskAdapter.CalculatePriority(taskModel.Priority),
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

	err := c.rabbitmqClient.Publish(context.Background(), message, publishOpts)
	if err != nil {
		// 清理等待任务
		c.mutex.Lock()
		delete(c.pendingTasks, pendingTask.TaskID)
		c.mutex.Unlock()
		pendingTask.Cancel()
		return fmt.Errorf("发送爬虫任务失败: %w", err)
	}

	c.logger.Infof("爬虫任务已发送: TaskID=%s, Queue=%s, RoutingKey=%s",
		pendingTask.TaskID, queueName, routingKey)

	return nil
}

// waitForResult 等待任务结果
func (c *DistributedCrawlerClient) waitForResult(pendingTask *PendingTask, taskID int64) (*CrawlResult, error) {
	select {
	case result := <-pendingTask.ResultChan:
		c.logger.Infof("收到爬虫结果: TaskID=%d, Success=%v", taskID, result.Success)
		return result, nil
	case <-pendingTask.Context.Done():
		// 清理等待任务
		c.mutex.Lock()
		delete(c.pendingTasks, pendingTask.TaskID)
		c.mutex.Unlock()
		return nil, fmt.Errorf("爬虫任务超时: TaskID=%d", taskID)
	}
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
