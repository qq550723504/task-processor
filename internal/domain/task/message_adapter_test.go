package task

import (
	"testing"

	"task-processor/internal/domain/model"

	"github.com/stretchr/testify/assert"
)

func TestMessageAdapter_GetQueueName(t *testing.T) {
	adapter := NewMessageAdapter()

	tests := []struct {
		name     string
		platform string
		expected string
	}{
		{"Amazon平台", "amazon", "amazon.tasks.queue"},
		{"Temu平台", "temu", "temu.tasks.queue"},
		{"Shein平台", "shein", "shein.tasks.queue"},
		{"未知平台", "unknown", "amazon.tasks.queue"}, // 默认队列
		{"空平台", "", "amazon.tasks.queue"},         // 默认队列
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueName := adapter.GetQueueName(tt.platform)
			assert.Equal(t, tt.expected, queueName)
		})
	}
}

func TestMessageAdapter_CalculatePriority(t *testing.T) {
	adapter := NewMessageAdapter()

	tests := []struct {
		name             string
		businessPriority int
		expected         uint8
	}{
		{"最高优先级", 1, 10},
		{"高优先级", 3, 8},
		{"中优先级", 5, 6},
		{"低优先级", 10, 1},
		{"超出范围-低", 0, 10}, // 自动调整为1
		{"超出范围-高", 15, 1}, // 自动调整为10
		{"负数", -5, 10},    // 自动调整为1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority := adapter.CalculatePriority(tt.businessPriority)
			assert.Equal(t, tt.expected, priority)
		})
	}
}

func TestMessageAdapter_BuildRoutingKey(t *testing.T) {
	adapter := NewMessageAdapter()

	tests := []struct {
		name     string
		task     *model.Task
		expected string
	}{
		{
			name: "紧急任务",
			task: &model.Task{
				Platform: "amazon",
				Priority: 1,
			},
			expected: "amazon.urgent",
		},
		{
			name: "高优先级任务",
			task: &model.Task{
				Platform: "temu",
				Priority: 5,
			},
			expected: "temu.high",
		},
		{
			name: "普通任务",
			task: &model.Task{
				Platform: "shein",
				Priority: 7,
			},
			expected: "shein.normal",
		},
		{
			name: "低优先级任务",
			task: &model.Task{
				Platform: "amazon",
				Priority: 10,
			},
			expected: "amazon.low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routingKey := adapter.BuildRoutingKey(tt.task)
			assert.Equal(t, tt.expected, routingKey)
		})
	}
}

func TestMessageAdapter_TaskToMessage(t *testing.T) {
	adapter := NewMessageAdapter()

	task := &model.Task{
		ID:            12345,
		TenantID:      1,
		StoreID:       100,
		Platform:      "amazon",
		Region:        "US",
		CategoryID:    200,
		ProductID:     "B001",
		Priority:      1,
		RetryCount:    0,
		MaxRetryCount: 3,
		CreateTime:    1234567890,
		Remark:        "test",
		Status:        0, // pending
	}

	taskMsg, err := adapter.TaskToMessage(task)

	assert.NoError(t, err)
	assert.NotNil(t, taskMsg)
	assert.Equal(t, int64(12345), taskMsg.TaskID)
	assert.Equal(t, int64(1), taskMsg.TenantID)
	assert.Equal(t, int64(100), taskMsg.StoreID)
	assert.Equal(t, "amazon", taskMsg.Platform)
	assert.Equal(t, "US", taskMsg.Region)
	assert.Equal(t, int64(200), taskMsg.CategoryID)
	assert.Equal(t, "B001", taskMsg.ProductID)
	assert.Equal(t, 1, taskMsg.Priority)
	assert.Equal(t, 0, taskMsg.RetryCount)
	assert.Equal(t, 3, taskMsg.MaxRetryCount)
	assert.Equal(t, int64(1234567890), taskMsg.CreatedAt)
	assert.Equal(t, "test", taskMsg.Remark)
	assert.Equal(t, "pending", taskMsg.Status)
}

func TestMessageAdapter_TaskToMessage_NilTask(t *testing.T) {
	adapter := NewMessageAdapter()

	taskMsg, err := adapter.TaskToMessage(nil)

	assert.Error(t, err)
	assert.Nil(t, taskMsg)
	assert.Contains(t, err.Error(), "任务不能为空")
}

func TestMessageAdapter_MessageToTask(t *testing.T) {
	adapter := NewMessageAdapter()

	msg := &Message{
		ID:   "12345",
		Type: "task",
		Payload: map[string]interface{}{
			"taskId":        float64(12345), // JSON 数字默认是 float64
			"tenantId":      float64(1),
			"storeId":       float64(100),
			"platform":      "amazon",
			"region":        "US",
			"categoryId":    float64(200),
			"productId":     "B001",
			"priority":      float64(1),
			"retryCount":    float64(0),
			"maxRetryCount": float64(3),
			"createdAt":     float64(1234567890),
			"remark":        "test",
			"status":        "pending",
		},
		Priority:   10,
		Timestamp:  1234567890,
		RetryCount: 0,
		MaxRetries: 3,
	}

	task, err := adapter.MessageToTask(msg)

	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, int64(12345), task.ID)
	assert.Equal(t, int64(1), task.TenantID)
	assert.Equal(t, int64(100), task.StoreID)
	assert.Equal(t, "amazon", task.Platform)
	assert.Equal(t, "US", task.Region)
	assert.Equal(t, int64(200), task.CategoryID)
	assert.Equal(t, "B001", task.ProductID)
	assert.Equal(t, 1, task.Priority)
	assert.Equal(t, int16(0), task.Status) // pending
	assert.Equal(t, "test", task.Remark)
}

func TestMessageAdapter_MessageToTask_NilMessage(t *testing.T) {
	adapter := NewMessageAdapter()

	task, err := adapter.MessageToTask(nil)

	assert.Error(t, err)
	assert.Nil(t, task)
	assert.Contains(t, err.Error(), "消息不能为空")
}

func TestMessageAdapter_StatusConversion(t *testing.T) {
	adapter := NewMessageAdapter()

	tests := []struct {
		name      string
		statusStr string
		statusInt int16
	}{
		{"待处理", "pending", 0},
		{"处理中", "processing", 1},
		{"已爬取", "crawled", 2},
		{"失败", "failed", 3},
		{"待重试", "pending_retry", 4},
		{"已排队", "queued", 5},
		{"已完成", "completed", 6},
		{"草稿", "draft", 8},
		{"暂停", "paused", 10},
		{"终止", "terminated", 13},
		{"未知", "unknown_status", 0}, // 默认为 pending
		{"空字符串", "", 0},             // 默认为 pending
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 测试字符串转int16
			statusInt := adapter.convertStatusStringToInt16(tt.statusStr)
			assert.Equal(t, tt.statusInt, statusInt)

			// 测试int16转字符串（如果不是未知状态）
			if tt.statusStr != "unknown_status" && tt.statusStr != "" {
				statusStr := adapter.convertStatusInt16ToString(tt.statusInt)
				assert.Equal(t, tt.statusStr, statusStr)
			}
		})
	}
}

func TestMessageAdapter_GetPriorityLevel(t *testing.T) {
	adapter := NewMessageAdapter()

	tests := []struct {
		name     string
		priority int
		expected string
	}{
		{"紧急-1", 1, "urgent"},
		{"紧急-2", 2, "urgent"},
		{"紧急-3", 3, "urgent"},
		{"高-4", 4, "high"},
		{"高-5", 5, "high"},
		{"高-6", 6, "high"},
		{"普通-7", 7, "normal"},
		{"普通-8", 8, "normal"},
		{"低-9", 9, "low"},
		{"低-10", 10, "low"},
		{"低-超出", 15, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := adapter.getPriorityLevel(tt.priority)
			assert.Equal(t, tt.expected, level)
		})
	}
}

func BenchmarkMessageAdapter_TaskToMessage(b *testing.B) {
	adapter := NewMessageAdapter()
	task := &model.Task{
		ID:         12345,
		TenantID:   1,
		Platform:   "amazon",
		Region:     "US",
		ProductID:  "B001",
		Priority:   1,
		CreateTime: 1234567890,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.TaskToMessage(task)
	}
}

func BenchmarkMessageAdapter_GetQueueName(b *testing.B) {
	adapter := NewMessageAdapter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.GetQueueName("amazon")
	}
}

func BenchmarkMessageAdapter_CalculatePriority(b *testing.B) {
	adapter := NewMessageAdapter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.CalculatePriority(5)
	}
}
