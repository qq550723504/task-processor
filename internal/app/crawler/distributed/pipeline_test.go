// Package distributed 端到端调用链测试
// 覆盖：crawlTaskID → publishCrawlTask → extractNestedPayload → parseTaskMessage → sendCrawlResult → handleMessage → PendingRegistry.Deliver
package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// crawlTaskIDForTest 复制 fetcher 包里的 crawlTaskID 逻辑，用于测试（避免跨包依赖）
func crawlTaskIDForTest(productID, region string) string {
	h := fnv.New64a()
	h.Write([]byte(productID + ":" + region))
	return fmt.Sprintf("%d", int64(h.Sum64()&0x7fffffffffffffff))
}

// --- 1. crawlTaskID 生成的 ID 能被 CrawlResult.TaskID（string）正确解析 ---

func TestCrawlTaskID_ParsedByCrawlResult(t *testing.T) {
	cases := []struct {
		productID string
		region    string
	}{
		{"B001TEST", "US"},
		{"B00AMAZON", "JP"},
		{"SHEIN12345", "EU"},
		{"TEMU99999", "UK"},
	}

	for _, tc := range cases {
		t.Run(tc.productID+":"+tc.region, func(t *testing.T) {
			id := crawlTaskIDForTest(tc.productID, tc.region)
			assert.NotEmpty(t, id, "crawlTaskID 不应为空")

			// 模拟 sendCrawlResult 序列化结果
			resultJSON := fmt.Sprintf(`{"taskId":%q,"success":true,"nodeId":"n1"}`, id)

			var result CrawlResult
			err := json.Unmarshal([]byte(resultJSON), &result)
			require.NoError(t, err, "CrawlResult 应能反序列化")
			assert.Equal(t, id, result.TaskID, "TaskID 应等于原始 crawlTaskID")
		})
	}
}

// --- 2. publishCrawlTask 序列化的消息体里 taskId 字段格式 ---

func TestPublishCrawlTask_MessageFormat(t *testing.T) {
	pub := &mockPublisher{}
	mock := newMockDeclarer()
	client := newTestClient(pub, mock, 5*time.Second)

	// 手动启动监听器（避免 SubmitCrawlTask 阻塞）
	_, err := client.ensureListenerStarted()
	require.NoError(t, err)

	taskID := crawlTaskIDForTest("B001TEST", "US")
	req := &CrawlRequest{
		TaskID:    taskID,
		Platform:  "amazon",
		Region:    "US",
		ProductID: "B001TEST",
		Priority:  5,
	}

	replyTo := client.listener.QueueName()
	err = client.publishCrawlTask(context.Background(), req, "amazon.crawler.normal", replyTo, taskID)
	require.NoError(t, err)
	require.Len(t, pub.published, 1)

	// 解析发布的消息体
	var msg map[string]any
	require.NoError(t, json.Unmarshal(pub.published[0].body, &msg))

	payload, ok := msg["payload"].(map[string]any)
	require.True(t, ok, "消息体应包含 payload 字段")

	// taskId 在 payload 里的 "id" 字段，应为 string 类型
	idVal, exists := payload["id"]
	require.True(t, exists, "payload 应包含 id 字段")
	idStr, isString := idVal.(string)
	require.True(t, isString, "payload.id 应为 string 类型，实际类型: %T, 值: %v", idVal, idVal)
	assert.Equal(t, taskID, idStr, "payload.id 应等于 crawlTaskID 生成的值")

	// reply_to 应存在
	replyToVal, exists := payload["reply_to"]
	require.True(t, exists, "payload 应包含 reply_to 字段")
	assert.NotEmpty(t, replyToVal, "reply_to 不应为空")
}

// --- 3. extractNestedPayload 把 id 映射为 taskId 后，能正确序列化为 JSON ---

func TestExtractNestedPayload_TaskIDMapping(t *testing.T) {
	taskID := crawlTaskIDForTest("B001TEST", "US")

	// 模拟 publishCrawlTask 发出的消息结构（经过 rabbitmq.Client.ParseMessage 解析后）
	innerPayload := map[string]any{
		"id":             taskID, // 内层 payload 里的 id（string 类型）
		"tenantId":       float64(1001),
		"storeId":        float64(2001),
		"sourcePlatform": "amazon",
		"region":         "US",
		"productId":      "B001TEST",
		"priority":       float64(5),
		"reply_to":       "crawler.results.node-123",
		"retryCount":     float64(0),
		"maxRetryCount":  float64(3),
	}

	// 模拟 extractNestedPayload 的逻辑：把 id 映射为 taskId
	if id, exists := innerPayload["id"]; exists {
		innerPayload["taskId"] = id
	}

	// 序列化再反序列化（模拟 parseTaskMessage 的处理）
	payloadBytes, err := json.Marshal(innerPayload)
	require.NoError(t, err)

	// taskId 应为 string 格式
	var rawMap map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payloadBytes, &rawMap))
	taskIDRaw := string(rawMap["taskId"])
	assert.True(t, len(taskIDRaw) >= 2 && taskIDRaw[0] == '"',
		"taskId 应为 JSON string 格式，实际: %s", taskIDRaw)

	// 反序列化为带 taskId 字段的结构
	var parsed struct {
		TaskID string `json:"taskId"`
		Region string `json:"region"`
	}
	require.NoError(t, json.Unmarshal(payloadBytes, &parsed))
	assert.Equal(t, taskID, parsed.TaskID)
	assert.Equal(t, "US", parsed.Region)
}

// --- 4. sendCrawlResult 序列化的结果里 taskId 字段，handleMessage 能正确反序列化 ---

func TestSendCrawlResult_HandleMessage_RoundTrip(t *testing.T) {
	taskID := crawlTaskIDForTest("B001TEST", "US")

	// 模拟 crawler_processor.sendCrawlResult 构造的结果 map
	result := map[string]any{
		"taskId":   taskID, // string 类型
		"success":  true,
		"duration": int64(1500000000),
		"nodeId":   "crawler-node-1",
	}

	resultBytes, err := json.Marshal(result)
	require.NoError(t, err)

	// 验证序列化后 taskId 是 string 格式（带引号）
	var rawMap map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resultBytes, &rawMap))
	taskIDRaw := string(rawMap["taskId"])
	assert.True(t, len(taskIDRaw) >= 2 && taskIDRaw[0] == '"',
		"taskId 应序列化为 JSON string（带引号），实际: %s", taskIDRaw)

	// 模拟 result_listener.handleMessage 反序列化
	var crawlResult CrawlResult
	err = json.Unmarshal(resultBytes, &crawlResult)
	require.NoError(t, err, "CrawlResult 反序列化不应失败")

	assert.Equal(t, taskID, crawlResult.TaskID, "CrawlResult.TaskID 应等于原始 crawlTaskID")
	assert.True(t, crawlResult.Success)
}

// --- 5. PendingRegistry.Deliver 能用 result.TaskID 找到等待任务 ---

func TestPendingRegistry_Deliver_WithCrawlTaskID(t *testing.T) {
	taskID := crawlTaskIDForTest("B001TEST", "US")

	reg := NewPendingRegistry(5 * time.Second)
	pt := reg.Register(context.Background(), taskID)

	result := &CrawlResult{
		TaskID:  taskID,
		Success: true,
		NodeID:  "test-node",
	}

	delivered := reg.Deliver(result)
	require.True(t, delivered, "Deliver 应成功找到等待任务")

	got, err := reg.Wait(pt)
	require.NoError(t, err)
	assert.Equal(t, taskID, got.TaskID)
	assert.True(t, got.Success)
}

// --- 6. 端到端：发送任务 → 模拟爬虫处理 → 回写结果 → 客户端收到 ---

func TestEndToEnd_SubmitAndReceiveResult(t *testing.T) {
	pub := &mockPublisher{}
	mock := newMockDeclarer()
	client := newTestClient(pub, mock, 3*time.Second)

	taskID := crawlTaskIDForTest("B001TEST", "US")
	req := &CrawlRequest{
		TaskID:    taskID,
		Platform:  "amazon",
		Region:    "US",
		ProductID: "B001TEST",
		Priority:  5,
	}

	// 后台模拟爬虫节点：
	// 1. 等待消息发布
	// 2. 解析消息里的 taskId
	// 3. 构造结果并通过 result_listener 投递
	go func() {
		// 等待消息发布
		for len(pub.published) == 0 {
			time.Sleep(5 * time.Millisecond)
		}

		// 解析发布的消息，提取 taskId
		var msg map[string]any
		if err := json.Unmarshal(pub.published[0].body, &msg); err != nil {
			t.Errorf("解析发布消息失败: %v", err)
			return
		}
		payload := msg["payload"].(map[string]any)
		publishedTaskID := payload["id"].(string)

		// 构造结果（直接格式，RabbitMQAdapter 发原始 JSON 不包装）
		resultMap := map[string]any{
			"taskId":  publishedTaskID,
			"success": true,
			"nodeId":  "test-node",
		}
		resultBytes, _ := json.Marshal(resultMap)

		// 通过 result_listener 投递（模拟爬虫节点发送到 reply_to 队列）
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)
		listener := NewResultListener(mock, client.registry, logger)
		listener.queueName = client.listener.QueueName() // 复用同一个 registry

		delivery := amqp.Delivery{Body: resultBytes}
		listener.handleMessage(delivery)
	}()

	result, err := client.SubmitCrawlTask(context.Background(), req)
	require.NoError(t, err, "端到端调用不应超时")
	assert.True(t, result.Success)
	assert.Equal(t, taskID, result.TaskID)
}
