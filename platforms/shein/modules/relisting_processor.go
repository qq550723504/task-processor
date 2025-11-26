package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"task-processor/common/memory"
	"time"

	"github.com/sirupsen/logrus"
)

// ReListingProcessor 重新上架任务处理器
type ReListingProcessor struct {
	queueManager *memory.ReListingQueueManager
}

// NewReListingProcessor 创建新的重新上架任务处理器
func NewReListingProcessor(queueManager *memory.ReListingQueueManager) *ReListingProcessor {
	return &ReListingProcessor{
		queueManager: queueManager,
	}
}

// ProcessReListingTasks 处理重新上架任务
func (p *ReListingProcessor) ProcessReListingTasks(ctx context.Context) error {
	logrus.Info("开始处理重新上架任务队列")

	// 获取所有店铺键
	keys := p.queueManager.GetAllKeys()

	for _, key := range keys {
		// 从键中提取租户ID和店铺ID (格式: tenantID:shopID)
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			logrus.Infof("解析键格式失败 %s: 格式应为 tenantID:shopID", key)
			continue
		}

		tenantID := parts[0]
		shopID := parts[1]

		// 处理该店铺的重新上架任务
		if err := p.processShopReListingTasks(ctx, tenantID, shopID); err != nil {
			logrus.Errorf("处理店铺 %s:%s 的重新上架任务失败: %v", tenantID, shopID, err)
		}
	}

	return nil
}

// processShopReListingTasks 处理特定店铺的重新上架任务
func (p *ReListingProcessor) processShopReListingTasks(ctx context.Context, tenantID, shopID string) error {
	// 转换为int64
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil {
		return fmt.Errorf("解析租户ID失败: %w", err)
	}
	shopIDInt, err := strconv.ParseInt(shopID, 10, 64)
	if err != nil {
		return fmt.Errorf("解析店铺ID失败: %w", err)
	}

	// 检查队列是否存在任务
	taskCount := p.queueManager.GetQueueLength(tenantIDInt, shopIDInt)
	if taskCount == 0 {
		// 没有重新上架任务
		return nil
	}

	logrus.Infof("店铺 %s:%s 有 %d 个重新上架任务待处理", tenantID, shopID, taskCount)

	// 处理所有重新上架任务
	for i := int64(0); i < taskCount; i++ {
		// 从队列中取出一个任务
		taskData, err := p.queueManager.PopTask(tenantIDInt, shopIDInt)
		if err != nil {
			logrus.Errorf("从重新上架队列获取任务失败: %v", err)
			continue
		}

		// 解析任务数据
		var reListingTask map[string]interface{}
		if err := json.Unmarshal([]byte(taskData), &reListingTask); err != nil {
			logrus.Errorf("解析重新上架任务数据失败: %v", err)
			continue
		}

		// 处理重新上架任务
		if err := p.handleReListingTask(ctx, tenantID, shopID, reListingTask); err != nil {
			logrus.Errorf("处理重新上架任务失败: %v", err)
			// 将任务重新放回队列
			p.queueManager.PushTask(tenantIDInt, shopIDInt, taskData)
		}
	}

	return nil
}

// handleReListingTask 处理单个重新上架任务
func (p *ReListingProcessor) handleReListingTask(ctx context.Context, tenantID, shopID string, task map[string]interface{}) error {
	taskID := "unknown"
	if id, ok := task["taskId"]; ok {
		taskID = fmt.Sprintf("%v", id)
	}

	logrus.Infof("处理重新上架任务: TaskID=%s, TenantID=%s, ShopID=%s", taskID, tenantID, shopID)

	// 检查任务是否满足重新上架条件
	// 例如：检查是否过了足够的等待时间
	if canReList, err := p.canReListTask(task); err != nil || !canReList {
		if err != nil {
			logrus.Warnf("检查重新上架条件失败: %v", err)
		} else {
			logrus.Infof("任务 %s 还未满足重新上架条件，将延后处理", taskID)
		}

		// 将任务重新放回队列尾部
		taskData, _ := json.Marshal(task)
		tenantIDInt, _ := strconv.ParseInt(tenantID, 10, 64)
		shopIDInt, _ := strconv.ParseInt(shopID, 10, 64)
		p.queueManager.PushTask(tenantIDInt, shopIDInt, string(taskData))
		return nil
	}

	// 满足条件，将任务移回待处理队列
	if err := p.moveToPendingQueue(ctx, tenantID, shopID, task); err != nil {
		return fmt.Errorf("将任务移回待处理队列失败: %w", err)
	}

	logrus.Infof("任务 %s 已移回待处理队列，准备重新上架", taskID)
	return nil
}

// canReListTask 检查任务是否满足重新上架条件
func (p *ReListingProcessor) canReListTask(task map[string]interface{}) (bool, error) {
	// 检查重新上架时间
	if relistTime, ok := task["relistTime"]; ok {
		var relistTimeInt int64
		switch v := relistTime.(type) {
		case float64:
			relistTimeInt = int64(v)
		case int64:
			relistTimeInt = v
		case string:
			var err error
			relistTimeInt, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return false, fmt.Errorf("解析重新上架时间失败: %w", err)
			}
		default:
			return false, fmt.Errorf("重新上架时间格式不正确")
		}

		// 检查是否过了24小时
		now := time.Now().Unix()
		if now-relistTimeInt < 24*60*60 { // 24小时
			return false, nil
		}
	}

	return true, nil
}

// moveToPendingQueue 将任务移回待处理队列
func (p *ReListingProcessor) moveToPendingQueue(ctx context.Context, tenantID, shopID string, task map[string]interface{}) error {
	// 移除重新上架相关的字段
	delete(task, "status")
	delete(task, "relistTime")
	delete(task, "relistReason")

	// 增加重试次数
	if retryCount, ok := task["retryCount"]; ok {
		switch v := retryCount.(type) {
		case float64:
			task["retryCount"] = int(v) + 1
		case int:
			task["retryCount"] = v + 1
		}
	} else {
		task["retryCount"] = 1
	}

	// 注意：不再需要添加到Redis待处理队列
	// 任务现在通过API获取，这里只是记录日志
	taskID := "unknown"
	if id, ok := task["taskId"]; ok {
		taskID = fmt.Sprintf("%v", id)
	}
	logrus.Infof("重新上架任务已准备好，将通过API重新获取: TaskID=%s", taskID)

	return nil
}
