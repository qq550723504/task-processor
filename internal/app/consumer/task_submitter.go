// Package consumer 提供消息处理应用层服务
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"task-processor/internal/app/task"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// submittedRecord 已提交记录
type submittedRecord struct {
	timestamp time.Time
}

// TaskSubmitter 任务提交服务（应用层）
// 负责协调任务提交的完整流程
type TaskSubmitter struct {
	client         *rabbitmq.Client
	adapter        *task.MessageAdapter
	queueNaming    *rabbitmq.NamingService
	logger         *logrus.Logger
	submittedCache sync.Map // key: "tenant:region:asin", value: *submittedRecord
	cacheDuration  time.Duration
	publishChannel *amqp.Channel // 独立的发布通道
	channelMutex   sync.Mutex    // 通道锁
}

// NewTaskSubmitter 创建任务提交服务
func NewTaskSubmitter(client *rabbitmq.Client, logger *logrus.Logger) *TaskSubmitter {
	ts := &TaskSubmitter{
		client:        client,
		adapter:       task.NewMessageAdapter(),
		queueNaming:   rabbitmq.NewNamingService(),
		logger:        logger,
		cacheDuration: 5 * time.Minute, // 缓存5分钟
	}

	// 注意：不在这里创建独立通道，因为连接可能还未建立
	// 独立通道将在第一次使用时延迟创建

	// 启动定期清理过期缓存的goroutine
	go ts.cleanExpiredCache()

	return ts
}

// cleanExpiredCache 定期清理过期缓存
func (ts *TaskSubmitter) cleanExpiredCache() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		expiredCount := 0

		ts.submittedCache.Range(func(key, value any) bool {
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
func (ts *TaskSubmitter) SubmitTask(ctx context.Context, t *model.Task) error {
	// 1. 使用领域层适配器转换任务
	taskMsg, err := ts.adapter.TaskToMessage(t)
	if err != nil {
		return fmt.Errorf("转换任务失败: %w", err)
	}

	// 2. 序列化为JSON
	body, err := json.Marshal(taskMsg)
	if err != nil {
		return fmt.Errorf("序列化任务消息失败: %w", err)
	}

	// 3. 获取队列名称和优先级（使用领域层业务规则）
	// 特殊处理：如果是爬虫任务（Platform包含.crawler），使用基于优先级的队列名称
	var queueName string
	if strings.Contains(t.Platform, ".crawler") {
		queueName = ts.queueNaming.BuildCrawlerQueueName(t.Platform, t.Priority)
	} else {
		queueName = ts.queueNaming.BuildTaskQueueName(t.Platform, t.Priority)
	}
	priority := ts.adapter.CalculatePriority(t.Priority)

	// 4. 获取RabbitMQ通道（使用独立通道避免死锁）
	ts.channelMutex.Lock()
	defer ts.channelMutex.Unlock()

	var channel *amqp.Channel
	if ts.publishChannel != nil && !ts.publishChannel.IsClosed() {
		// 使用已有的独立通道
		channel = ts.publishChannel
	} else {
		// 延迟创建独立通道（第一次使用或通道已关闭时）
		var channelErr error
		channel, channelErr = ts.client.GetConnectionManager().CreateChannel()
		if channelErr != nil {
			return fmt.Errorf("获取通道失败: %w", channelErr)
		}
		ts.publishChannel = channel
		ts.logger.Info("✅ 创建独立发布通道（避免与消费者共享通道）")
	}

	// 5. 构建发布消息
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		Priority:     priority,
		Timestamp:    time.Now(),
		MessageId:    strconv.FormatInt(t.ID, 10),
		Type:         "task",
		DeliveryMode: 2, // 持久化
	}

	// 6. 发布到队列
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

	ts.logger.Debugf("✅ 任务消息已提交: ID=%d, Queue=%s", t.ID, queueName)
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

		// 过滤规则2: 跳过当前任务的ProductID
		if v.Asin == parentTask.ProductID {
			ts.logger.Debugf("   [%d/%d] ⏭️ 跳过当前产品: %s (原因: 与当前任务ProductID相同)", i+1, len(variations), v.Asin)
			skipCount++
			continue
		}

		// 过滤规则3: 检查是否最近已提交过
		if ts.isRecentlySubmitted(parentTask.TenantID, parentTask.Region, v.Asin) {
			ts.logger.Debugf("   [%d/%d] ⏭️ 跳过重复提交: %s (原因: 5分钟内已提交过)", i+1, len(variations), v.Asin)
			skipCount++
			continue
		}

		// 为每个变体创建任务
		// 确保平台名称正确：如果父任务已经包含.crawler后缀，则去掉后再使用基础平台名
		variantPlatform := parentTask.Platform
		if strings.Contains(variantPlatform, ".crawler") {
			// 去掉.crawler后缀，只保留基础平台名（如 Amazon）
			variantPlatform = strings.TrimSuffix(variantPlatform, ".crawler")
		}

		variantTask := &model.Task{
			ID:            time.Now().UnixNano(),
			TenantID:      parentTask.TenantID,
			StoreID:       parentTask.StoreID,
			Platform:      variantPlatform, // 使用基础平台名（如 Amazon）
			Region:        parentTask.Region,
			ProductID:     v.Asin,
			CategoryID:    parentTask.CategoryID,
			Priority:      parentTask.Priority + 1,
			Remark:        "variant",
			CreateTime:    time.Now().Unix(),
			UpdateTime:    time.Now().Unix(),
			MaxRetryCount: 3,
		}

		ts.logger.Infof("   [%d/%d] 📤 提交变体任务: ASIN=%s, TaskID=%d",
			i+1, len(variations), v.Asin, variantTask.ID)

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
