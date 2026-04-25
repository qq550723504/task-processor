package task

import (
	"encoding/json"
	"testing"

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

func TestMessageAdapterMessageToTaskRejectsPlatformTargetConflict(t *testing.T) {
	adapter := NewMessageAdapter()

	_, err := adapter.MessageToTask(&Message{
		ID:   "conflict",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(12345),
			"platform":       "shein",
			"targetPlatform": "temu",
			"productId":      "B001TEST",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicts with targetPlatform")
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
