// Package rabbitmq 提供任务消息适配器
package rabbitmq

import (
	"encoding/json"
	"fmt"
	"strconv"

	"task-processor/internal/domain/model"
)

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
	Remark        string `json:"remark,omitempty"` // 备注，用于标记任务类型（如"variant"）
}

// TaskMessageAdapter 任务消息适配器
type TaskMessageAdapter struct{}

// NewTaskMessageAdapter 创建任务消息适配器
func NewTaskMessageAdapter() *TaskMessageAdapter {
	return &TaskMessageAdapter{}
}

// MessageToTask 将RabbitMQ消息转换为任务对象
func (adapter *TaskMessageAdapter) MessageToTask(msg *Message) (*model.Task, error) {
	if msg == nil {
		return nil, fmt.Errorf("消息不能为空")
	}

	// 解析消息载荷
	taskMsg, err := adapter.parseTaskMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("解析任务消息失败: %w", err)
	}

	// 转换为任务对象
	task := &model.Task{
		ID:            taskMsg.TaskID,
		TenantID:      taskMsg.TenantID,
		StoreID:       taskMsg.StoreID,
		Platform:      taskMsg.Platform,
		Region:        taskMsg.Region,
		CategoryID:    taskMsg.CategoryID,
		ProductID:     taskMsg.ProductID,
		Status:        1, // 处理中状态
		RetryCount:    taskMsg.RetryCount,
		MaxRetryCount: taskMsg.MaxRetryCount,
		Priority:      taskMsg.Priority,
		Remark:        taskMsg.Remark, // 传递备注字段
		CreateTime:    taskMsg.CreatedAt,
		UpdateTime:    taskMsg.CreatedAt,
	}

	return task, nil
}

// TaskToMessage 将任务对象转换为RabbitMQ消息
func (adapter *TaskMessageAdapter) TaskToMessage(task *model.Task) (*Message, error) {
	if task == nil {
		return nil, fmt.Errorf("任务不能为空")
	}

	// 创建任务消息
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

	// 序列化任务消息
	payload, err := adapter.taskMessageToPayload(taskMsg)
	if err != nil {
		return nil, fmt.Errorf("序列化任务消息失败: %w", err)
	}

	// 创建RabbitMQ消息
	msg := &Message{
		ID:         strconv.FormatInt(task.ID, 10),
		Type:       "task",
		Payload:    payload,
		Priority:   adapter.calculateRabbitMQPriority(task.Priority),
		Timestamp:  task.CreateTime,
		RetryCount: task.RetryCount,
		MaxRetries: task.MaxRetryCount,
	}

	return msg, nil
}

// parseTaskMessage 解析任务消息
func (adapter *TaskMessageAdapter) parseTaskMessage(msg *Message) (*TaskMessage, error) {
	// 如果载荷为空，尝试从消息ID解析
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

// taskMessageToPayload 将任务消息转换为载荷
func (adapter *TaskMessageAdapter) taskMessageToPayload(taskMsg TaskMessage) (map[string]interface{}, error) {
	// 序列化任务消息
	data, err := json.Marshal(taskMsg)
	if err != nil {
		return nil, fmt.Errorf("序列化任务消息失败: %w", err)
	}

	// 反序列化为map
	var payload map[string]interface{}
	err = json.Unmarshal(data, &payload)
	if err != nil {
		return nil, fmt.Errorf("转换载荷失败: %w", err)
	}

	return payload, nil
}

// calculateRabbitMQPriority 计算RabbitMQ优先级
// 业务优先级(1-10) -> RabbitMQ优先级(0-10)，数字越大优先级越高
func (adapter *TaskMessageAdapter) calculateRabbitMQPriority(businessPriority int) uint8 {
	// 业务优先级范围检查
	if businessPriority < 1 {
		businessPriority = 1
	}
	if businessPriority > 10 {
		businessPriority = 10
	}

	// 转换为RabbitMQ优先级（1-10 -> 10-1，然后映射到0-10）
	// 业务优先级1（最高）-> RabbitMQ优先级10
	// 业务优先级10（最低）-> RabbitMQ优先级1
	rabbitMQPriority := 11 - businessPriority

	return uint8(rabbitMQPriority)
}

// BuildRoutingKey 构建路由键 - 简化格式，只按平台和优先级路由
func (adapter *TaskMessageAdapter) BuildRoutingKey(task *model.Task) string {
	// 格式: {platform}.{priority}
	priorityLevel := adapter.getPriorityLevel(task.Priority)
	return fmt.Sprintf("%s.%s", task.Platform, priorityLevel)
}

// getPriorityLevel 获取优先级级别
func (adapter *TaskMessageAdapter) getPriorityLevel(priority int) string {
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

// GetQueueName 根据平台获取队列名称
func (adapter *TaskMessageAdapter) GetQueueName(platform string) string {
	switch platform {
	case "amazon":
		return "amazon.tasks.queue"
	case "temu":
		return "temu.tasks.queue"
	case "shein":
		return "shein.tasks.queue"
	default:
		return "amazon.tasks.queue" // 默认队列
	}
}
