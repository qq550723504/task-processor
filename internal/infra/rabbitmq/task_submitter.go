// Package rabbitmq 提供任务提交功能
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"task-processor/internal/domain/model"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// submittedRecord 已提交记录
type submittedRecord struct {
	timestamp time.Time
}

// TaskSubmitter 任务提交器
type TaskSubmitter struct {
	client         *Client
	taskAdapter    *TaskMessageAdapter
	logger         *logrus.Logger
	submittedCache sync.Map // key: "tenant:region:asin", value: *submittedRecord
	cacheDuration  time.Duration
}

// NewTaskSubmitter 创建任务提交器
func NewTaskSubmitter(client *Client, logger *logrus.Logger) *TaskSubmitter {
	ts := &TaskSubmitter{
		client:        client,
		taskAdapter:   NewTaskMessageAdapter(),
		logger:        logger,
		cacheDuration: 5 * time.Minute, // 缓存5分钟
	}

	// 启动定期清理过期缓存的goroutine
	go ts.cleanExpiredCache()

	return ts
}

// cleanExpiredCache 定期清理过期缓存
func (ts *TaskSubmitter) cleanExpiredCache() {
	ticker := time.NewTicker(1 * time.Minute) // 每分钟清理一次
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		expiredCount := 0

		ts.submittedCache.Range(func(key, value interface{}) bool {
			record := value.(*submittedRecord)
			if now.Sub(record.timestamp) > ts.cacheDuration {
				ts.submittedCache.Delete(key)
				expiredCount++
			}
			return true
		})

		if expiredCount > 0 {
			ts.logger.Debugf("🧹 清理过期缓存: 删除=%d", expiredCount)
		}
	}
}

// getCacheKey 生成缓存键
func (ts *TaskSubmitter) getCacheKey(tenantID int64, region, asin string) string {
	return fmt.Sprintf("%d:%s:%s", tenantID, region, asin)
}

// isRecentlySubmitted 检查是否最近已提交
func (ts *TaskSubmitter) isRecentlySubmitted(tenantID int64, region, asin string) bool {
	key := ts.getCacheKey(tenantID, region, asin)
	if value, ok := ts.submittedCache.Load(key); ok {
		record := value.(*submittedRecord)
		if time.Since(record.timestamp) <= ts.cacheDuration {
			return true
		}
	}
	return false
}

// markAsSubmitted 标记为已提交
func (ts *TaskSubmitter) markAsSubmitted(tenantID int64, region, asin string) {
	key := ts.getCacheKey(tenantID, region, asin)
	ts.submittedCache.Store(key, &submittedRecord{
		timestamp: time.Now(),
	})
}

// SubmitTask 提交单个任务
func (ts *TaskSubmitter) SubmitTask(ctx context.Context, task *model.Task) error {
	// 创建TaskMessage
	taskMsg := TaskMessage{
		TaskID:        task.ID,
		TenantID:      task.TenantID,
		StoreID:       task.StoreID,
		Platform:      task.Platform,
		Region:        task.Region,
		CategoryID:    task.CategoryID,
		ProductID:     task.ProductID,
		Priority:      task.Priority,
		RetryCount:    task.RetryCount,
		MaxRetryCount: task.MaxRetryCount,
		CreatedAt:     task.CreateTime,
		Remark:        task.Remark, // 传递备注字段
	}

	// 序列化TaskMessage为JSON（只包含TaskMessage，不包含Message外层）
	body, err := json.Marshal(taskMsg)
	if err != nil {
		return fmt.Errorf("序列化任务消息失败: %w", err)
	}

	// 获取队列名称和优先级
	queueName := ts.taskAdapter.GetQueueName(task.Platform)
	priority := ts.taskAdapter.calculateRabbitMQPriority(task.Priority)

	// 获取通道
	channel, err := ts.client.connManager.GetChannel()
	if err != nil {
		return fmt.Errorf("获取通道失败: %w", err)
	}

	// 构建发布消息（与Python脚本格式一致）
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body, // 只包含TaskMessage
		Priority:     priority,
		Timestamp:    time.Now(),
		MessageId:    strconv.FormatInt(task.ID, 10), // 通过properties传递
		Type:         "task",                         // 通过properties传递
		DeliveryMode: 2,                              // 持久化
	}

	// 直接发送到队列（使用默认交换机）
	err = channel.PublishWithContext(
		ctx,
		"",        // 使用默认交换机
		queueName, // 路由键就是队列名
		false,     // mandatory
		false,     // immediate
		publishing,
	)

	if err != nil {
		return fmt.Errorf("发布任务消息失败: %w", err)
	}

	ts.logger.Debugf("✅ 任务消息已提交: ID=%d, Queue=%s", task.ID, queueName)
	return nil
}

// SubmitVariantTasks 批量提交变体任务（带去重）
func (ts *TaskSubmitter) SubmitVariantTasks(ctx context.Context, parentTask *model.Task, variations []model.Variation, parentAsin string) (int, int) {
	successCount := 0
	failCount := 0
	skipCount := 0

	ts.logger.Infof("📋 [变体提交] 开始提交变体任务 - 父任务ID=%d, 父ProductID=%s, 父ASIN=%s, 变体总数=%d",
		parentTask.ID, parentTask.ProductID, parentAsin, len(variations))

	for i, v := range variations {
		// 过滤规则1: 跳过父ASIN本身
		if v.Asin == parentAsin && parentAsin != "" {
			ts.logger.Debugf("   [%d/%d] ⏭️ 跳过父ASIN: %s (原因: 这是父ASIN本身)", i+1, len(variations), v.Asin)
			skipCount++
			continue
		}

		// 过滤规则2: 跳过当前任务的ProductID（避免自己提交自己）
		if v.Asin == parentTask.ProductID {
			ts.logger.Debugf("   [%d/%d] ⏭️ 跳过当前产品: %s (原因: 与当前任务ProductID相同)", i+1, len(variations), v.Asin)
			skipCount++
			continue
		}

		// 过滤规则3: 检查是否最近已提交过（5分钟内）
		if ts.isRecentlySubmitted(parentTask.TenantID, parentTask.Region, v.Asin) {
			ts.logger.Debugf("   [%d/%d] ⏭️ 跳过重复提交: %s (原因: 5分钟内已提交过)", i+1, len(variations), v.Asin)
			skipCount++
			continue
		}

		// 为每个变体创建任务
		variantTask := &model.Task{
			ID:            time.Now().UnixNano(), // 生成唯一ID
			TenantID:      parentTask.TenantID,
			StoreID:       parentTask.StoreID,
			Platform:      parentTask.Platform,
			Region:        parentTask.Region,
			ProductID:     v.Asin,
			CategoryID:    parentTask.CategoryID,
			Priority:      parentTask.Priority + 1, // 变体优先级稍高
			Remark:        "variant",               // 标记为变体任务，避免递归提交
			CreateTime:    time.Now().Unix(),
			UpdateTime:    time.Now().Unix(),
			MaxRetryCount: 3,
		}

		ts.logger.Infof("   [%d/%d] 📤 提交变体任务: ASIN=%s, Name=%s, TaskID=%d",
			i+1, len(variations), v.Asin, v.Name, variantTask.ID)

		// 提交任务
		if err := ts.SubmitTask(ctx, variantTask); err != nil {
			ts.logger.Errorf("❌ 提交变体任务失败: ASIN=%s, Error=%v", v.Asin, err)
			failCount++
			continue
		}

		// 标记为已提交
		ts.markAsSubmitted(parentTask.TenantID, parentTask.Region, v.Asin)

		ts.logger.Debugf("✅ 变体任务已提交: ASIN=%s", v.Asin)
		successCount++
	}

	ts.logger.Infof("📊 [变体提交] 提交完成 - 成功=%d, 失败=%d, 跳过=%d, 总数=%d",
		successCount, failCount, skipCount, len(variations))

	return successCount, failCount
}
