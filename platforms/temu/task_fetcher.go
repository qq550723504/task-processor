package temu

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/common/config"
	"task-processor/common/management"
	"task-processor/common/processor"
	"task-processor/common/types"
	"time"

	"github.com/sirupsen/logrus"
)

// TemuTaskFetcher TEMU任务获取器
type TemuTaskFetcher struct {
	config           *config.Config
	workerPool       processor.WorkerPool
	interval         time.Duration
	managementClient *management.Client
}

// NewTemuTaskFetcher 创建TEMU任务获取器
func NewTemuTaskFetcher(cfg *config.Config, workerPool processor.WorkerPool, managementClient *management.Client) *TemuTaskFetcher {
	interval := time.Duration(cfg.Worker.TaskInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}

	return &TemuTaskFetcher{
		config:           cfg,
		workerPool:       workerPool,
		interval:         interval,
		managementClient: managementClient,
	}
}

// Start 启动任务获取
func (f *TemuTaskFetcher) Start(ctx context.Context) {
	logrus.Infof("[TEMU] 任务获取间隔: %v", f.interval)

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	statusTicker := time.NewTicker(30 * time.Second)
	defer statusTicker.Stop()

	lastTaskTime := time.Now()
	taskCount := 0

	for {
		select {
		case <-ctx.Done():
			logrus.Info("[TEMU] 任务处理循环已停止")
			return
		case <-statusTicker.C:
			f.logStatus(taskCount, lastTaskTime)
			taskCount = 0
		case <-ticker.C:
			fetched := f.fetchAndSubmitTasks(ctx)
			if fetched > 0 {
				lastTaskTime = time.Now()
				taskCount += fetched
			}
		}
	}
}

// fetchAndSubmitTasks 获取并提交任务
func (f *TemuTaskFetcher) fetchAndSubmitTasks(_ context.Context) int {
	availableSlots := f.workerPool.AvailableSlots()
	if availableSlots <= 0 {
		return 0
	}

	maxTasks := min(10, availableSlots)
	tasks, err := f.GetPendingTasks(maxTasks)
	if err != nil {
		logrus.Errorf("[TEMU] 获取待处理任务失败: %v", err)
		return 0
	}

	if len(tasks) > 0 {
		logrus.Infof("[TEMU] 📥 获取到 %d 个待处理任务", len(tasks))
	}

	for _, taskData := range tasks {
		if err := f.submitTask(taskData); err != nil {
			logrus.Errorf("[TEMU] 提交任务失败: %v", err)
			continue
		}
	}

	return len(tasks)
}

// GetPendingTasks 从API获取待处理任务
func (f *TemuTaskFetcher) GetPendingTasks(maxTasks int) ([]string, error) {
	// 检查是否有有效的用户令牌
	if f.managementClient == nil {
		logrus.Debug("[TEMU] 管理客户端为空，跳过任务获取")
		return []string{}, nil
	}

	if !f.managementClient.HasValidToken() {
		// 获取当前令牌状态用于调试
		accessToken, tenantID := f.managementClient.GetUserToken()
		logrus.Debugf("[TEMU] 令牌验证失败 - AccessToken: %s, TenantID: %s",
			maskToken(accessToken), tenantID)
		return []string{}, nil
	}

	// 使用真实的API调用获取任务，指定平台为"temu"
	apiTasks, err := f.managementClient.GetPendingTasks(maxTasks, f.config.Management.UserID, f.config.Management.StoreIDs, "Amazon")
	if err != nil {
		return nil, fmt.Errorf("API调用失败: %w", err)
	}

	// 转换API任务为内部任务格式
	if len(apiTasks) == 0 {
		return []string{}, nil
	}

	taskDataList := make([]string, 0, len(apiTasks))
	for _, apiTask := range apiTasks {
		internalTask := types.Task{
			ID:         fmt.Sprintf("%d", apiTask.ID),
			TenantID:   apiTask.TenantID,
			ProductID:  apiTask.ProductID,
			Platform:   apiTask.Platform,
			Region:     apiTask.Region,
			StoreID:    apiTask.StoreID,
			CategoryID: apiTask.CategoryID,
			CreateTime: apiTask.CreateTime,
			RetryCount: apiTask.RetryCount,
			Priority:   apiTask.Priority,
			Creator:    apiTask.Creator,
		}

		taskData, err := json.Marshal(internalTask)
		if err != nil {
			logrus.Errorf("[TEMU] 序列化任务失败: %v", err)
			continue
		}
		taskDataList = append(taskDataList, string(taskData))
	}

	logrus.Infof("[TEMU] 从API获取到 %d 个任务", len(taskDataList))
	return taskDataList, nil
}

// submitTask 提交单个任务
func (f *TemuTaskFetcher) submitTask(taskData string) error {
	var task types.Task
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	// 更新任务状态为处理中
	f.updateTaskStatusToProcessing(task.ID)

	if task.CreateTime > 0 {
		waitTime := time.Since(time.Unix(task.CreateTime/1000, 0))
		logrus.Infof("[TEMU] Task %s: Pending -> Processing (Priority: %d, WaitTime: %v)",
			task.ID, task.Priority, waitTime.Truncate(time.Millisecond))
	}

	if err := f.workerPool.Submit(processor.WorkerJob{
		TenantID: fmt.Sprintf("%d", task.TenantID),
		ShopID:   fmt.Sprintf("%d", task.StoreID),
		TaskData: taskData,
	}); err != nil {
		return fmt.Errorf("提交任务到工作池失败: %w", err)
	}

	logrus.Infof("[TEMU] ✅ 任务已发送到工作池: ID=%s, ProductID=%s, Priority=%d, Platform=%s",
		task.ID, task.ProductID, task.Priority, task.Platform)

	return nil
}

// updateTaskStatusToProcessing 更新任务状态为处理中
func (f *TemuTaskFetcher) updateTaskStatusToProcessing(taskID string) {
	// 异步更新状态，不阻塞任务提交
	go func() {
		if f.managementClient == nil || !f.managementClient.HasValidToken() {
			logrus.Warn("[TEMU] 管理客户端未初始化或令牌无效，跳过状态更新")
			return
		}

		// 解析任务ID
		var id int64
		if _, err := fmt.Sscanf(taskID, "%d", &id); err != nil {
			logrus.Errorf("[TEMU] 解析任务ID失败: %v", err)
			return
		}

		// 调用真实的API更新任务状态
		if err := f.managementClient.UpdateTaskStatus(id, 1); err != nil { // 1 = 处理中状态
			logrus.Errorf("[TEMU] 更新任务状态为处理中失败 (TaskID: %s): %v", taskID, err)
		} else {
			logrus.Infof("[TEMU] ✅ 任务状态已更新为处理中 (TaskID: %s)", taskID)
		}
	}()
}

// logStatus 输出运行状态
func (f *TemuTaskFetcher) logStatus(taskCount int, lastTaskTime time.Time) {
	if taskCount == 0 {
		logrus.Infof("[TEMU] 🔄 任务处理器运行中 - 当前无待处理任务 (上次处理任务: %v前)",
			time.Since(lastTaskTime).Truncate(time.Second))
	} else {
		logrus.Infof("[TEMU] 📊 任务处理器状态 - 最近30秒处理了 %d 个任务", taskCount)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// maskToken 遮蔽令牌用于日志输出
func maskToken(token string) string {
	if token == "" {
		return "<empty>"
	}
	if len(token) <= 10 {
		return "<short>"
	}
	return token[:10] + "..."
}
