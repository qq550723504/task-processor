// Package distributed 提供分布式爬虫客户端
package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/app/task"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

// DistributedCrawlerClient 分布式爬虫客户端
type DistributedCrawlerClient struct {
	publisher   Publisher
	listener    *ResultListener
	registry    *PendingRegistry
	taskAdapter priorityCalculator
	queueNaming queueNamer
	logger      *logrus.Logger

	// 懒加载保护：用 Mutex 而非 sync.Once，支持失败后重试
	startMu sync.Mutex
}

// NewDistributedCrawlerClient 创建分布式爬虫客户端
func NewDistributedCrawlerClient(rabbitmqClient *rabbitmq.Client, logger *logrus.Logger) (*DistributedCrawlerClient, error) {
	if rabbitmqClient == nil {
		return nil, fmt.Errorf("RabbitMQ客户端不能为空")
	}

	adapter := NewRabbitMQAdapter(rabbitmqClient)
	registry := NewPendingRegistry(5 * time.Minute)
	listener := NewResultListener(adapter, registry, logger)

	c := &DistributedCrawlerClient{
		publisher:   adapter,
		listener:    listener,
		registry:    registry,
		taskAdapter: task.NewMessageAdapter(),
		queueNaming: rabbitmq.NewNamingService(),
		logger:      logger,
	}

	// 注册重连回调：连接恢复后重新启动结果监听器
	rabbitmqClient.GetConnectionManager().RegisterReconnectCallback(func() error {
		logger.Info("RabbitMQ重连成功，重新启动爬虫结果监听器...")
		return listener.Restart()
	})

	logger.Info("分布式爬虫客户端创建成功（结果监听器将在首次使用时启动）")
	return c, nil
}

// SetTimeout 设置等待超时
func (c *DistributedCrawlerClient) SetTimeout(timeout time.Duration) {
	c.registry.timeout = timeout
}

// SubmitCrawlTask 提交爬虫任务并同步等待结果
func (c *DistributedCrawlerClient) SubmitCrawlTask(ctx context.Context, req *CrawlRequest) (*CrawlResult, error) {
	c.logger.Infof("提交爬虫任务: TaskID=%s, ProductID=%s", req.TaskID, req.ProductID)

	// 步骤1：确保结果监听器已启动（懒加载，只执行一次）
	replyTo, err := c.ensureListenerStarted()
	if err != nil {
		return nil, fmt.Errorf("启动结果监听器失败: %w", err)
	}

	// 步骤2：注册等待任务
	pt := c.registry.Register(ctx, req.TaskID)

	// 步骤3：构建并发布爬虫任务消息
	queueName := c.queueNaming.BuildCrawlerQueueName(req.Platform, req.Priority)
	if err := c.publishCrawlTask(ctx, req, queueName, replyTo, pt.TaskID); err != nil {
		c.registry.Remove(pt.TaskID)
		return nil, err
	}

	c.logger.Infof("爬虫任务已发送: TaskID=%s, Queue=%s, ReplyTo=%s", pt.TaskID, queueName, replyTo)

	// 步骤4：等待结果
	return c.registry.Wait(pt)
}

// ensureListenerStarted 确保结果监听器已启动，返回结果队列名。
// 支持失败后重试（不使用 sync.Once，避免失败后无法重试的问题）。
func (c *DistributedCrawlerClient) ensureListenerStarted() (string, error) {
	// 快路径：已启动则直接返回
	if name := c.listener.QueueName(); name != "" {
		return name, nil
	}

	c.startMu.Lock()
	defer c.startMu.Unlock()

	// 双重检查
	if name := c.listener.QueueName(); name != "" {
		return name, nil
	}

	c.logger.Info("首次使用，启动结果监听器...")
	name, err := c.listener.Start()
	if err != nil {
		return "", err
	}
	return name, nil
}

// publishCrawlTask 构建消息体并发布到爬虫队列
func (c *DistributedCrawlerClient) publishCrawlTask(
	ctx context.Context,
	req *CrawlRequest,
	queueName, replyTo, taskIDStr string,
) error {
	now := time.Now().Unix()
	priority := c.taskAdapter.CalculatePriority(req.Priority)

	payload := map[string]any{
		"id":             req.TaskID, // string，避免 JSON float64 精度丢失
		"tenantId":       req.TenantID,
		"storeId":        req.StoreID,
		"sourcePlatform": req.Platform,
		"region":         req.Region,
		"productId":      req.ProductID,
		"priority":       req.Priority,
		"reply_to":       replyTo,
		"createTime":     now,
		"updateTime":     now,
		"retryCount":     0,
		"maxRetryCount":  3,
	}

	body, err := json.Marshal(&rabbitmq.Message{
		ID:         taskIDStr,
		Type:       "task",
		Payload:    payload,
		Priority:   priority,
		Timestamp:  now,
		RetryCount: 0,
		MaxRetries: 3,
	})
	if err != nil {
		return fmt.Errorf("序列化爬虫任务消息失败: %w", err)
	}

	if err := c.publisher.Publish(ctx, queueName, body, priority); err != nil {
		return fmt.Errorf("发布爬虫任务失败: %w", err)
	}
	return nil
}

// GetStats 获取统计信息
func (c *DistributedCrawlerClient) GetStats() map[string]any {
	return map[string]any{
		"pending_tasks":  c.registry.Len(),
		"timeout":        c.registry.timeout.String(),
		"listener_queue": c.listener.QueueName(),
	}
}

// Close 关闭客户端，取消所有等待任务
func (c *DistributedCrawlerClient) Close() error {
	c.logger.Info("关闭分布式爬虫客户端")
	// registry 里的所有 pending task 会在 context 超时时自动清理
	// 这里主动触发清理
	c.registry.mu.Lock()
	for _, pt := range c.registry.tasks {
		pt.Cancel()
	}
	c.registry.tasks = make(map[string]*PendingTask)
	c.registry.mu.Unlock()
	return nil
}
