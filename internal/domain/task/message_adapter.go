// Package task 提供任务消息适配器
package task

import (
	"encoding/json"
	"fmt"
	"time"

	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/types"
)

// Message 消息结构（从基础设施层解耦）
type Message struct {
	ID         string
	Type       string
	Payload    map[string]interface{}
	Priority   uint8
	Timestamp  int64
	RetryCount int
	MaxRetries int
}

// TaskMessage 任务消息结构
type TaskMessage struct {
	TaskID         int64               `json:"taskId"`
	TenantID       int64               `json:"tenantId"`
	StoreID        int64               `json:"storeId"`
	Platform       string              `json:"platform"`       // 爬虫平台（数据来源，如"amazon"）
	TargetPlatform string              `json:"targetPlatform"` // 目标上架平台（如"temu"、"shein"、"amazon"）
	Region         string              `json:"region"`
	CategoryID     int64               `json:"categoryId"`
	ProductID      string              `json:"productId"`
	Priority       int                 `json:"priority"`
	RetryCount     int                 `json:"retryCount"`
	MaxRetryCount  int                 `json:"maxRetryCount"`
	CreatedAt      *types.FlexibleTime `json:"createdAt"` // 支持多种时间格式
	Remark         string              `json:"remark,omitempty"`
	Status         string              `json:"status,omitempty"`
}

// MessageAdapter 任务消息适配器（领域逻辑）
// 负责任务对象与消息格式之间的转换
type MessageAdapter struct {
	queueMapping map[string]string // platform -> queue
}

// NewMessageAdapter 创建任务消息适配器
func NewMessageAdapter() *MessageAdapter {
	return &MessageAdapter{
		queueMapping: map[string]string{
			// 上架任务队列
			"amazon": "amazon.tasks.queue",
			"temu":   "temu.tasks.queue",
			"shein":  "shein.tasks.queue",

			// 爬虫任务队列
			"amazon.crawler": "amazon.crawler.queue",
			"1688.crawler":   "1688.crawler.queue",
		},
	}
}

// MessageToTask 将消息转换为任务对象
func (a *MessageAdapter) MessageToTask(msg *Message) (*model.Task, error) {
	if msg == nil {
		return nil, fmt.Errorf("消息不能为空")
	}

	// 解析消息载荷
	taskMsg, err := a.parseTaskMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("解析任务消息失败: %w", err)
	}

	// 转换状态：从字符串转为int16
	status := a.ConvertStatusStringToInt16(taskMsg.Status)

	// 处理创建时间
	var createTime int64
	if taskMsg.CreatedAt != nil && !taskMsg.CreatedAt.IsZero() {
		createTime = taskMsg.CreatedAt.Unix()
	}

	// 确定目标平台：优先使用TargetPlatform，如果为空则使用Platform（向后兼容）
	targetPlatform := taskMsg.TargetPlatform
	if targetPlatform == "" {
		targetPlatform = taskMsg.Platform
	}

	// 数据来源平台
	sourcePlatform := taskMsg.Platform
	if sourcePlatform == "" {
		sourcePlatform = targetPlatform // 如果没有指定来源，默认为目标平台
	}

	// 转换为任务对象
	task := &model.Task{
		ID:             taskMsg.TaskID,
		TenantID:       taskMsg.TenantID,
		StoreID:        taskMsg.StoreID,
		Platform:       targetPlatform, // 目标上架平台
		SourcePlatform: sourcePlatform, // 数据来源平台
		Region:         taskMsg.Region,
		CategoryID:     taskMsg.CategoryID,
		ProductID:      taskMsg.ProductID,
		Status:         status,
		RetryCount:     taskMsg.RetryCount,
		MaxRetryCount:  taskMsg.MaxRetryCount,
		Priority:       taskMsg.Priority,
		Remark:         taskMsg.Remark,
		CreateTime:     createTime,
		UpdateTime:     createTime,
	}

	return task, nil
}

// TaskToMessage 将任务对象转换为消息
func (a *MessageAdapter) TaskToMessage(task *model.Task) (*TaskMessage, error) {
	if task == nil {
		return nil, fmt.Errorf("任务不能为空")
	}

	// 创建任务消息
	taskMsg := &TaskMessage{
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
		CreatedAt:     types.ToFlexibleTime(&time.Time{}),
		Remark:        task.Remark,
		Status:        a.convertStatusInt16ToString(task.Status),
	}

	// 设置创建时间
	if task.CreateTime > 0 {
		t := time.Unix(task.CreateTime, 0)
		taskMsg.CreatedAt = types.ToFlexibleTime(&t)
	}

	return taskMsg, nil
}

// GetQueueName 根据平台获取队列名称（业务规则）
func (a *MessageAdapter) GetQueueName(platform string) string {
	if queue, ok := a.queueMapping[platform]; ok {
		return queue
	}
	return "amazon.tasks.queue" // 默认队列
}

// CalculatePriority 计算消息优先级
// 业务优先级(1-10) -> 消息优先级(0-10)，数字越大优先级越高
func (a *MessageAdapter) CalculatePriority(businessPriority int) uint8 {
	// 业务优先级范围检查
	if businessPriority < 1 {
		businessPriority = 1
	}
	if businessPriority > 10 {
		businessPriority = 10
	}

	// 转换为消息优先级（1-10 -> 10-1，然后映射到0-10）
	// 业务优先级1（最高）-> 消息优先级10
	// 业务优先级10（最低）-> 消息优先级1
	messagePriority := 11 - businessPriority

	return uint8(messagePriority)
}

// BuildRoutingKey 构建路由键（业务规则）
// 格式: {targetPlatform}.{sourcePlatform}.{priority}.{region}
// 示例: shein.amazon.normal.us (Amazon数据 → SHEIN上架)
func (a *MessageAdapter) BuildRoutingKey(task *model.Task) string {
	priorityLevel := a.getPriorityLevel(task.Priority)

	// 使用任务对象中的 SourcePlatform 字段
	// 如果为空，默认使用目标平台（表示同平台数据）
	sourcePlatform := task.SourcePlatform
	if sourcePlatform == "" {
		sourcePlatform = task.Platform
	}

	return fmt.Sprintf("%s.%s.%s.%s",
		task.Platform,  // 目标平台
		sourcePlatform, // 来源平台
		priorityLevel,  // 优先级
		task.Region)    // 区域
}

// parseTaskMessage 解析任务消息
// 自动检测并支持两种消息格式:
// 1. 嵌套格式(Go): {id, type, payload: {...}, priority, timestamp}
// 2. 扁平格式(Java): {taskId, tenantId, storeId, ...}
func (a *MessageAdapter) parseTaskMessage(msg *Message) (*TaskMessage, error) {
	if len(msg.Payload) == 0 {
		return nil, fmt.Errorf("消息载荷为空")
	}

	// 序列化载荷为JSON字符串
	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return nil, fmt.Errorf("序列化载荷失败: %w", err)
	}

	// 反序列化为任务消息
	var taskMsg TaskMessage
	err = json.Unmarshal(payloadBytes, &taskMsg)
	if err != nil {
		return nil, fmt.Errorf("反序列化任务消息失败: %w", err)
	}

	// 设置重试信息（如果消息中没有，使用顶层的重试信息）
	if taskMsg.RetryCount == 0 && msg.RetryCount > 0 {
		taskMsg.RetryCount = msg.RetryCount
	}
	if taskMsg.MaxRetryCount == 0 && msg.MaxRetries > 0 {
		taskMsg.MaxRetryCount = msg.MaxRetries
	}
	// 如果都没有，设置默认值
	if taskMsg.MaxRetryCount == 0 {
		taskMsg.MaxRetryCount = 3
	}

	return &taskMsg, nil
}

// getPriorityLevel 获取优先级级别
func (a *MessageAdapter) getPriorityLevel(priority int) string {
	switch {
	case priority >= 1 && priority <= 3:
		return "urgent"
	case priority >= 4 && priority <= 6:
		return "high"
	case priority >= 7 && priority <= 8:
		return "normal"
	default:
		return "low"
	}
}

// ConvertStatusStringToInt16 将字符串状态转换为int16（公共方法）
func (a *MessageAdapter) ConvertStatusStringToInt16(statusStr string) int16 {
	if statusStr == "" {
		return 0 // TaskStatusPending
	}

	switch statusStr {
	case "pending":
		return 0
	case "processing":
		return 1
	case "completed":
		return 6
	case "failed":
		return 3
	case "pending_retry":
		return 4
	case "draft":
		return 8
	case "paused":
		return 10
	case "terminated":
		return 13
	case "crawled":
		return 2
	case "queued":
		return 5
	default:
		return 0
	}
}

// convertStatusInt16ToString 将int16状态转换为字符串
func (a *MessageAdapter) convertStatusInt16ToString(status int16) string {
	switch status {
	case 0:
		return "pending"
	case 1:
		return "processing"
	case 2:
		return "crawled"
	case 3:
		return "failed"
	case 4:
		return "pending_retry"
	case 5:
		return "queued"
	case 6:
		return "completed"
	case 7:
		return "republishing"
	case 8:
		return "draft"
	case 9:
		return "cancelled"
	case 10:
		return "paused"
	case 11:
		return "resumed"
	case 12:
		return "resuming"
	case 13:
		return "terminated"
	default:
		return "unknown"
	}
}
