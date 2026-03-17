// Package distributed 提供分布式爬虫客户端
package distributed

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/model"
	"task-processor/internal/domain/queue"
	"task-processor/internal/domain/task"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

// DistributedCrawlerClient 分布式爬虫客户端
// 用于在现有task-processor中调用分布式爬虫服务
type DistributedCrawlerClient struct {
	rabbitmqClient *rabbitmq.Client
	taskAdapter    *task.MessageAdapter
	queueNaming    *queue.NamingService
	logger         *logrus.Logger

	// 结果等待管理
	pendingTasks map[string]*PendingTask
	mutex        sync.RWMutex

	// 配置
	timeout time.Duration

	// 懒加载标志
	listenerStarted bool
	listenerMutex   sync.Mutex
	resultQueueName string // 结果队列名称
}

// NewDistributedCrawlerClient 创建分布式爬虫客户端（使用已存在的RabbitMQ客户端）
func NewDistributedCrawlerClient(rabbitmqClient *rabbitmq.Client, logger *logrus.Logger) (*DistributedCrawlerClient, error) {
	if rabbitmqClient == nil {
		return nil, fmt.Errorf("RabbitMQ客户端不能为空")
	}

	client := &DistributedCrawlerClient{
		rabbitmqClient:  rabbitmqClient,
		taskAdapter:     task.NewMessageAdapter(),
		queueNaming:     queue.NewNamingService(),
		logger:          logger,
		pendingTasks:    make(map[string]*PendingTask),
		timeout:         5 * time.Minute, // 默认5分钟超时
		listenerStarted: false,
	}

	// 不在构造函数中启动监听器，改为懒加载
	// 在第一次提交任务时启动
	logger.Info("分布式爬虫客户端创建成功（结果监听器将在首次使用时启动）")

	return client, nil
}

// SubmitCrawlTask 提交爬虫任务并等待结果
func (c *DistributedCrawlerClient) SubmitCrawlTask(ctx context.Context, req *CrawlRequest) (*CrawlResult, error) {
	c.logger.Infof("提交爬虫任务: TaskID=%d, ProductID=%s", req.TaskID, req.ProductID)

	// 懒加载：首次使用时启动结果监听器
	if err := c.ensureListenerStarted(); err != nil {
		return nil, fmt.Errorf("启动结果监听器失败: %w", err)
	}

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
	// 注意：爬虫任务使用专门的爬虫队列，根据优先级选择队列
	routingKey := c.taskAdapter.BuildRoutingKey(taskModel)
	queueName := c.queueNaming.BuildCrawlerQueueName(req.Platform, req.Priority)

	// 创建等待任务
	pendingTask := c.createPendingTask(ctx, req.TaskID)

	// 创建并发送 RabbitMQ 消息
	if err := c.publishTask(taskModel, messageData, queueName, routingKey, pendingTask); err != nil {
		return nil, err
	}

	// 等待结果
	return c.waitForResult(pendingTask, req.TaskID)
}

// ensureListenerStarted 确保结果监听器已启动（懒加载）
func (c *DistributedCrawlerClient) ensureListenerStarted() error {
	c.listenerMutex.Lock()
	defer c.listenerMutex.Unlock()

	// 如果已经启动，直接返回
	if c.listenerStarted {
		return nil
	}

	// 启动结果监听器
	c.logger.Info("首次使用，启动结果监听器...")
	if err := c.startResultListener(); err != nil {
		return err
	}

	c.listenerStarted = true
	return nil
}

// buildMessageData 构建消息数据
func (c *DistributedCrawlerClient) buildMessageData(_ any, req *CrawlRequest) map[string]any {
	// 直接构建符合 model.Task 结构的消息数据
	// 注意：字段名使用大写开头，与 model.Task 的 JSON 标签一致
	messageData := map[string]any{
		"id":            req.TaskID,
		"tenantId":      req.TenantID,
		"storeId":       req.StoreID,
		"platform":      req.Platform,
		"region":        req.Region,
		"productId":     req.ProductID,
		"priority":      req.Priority,
		"reply_to":      c.resultQueueName, // 添加回复队列
		"createTime":    time.Now().Unix(),
		"updateTime":    time.Now().Unix(),
		"retryCount":    0,
		"maxRetryCount": 3,
	}

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
	messageData map[string]any,
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
func (c *DistributedCrawlerClient) GetStats() map[string]any {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return map[string]any{
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

