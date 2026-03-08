// Package worker 提供工作池相关数据结构
package worker

// QueueStats 队列统计信息
type QueueStats struct {
	QueueSize      int     // 当前队列中的任务数
	BufferSize     int     // 队列总容量
	AvailableSlots int     // 可用槽位数
	UsagePercent   float64 // 使用率（%）
}

// Job 工作任务接口
// 这是一个通用接口，具体的业务任务应该实现这个接口
type Job interface {
	// GetID 获取任务ID，用于追踪和指标收集
	GetID() int64
}

// WorkerJob 工作任务（已废弃，保留用于向后兼容）
// 建议：在业务层定义自己的 Job 类型并实现 Job 接口
// Deprecated: 使用 Job 接口代替
type WorkerJob struct {
	TaskID   int64  // 任务ID，用于追踪和指标收集
	TenantID string // 租户ID（业务字段，不应在infra层）
	ShopID   string // 店铺ID（业务字段，不应在infra层）
	TaskData string // 任务数据（业务字段，不应在infra层）
}

// GetID 实现 Job 接口
func (j WorkerJob) GetID() int64 {
	return j.TaskID
}
