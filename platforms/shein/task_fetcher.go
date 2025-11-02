package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/common/auth"
	"task-processor/common/config"
	"task-processor/common/processor"
	"task-processor/common/types"
	"time"

	"github.com/sirupsen/logrus"
)

// SheinTaskFetcher SHEIN任务获取器
type SheinTaskFetcher struct {
	config     *config.Config
	workerPool processor.WorkerPool
	interval   time.Duration
	authClient *auth.ClientCredentialsAuth
}

// NewSheinTaskFetcher 创建SHEIN任务获取器
func NewSheinTaskFetcher(cfg *config.Config, workerPool processor.WorkerPool) *SheinTaskFetcher {
	interval := time.Duration(cfg.Worker.TaskInterval) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second // SHEIN默认60秒间隔
	}

	// 创建客户端凭证认证
	authClient := auth.NewClientCredentialsAuth(
		cfg.Management.TokenURL,
		cfg.Management.ClientID,
		cfg.Management.ClientSecret,
		cfg.Management.Scopes,
	)

	return &SheinTaskFetcher{
		config:     cfg,
		workerPool: workerPool,
		interval:   interval,
		authClient: authClient,
	}
}

// Start 启动任务获取
func (f *SheinTaskFetcher) Start(ctx context.Context) {
	logrus.Infof("[SHEIN] 任务获取间隔: %v", f.interval)

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	statusTicker := time.NewTicker(30 * time.Second)
	defer statusTicker.Stop()

	lastTaskTime := time.Now()
	taskCount := 0

	for {
		select {
		case <-ctx.Done():
			logrus.Info("[SHEIN] 任务处理循环已停止")
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
func (f *SheinTaskFetcher) fetchAndSubmitTasks(ctx context.Context) int {
	availableSlots := f.workerPool.AvailableSlots()
	if availableSlots <= 0 {
		return 0
	}

	maxTasks := min(10, availableSlots)
	tasks, err := f.GetPendingTasks(maxTasks)
	if err != nil {
		logrus.Errorf("[SHEIN] 获取待处理任务失败: %v", err)
		return 0
	}

	if len(tasks) > 0 {
		logrus.Infof("[SHEIN] 获取到 %d 个待处理任务", len(tasks))
	}

	for _, taskData := range tasks {
		if err := f.submitTask(taskData); err != nil {
			logrus.Errorf("[SHEIN] 提交任务失败: %v", err)
			continue
		}
	}

	return len(tasks)
}

// GetPendingTasks 从API获取待处理任务
func (f *SheinTaskFetcher) GetPendingTasks(maxTasks int) ([]string, error) {
	// 获取访问令牌
	token, err := f.authClient.GetToken(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %w", err)
	}

	// 模拟从API获取SHEIN任务
	tasks := f.mockGetSheinTasks(maxTasks, token)

	if len(tasks) == 0 {
		return []string{}, nil
	}

	taskDataList := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskData, err := json.Marshal(task)
		if err != nil {
			logrus.Errorf("[SHEIN] 序列化任务失败: %v", err)
			continue
		}
		taskDataList = append(taskDataList, string(taskData))
	}

	logrus.Infof("[SHEIN] 从API获取到 %d 个任务", len(taskDataList))
	return taskDataList, nil
}

// mockGetSheinTasks 模拟获取SHEIN任务（实际应该调用真实API）
func (f *SheinTaskFetcher) mockGetSheinTasks(maxTasks int, token string) []types.Task {
	// 模拟任务数据
	mockTasks := []types.Task{
		{
			ID:         "shein_001",
			TenantID:   1001,
			ProductID:  "SHEIN_P001",
			Platform:   "shein",
			Region:     "US",
			StoreID:    2001,
			CategoryID: 1001,
			CreateTime: time.Now().Unix() * 1000,
			RetryCount: 0,
			Priority:   1,
			Creator:    "system",
		},
		{
			ID:         "shein_002",
			TenantID:   1001,
			ProductID:  "SHEIN_P002",
			Platform:   "amazon", // 这个任务需要Amazon数据
			Region:     "US",
			StoreID:    2001,
			CategoryID: 1002,
			CreateTime: time.Now().Unix() * 1000,
			RetryCount: 0,
			Priority:   2,
			Creator:    "system",
		},
		{
			ID:         "shein_003",
			TenantID:   1001,
			ProductID:  "B08N5WRWNW", // Amazon ASIN
			Platform:   "amazon",
			Region:     "US",
			StoreID:    2002,
			CategoryID: 1003,
			CreateTime: time.Now().Unix() * 1000,
			RetryCount: 0,
			Priority:   3,
			Creator:    "system",
		},
	}

	// 根据maxTasks限制返回数量
	if maxTasks < len(mockTasks) {
		return mockTasks[:maxTasks]
	}

	return mockTasks
}

// submitTask 提交单个任务
func (f *SheinTaskFetcher) submitTask(taskData string) error {
	var task types.Task
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	// 更新任务状态为处理中
	f.updateTaskStatusToProcessing(task.ID)

	if task.CreateTime > 0 {
		waitTime := time.Since(time.Unix(task.CreateTime/1000, 0))
		logrus.Infof("[SHEIN] Task %s: Pending -> Processing (Priority: %d, WaitTime: %v, Platform: %s)",
			task.ID, task.Priority, waitTime.Truncate(time.Millisecond), task.Platform)
	}

	if err := f.workerPool.Submit(processor.WorkerJob{
		TenantID: fmt.Sprintf("%d", task.TenantID),
		ShopID:   fmt.Sprintf("%d", task.StoreID),
		TaskData: taskData,
	}); err != nil {
		return fmt.Errorf("提交任务到工作池失败: %w", err)
	}

	logrus.Infof("[SHEIN] 任务已发送到工作池: ID=%s, ProductID=%s, Priority=%d, Platform=%s",
		task.ID, task.ProductID, task.Priority, task.Platform)

	return nil
}

// updateTaskStatusToProcessing 更新任务状态为处理中
func (f *SheinTaskFetcher) updateTaskStatusToProcessing(taskID string) {
	// 异步更新状态，不阻塞任务提交
	go func() {
		// 这里应该调用真实的API更新任务状态
		logrus.Infof("[SHEIN] 任务状态已更新为处理中 (TaskID: %s)", taskID)
	}()
}

// logStatus 输出运行状态
func (f *SheinTaskFetcher) logStatus(taskCount int, lastTaskTime time.Time) {
	if taskCount == 0 {
		logrus.Infof("[SHEIN] 任务处理器运行中 - 当前无待处理任务 (上次处理任务: %v前)",
			time.Since(lastTaskTime).Truncate(time.Second))
	} else {
		logrus.Infof("[SHEIN] 任务处理器状态 - 最近30秒处理了 %d 个任务", taskCount)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
