// Package task 提供任务消息适配器
package task

import (
	"encoding/json"
	"fmt"
	"time"

	"task-processor/internal/model"
	"task-processor/internal/pkg/types"
)

// Message 消息结构（从基础设施层解耦）
type Message struct {
	ID         string
	Type       string
	Payload    map[string]any
	Priority   uint8
	Timestamp  int64
	RetryCount int
	MaxRetries int
}

// TaskMessage 任务消息结构
type TaskMessage struct {
	TaskID         flexTaskID          `json:"taskId"` // 兼容 JSON string 和 number 两种格式
	TenantID       int64               `json:"tenantId"`
	StoreID        int64               `json:"storeId"`
	SourcePlatform string              `json:"sourcePlatform"`
	TargetPlatform string              `json:"targetPlatform"`
	Region         string              `json:"region"`
	CategoryID     int64               `json:"categoryId"`
	ProductID      string              `json:"productId"`
	Priority       int                 `json:"priority"`
	RetryCount     int                 `json:"retryCount"`
	MaxRetryCount  int                 `json:"maxRetryCount"`
	CreatedAt      *types.FlexibleTime `json:"createdAt"`
	Remark         string              `json:"remark,omitempty"`
	Status         string              `json:"status,omitempty"`
}

// flexTaskID 兼容 JSON 中 string 和 number 两种格式的任务ID
// 分布式爬虫用 string（哈希值可能超出 int64），普通任务用 number
type flexTaskID string

func (f *flexTaskID) UnmarshalJSON(b []byte) error {
	s := string(b)
	// string 格式：去掉引号直接用
	if len(s) >= 2 && s[0] == '"' {
		*f = flexTaskID(s[1 : len(s)-1])
		return nil
	}
	// number 格式：直接保留原始字符串表示
	*f = flexTaskID(s)
	return nil
}

func (f flexTaskID) Int64() int64 {
	var n int64
	fmt.Sscanf(string(f), "%d", &n)
	return n
}

func (f flexTaskID) String() string {
	return string(f)
}

// MessageAdapter 任务消息适配器，负责任务对象与消息格式之间的转换
type MessageAdapter struct {
	queueMapping map[string]string
}

// NewMessageAdapter 创建任务消息适配器
func NewMessageAdapter() *MessageAdapter {
	return &MessageAdapter{
		queueMapping: map[string]string{
			"amazon.crawler": "amazon.crawler",
			"1688.crawler":   "1688.crawler",
		},
	}
}

// MessageToTask 将消息转换为任务对象
func (a *MessageAdapter) MessageToTask(msg *Message) (*model.Task, error) {
	if msg == nil {
		return nil, fmt.Errorf("消息不能为空")
	}

	taskMsg, err := a.parseTaskMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("解析任务消息失败: %w", err)
	}

	status := a.ConvertStatusStringToInt16(taskMsg.Status)

	var createTime int64
	if taskMsg.CreatedAt != nil && !taskMsg.CreatedAt.IsZero() {
		createTime = taskMsg.CreatedAt.Unix()
	}

	targetPlatform := taskMsg.TargetPlatform
	if targetPlatform == "" {
		targetPlatform = taskMsg.SourcePlatform
	}

	sourcePlatform := taskMsg.SourcePlatform
	if sourcePlatform == "" {
		sourcePlatform = targetPlatform
	}

	task := &model.Task{
		ID:             taskMsg.TaskID.Int64(),
		TenantID:       taskMsg.TenantID,
		StoreID:        taskMsg.StoreID,
		Platform:       targetPlatform,
		SourcePlatform: sourcePlatform,
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

	taskMsg := &TaskMessage{
		TaskID:         flexTaskID(fmt.Sprintf("%d", task.ID)),
		TenantID:       task.TenantID,
		StoreID:        task.StoreID,
		SourcePlatform: task.SourcePlatform,
		TargetPlatform: task.Platform,
		Region:         task.Region,
		CategoryID:     task.CategoryID,
		ProductID:      task.ProductID,
		Priority:       task.Priority,
		RetryCount:     task.RetryCount,
		MaxRetryCount:  task.MaxRetryCount,
		CreatedAt:      types.ToFlexibleTime(&time.Time{}),
		Remark:         task.Remark,
		Status:         a.convertStatusInt16ToString(task.Status),
	}

	if task.CreateTime > 0 {
		t := time.Unix(task.CreateTime, 0)
		taskMsg.CreatedAt = types.ToFlexibleTime(&t)
	}

	return taskMsg, nil
}

// GetQueueName 根据平台获取爬虫队列名称（仅用于爬虫任务）
func (a *MessageAdapter) GetQueueName(platform string) string {
	if queue, ok := a.queueMapping[platform]; ok {
		return queue
	}
	return "amazon.crawler"
}

// CalculatePriority 将业务优先级(1-10)转换为消息优先级(0-10)
func (a *MessageAdapter) CalculatePriority(businessPriority int) uint8 {
	if businessPriority < 1 {
		businessPriority = 1
	}
	if businessPriority > 10 {
		businessPriority = 10
	}
	return uint8(11 - businessPriority)
}

// BuildRoutingKey 构建路由键，格式: {targetPlatform}.{sourcePlatform}.{priority}.{region}
func (a *MessageAdapter) BuildRoutingKey(task *model.Task) string {
	priorityLevel := a.getPriorityLevel(task.Priority)

	sourcePlatform := task.SourcePlatform
	if sourcePlatform == "" {
		sourcePlatform = task.Platform
	}

	return fmt.Sprintf("%s.%s.%s.%s",
		task.Platform,
		sourcePlatform,
		priorityLevel,
		task.Region)
}

// ConvertStatusStringToInt16 将字符串状态转换为 int16
func (a *MessageAdapter) ConvertStatusStringToInt16(statusStr string) int16 {
	switch statusStr {
	case "pending":
		return 0
	case "processing":
		return 1
	case "crawled":
		return 2
	case "failed":
		return 3
	case "pending_retry":
		return 4
	case "queued":
		return 5
	case "completed":
		return 6
	case "draft":
		return 8
	case "paused":
		return 10
	case "terminated":
		return 13
	default:
		return 0
	}
}

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

func (a *MessageAdapter) parseTaskMessage(msg *Message) (*TaskMessage, error) {
	if len(msg.Payload) == 0 {
		return nil, fmt.Errorf("消息载荷为空")
	}

	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return nil, fmt.Errorf("序列化载荷失败: %w", err)
	}

	var taskMsg TaskMessage
	if err = json.Unmarshal(payloadBytes, &taskMsg); err != nil {
		return nil, fmt.Errorf("反序列化任务消息失败: %w", err)
	}

	if taskMsg.RetryCount == 0 && msg.RetryCount > 0 {
		taskMsg.RetryCount = msg.RetryCount
	}
	if taskMsg.MaxRetryCount == 0 && msg.MaxRetries > 0 {
		taskMsg.MaxRetryCount = msg.MaxRetries
	}
	if taskMsg.MaxRetryCount == 0 {
		taskMsg.MaxRetryCount = 3
	}

	return &taskMsg, nil
}

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
