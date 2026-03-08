// Package task 提供任务相关的领域模型
package task

// TaskJob 任务工作单元
// 这是业务层的 Job 定义，包含任务处理所需的业务字段
type TaskJob struct {
	TaskID   int64  // 任务ID
	TenantID string // 租户ID
	ShopID   string // 店铺ID
	TaskData string // 任务数据（JSON格式）
}

// GetID 实现 worker.Job 接口
func (j TaskJob) GetID() int64 {
	return j.TaskID
}

// NewTaskJob 创建任务工作单元
func NewTaskJob(taskID int64, tenantID, shopID, taskData string) TaskJob {
	return TaskJob{
		TaskID:   taskID,
		TenantID: tenantID,
		ShopID:   shopID,
		TaskData: taskData,
	}
}
