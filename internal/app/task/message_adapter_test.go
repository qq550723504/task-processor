package task

import (
	"encoding/json"
	"testing"

	coremetrics "task-processor/internal/core/metrics"
	taskdomain "task-processor/internal/domain/task"
	"task-processor/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFlexTaskID_UnmarshalJSON 验证 flexTaskID 兼容 string 和 number 两种 JSON 格式
func TestFlexTaskID_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		name    string
		json    string
		wantStr string
		wantInt int64
	}{
		{"string 小数字", `"42"`, "42", 42},
		{"number 小数字", `42`, "42", 42},
		{"string 负数（FNV 哈希溢出）", `"-4941405290761185932"`, "-4941405290761185932", -4941405290761185932},
		{"number 负数", `-4941405290761185932`, "-4941405290761185932", -4941405290761185932},
		{"string 零", `"0"`, "0", 0},
		{"number 零", `0`, "0", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var fid flexTaskID
			err := json.Unmarshal([]byte(tc.json), &fid)
			require.NoError(t, err)
			assert.Equal(t, tc.wantStr, fid.String())
			assert.Equal(t, tc.wantInt, fid.Int64())
		})
	}
}

// TestFlexTaskID_InTaskMessage 验证 TaskMessage 反序列化时 TaskID 字段正确解析
func TestFlexTaskID_InTaskMessage(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantStr string
		wantInt int64
	}{
		{
			name:    "taskId 为 string（分布式爬虫格式）",
			payload: `{"taskId":"-4941405290761185932","sourcePlatform":"amazon","region":"US","productId":"B001TEST"}`,
			wantStr: "-4941405290761185932",
			wantInt: -4941405290761185932,
		},
		{
			name:    "taskId 为 number（普通任务格式）",
			payload: `{"taskId":12345,"sourcePlatform":"amazon","region":"US","productId":"B001TEST"}`,
			wantStr: "12345",
			wantInt: 12345,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var msg TaskMessage
			err := json.Unmarshal([]byte(tc.payload), &msg)
			require.NoError(t, err)
			assert.Equal(t, tc.wantStr, msg.TaskID.String())
			assert.Equal(t, tc.wantInt, msg.TaskID.Int64())
		})
	}
}

// TestMessageAdapter_MessageToTask_CrawlerTaskID 验证分布式爬虫任务 ID 经过 MessageToTask 后不为 0
func TestMessageAdapter_MessageToTask_CrawlerTaskID(t *testing.T) {
	adapter := NewMessageAdapter()

	msg := &Message{
		ID:   "430604922543791994",
		Type: "task",
		Payload: map[string]any{
			"taskId":         "430604922543791994", // string 格式（正数，掩码后）
			"sourcePlatform": "amazon",
			"targetPlatform": "amazon",
			"region":         "US",
			"productId":      "B001TEST",
			"storeId":        float64(2001),
			"tenantId":       float64(1001),
			"priority":       float64(5),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
		},
	}

	task, err := adapter.MessageToTask(msg)
	require.NoError(t, err)
	assert.Equal(t, int64(430604922543791994), task.ID, "Task.ID 应等于 FNV 哈希值（正数）")
	assert.Equal(t, "B001TEST", task.ProductID)
	assert.Equal(t, "US", task.Region)
}

func TestMessageAdapterMessageToTaskUsesLegacyPlatformAtAdapterBoundary(t *testing.T) {
	adapter := NewMessageAdapter()

	task, err := adapter.MessageToTask(&Message{
		ID:   "legacy-platform",
		Type: "task",
		Payload: map[string]any{
			"taskId":    float64(12345),
			"platform":  "shein",
			"productId": "B001TEST",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "shein", task.Platform)
	assert.Equal(t, "shein", task.SourcePlatform)
}

func TestMessageAdapterMessageToTaskAcceptsLegacySourceWithExplicitTarget(t *testing.T) {
	adapter := NewMessageAdapter()

	task, err := adapter.MessageToTask(&Message{
		ID:   "legacy-source-explicit-target",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(12345),
			"platform":       "amazon",
			"targetPlatform": "shein",
			"productId":      "B001TEST",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "shein", task.Platform)
	assert.Equal(t, "amazon", task.SourcePlatform)
}

func TestMessageAdapterMessageToTaskRejectsPlatformSourceConflict(t *testing.T) {
	adapter := NewMessageAdapter()

	_, err := adapter.MessageToTask(&Message{
		ID:   "conflict",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(12345),
			"platform":       "shein",
			"sourcePlatform": "amazon",
			"targetPlatform": "temu",
			"productId":      "B001TEST",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicts with sourcePlatform")
}

func TestMessageAdapterMessageToTaskRequiresTargetPlatform(t *testing.T) {
	adapter := NewMessageAdapter()

	_, err := adapter.MessageToTask(&Message{
		ID:   "missing-target",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(12345),
			"sourcePlatform": "amazon",
			"productId":      "B001TEST",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing target platform")
}

func TestMessageAdapterTaskToMessagePublishesTaskEventV2(t *testing.T) {
	adapter := NewMessageAdapter()

	event, err := adapter.TaskToMessage(&model.Task{
		ID:             12345,
		SourcePlatform: "amazon",
		Platform:       "shein",
		ProductID:      "B001TEST",
		TraceID:        "trace-123",
		Metadata:       map[string]string{"request_id": "request-456"},
	})
	require.NoError(t, err)
	assert.Equal(t, taskdomain.TaskEventSchemaVersionV2, event.SchemaVersion)
	assert.Equal(t, "12345", event.TaskID)
	assert.Equal(t, taskdomain.SourcePlatformAmazon, event.SourcePlatform)
	assert.Equal(t, taskdomain.TargetPlatformShein, event.TargetPlatform)
	assert.Equal(t, "trace-123", event.TraceID)
	assert.Equal(t, map[string]string{"request_id": "request-456"}, event.Metadata)
}

func TestMessageAdapterMessageToTaskDecodesTaskEventV2(t *testing.T) {
	adapter := NewMessageAdapter()
	payload, err := json.Marshal(TaskPayload{
		TaskID:         "12345",
		SourcePlatform: "amazon",
		TargetPlatform: "shein",
		ProductID:      "B001TEST",
	})
	require.NoError(t, err)

	task, err := adapter.MessageToTask(&Message{Payload: map[string]any{
		"schemaVersion":  float64(taskdomain.TaskEventSchemaVersionV2),
		"taskId":         "12345",
		"sourcePlatform": "amazon",
		"targetPlatform": "shein",
		"payload":        json.RawMessage(payload),
		"traceId":        "trace-123",
		"metadata":       map[string]any{"request_id": "request-456"},
	}})
	require.NoError(t, err)
	assert.Equal(t, int64(12345), task.ID)
	assert.Equal(t, "amazon", task.SourcePlatform)
	assert.Equal(t, "shein", task.Platform)
	assert.Equal(t, "trace-123", task.TraceID)
	assert.Equal(t, map[string]string{"request_id": "request-456"}, task.Metadata)
}

func TestMessageAdapterLegacyDecodeRecordsMetric(t *testing.T) {
	metrics := coremetrics.GlobalTaskMetrics()
	before := metrics.GetSnapshot().LegacyTaskEventDecodedCount

	_, err := NewMessageAdapter().MessageToTask(&Message{Payload: map[string]any{
		"taskId":    float64(12345),
		"platform":  "shein",
		"productId": "B001TEST",
	}})
	require.NoError(t, err)
	assert.Equal(t, before+1, metrics.GetSnapshot().LegacyTaskEventDecodedCount)
}

func TestMessageAdapterMessageToTaskRejectsConflictingV2PayloadRouting(t *testing.T) {
	adapter := NewMessageAdapter()
	payload, err := json.Marshal(TaskPayload{
		TaskID:         "12345",
		SourcePlatform: "1688",
		TargetPlatform: "shein",
		ProductID:      "B001TEST",
	})
	require.NoError(t, err)

	_, err = adapter.MessageToTask(&Message{Payload: map[string]any{
		"schemaVersion":  float64(taskdomain.TaskEventSchemaVersionV2),
		"taskId":         "12345",
		"sourcePlatform": "amazon",
		"targetPlatform": "shein",
		"payload":        json.RawMessage(payload),
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payload sourcePlatform")
}

func TestMessageAdapterGetQueueNameRejectsUnknownCrawlerRoute(t *testing.T) {
	adapter := NewMessageAdapter()

	_, err := adapter.GetQueueName("unknown.crawler")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UNKNOWN_CRAWLER_ROUTE")
}
