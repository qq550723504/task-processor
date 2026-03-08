// Package task 提供任务消息适配器
package task

import (
	"encoding/json"
	"fmt"

	"task-processor/internal/domain/model"
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
	TaskID        int64  `json:"taskId"`
	TenantID      int64  `json:"tenantId"`
	StoreID       int64  `json:"storeId"`
	Platform      string `json:"platform"`
	Region        string `json:"region"`
	CategoryID    int64  `json:"categoryId"`
	ProductID     string `json:"productId"`
	Priority      int    `json:"priority"`
	RetryCount    int    `json:"retryCount"`
	MaxRetryCount int    `json:"maxRetryCount"`
	CreatedAt     int64  `json:"createdAt"`
	Remark        string `json:"remark,omitempty"`
	Status        string `json:"status,omitempty"`
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
			"amazon": "amazon.tasks.queue",
			"temu":   "temu.tasks.queue",
			"shein":  "shein.tasks.queue",
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
	status := a.convertStatusStringToInt16(taskMsg.Status)

	// 转换为任务对象
	task := &model.Task{
		ID:            taskMsg.TaskID,
		TenantID:      taskMsg.TenantID,
		StoreID:       taskMsg.StoreID,
		Platform:      taskMsg.Platform,
		Region:        taskMsg.Region,
		CategoryID:    taskMsg.CategoryID,
		ProductID:     taskMsg.ProductID,
		Status:        status,
		RetryCount:    taskMsg.RetryCount,
		MaxRetryCount: taskMsg.MaxRetryCount,
		Priority:      taskMsg.Priority,
		Remark:        taskMsg.Remark,
		CreateTime:    taskMsg.CreatedAt,
		UpdateTime:    taskMsg.CreatedAt,
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
		CreatedAt:     task.CreateTime,
		Remark:        task.Remark,
		Status:        a.convertStatusInt16ToString(task.Status),
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
func (a *MessageAdapter) BuildRoutingKey(task *model.Task) string {
	// 格式: {platform}.{priority}
	priorityLevel := a.getPriorityLevel(task.Priority)
	return fmt.Sprintf("%s.%s", task.Platform, priorityLevel)
}

// parseTaskMessage 解析任务消息
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

	// 设置重试信息
	taskMsg.RetryCount = msg.RetryCount
	taskMsg.MaxRetryCount = msg.MaxRetries

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

// convertStatusStringToInt16 将字符串状态转换为int16
func (a *MessageAdapter) convertStatusStringToInt16(statusStr string) int16 {
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
