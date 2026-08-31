package rabbitmq

import (
	"encoding/json"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestMessage_Serialization 测试消息序列化
func TestMessage_Serialization(t *testing.T) {
	msg := &Message{
		ID:   "test-001",
		Type: "task.process",
		Payload: map[string]any{
			"task_id": "123",
			"action":  "process",
		},
		Priority:   5,
		Timestamp:  time.Now().Unix(),
		RetryCount: 0,
		MaxRetries: 3,
	}

	// 序列化
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 反序列化
	var decoded Message
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证
	if decoded.ID != msg.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, msg.ID)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Type = %v, want %v", decoded.Type, msg.Type)
	}

	if decoded.Priority != msg.Priority {
		t.Errorf("Priority = %v, want %v", decoded.Priority, msg.Priority)
	}
}

// TestClient_ParseMessage_StandardFormat 测试标准格式消息解析
func TestClient_ParseMessage_StandardFormat(t *testing.T) {
	client := &Client{}

	// 标准嵌套格式
	msg := Message{
		ID:   "msg-001",
		Type: "task.process",
		Payload: map[string]any{
			"task_id": "123",
			"action":  "process",
		},
		Priority:   5,
		Timestamp:  time.Now().Unix(),
		RetryCount: 0,
		MaxRetries: 3,
	}

	body, _ := json.Marshal(msg)

	delivery := amqp.Delivery{
		MessageId: "msg-001",
		Type:      "task.process",
		Body:      body,
		Timestamp: time.Now(),
		Priority:  5,
	}

	parsed, err := client.ParseMessage(delivery)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if parsed.ID != "msg-001" {
		t.Errorf("ID = %v, want %v", parsed.ID, "msg-001")
	}

	if parsed.Payload == nil {
		t.Fatal("Payload 不应该为 nil")
	}

	if parsed.Payload["task_id"] != "123" {
		t.Errorf("Payload[task_id] = %v, want %v", parsed.Payload["task_id"], "123")
	}
}

// TestClient_ParseMessage_FlatFormat 测试扁平格式消息解析
func TestClient_ParseMessage_FlatFormat(t *testing.T) {
	client := &Client{}

	// 扁平格式（整个消息体作为 payload）
	flatMsg := map[string]any{
		"task_id": "456",
		"action":  "execute",
		"data":    "test data",
	}

	body, _ := json.Marshal(flatMsg)

	delivery := amqp.Delivery{
		MessageId: "msg-002",
		Type:      "task.execute",
		Body:      body,
		Timestamp: time.Now(),
		Priority:  3,
	}

	parsed, err := client.ParseMessage(delivery)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if parsed.ID != "msg-002" {
		t.Errorf("ID = %v, want %v", parsed.ID, "msg-002")
	}

	if parsed.Type != "task.execute" {
		t.Errorf("Type = %v, want %v", parsed.Type, "task.execute")
	}

	if parsed.Payload == nil {
		t.Fatal("Payload 不应该为 nil")
	}

	if parsed.Payload["task_id"] != "456" {
		t.Errorf("Payload[task_id] = %v, want %v", parsed.Payload["task_id"], "456")
	}

	if parsed.Payload["action"] != "execute" {
		t.Errorf("Payload[action] = %v, want %v", parsed.Payload["action"], "execute")
	}
}

// TestClient_ParseMessage_NoMessageID 测试没有 MessageID 的情况
func TestClient_ParseMessage_NoMessageID(t *testing.T) {
	client := &Client{}

	flatMsg := map[string]any{
		"task_id": "789",
	}

	body, _ := json.Marshal(flatMsg)

	delivery := amqp.Delivery{
		MessageId: "", // 空 MessageID
		Body:      body,
		Timestamp: time.Now(),
	}

	parsed, err := client.ParseMessage(delivery)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 应该生成一个 ID
	if parsed.ID == "" {
		t.Error("应该生成一个 MessageID")
	}

	if parsed.ID[:4] != "msg-" {
		t.Errorf("生成的 ID 应该以 'msg-' 开头, got %v", parsed.ID)
	}
}

// TestClient_ParseMessage_WithRetryHeaders 测试带重试信息的消息
func TestClient_ParseMessage_WithRetryHeaders(t *testing.T) {
	client := &Client{}

	flatMsg := map[string]any{
		"task_id": "retry-001",
	}

	body, _ := json.Marshal(flatMsg)

	delivery := amqp.Delivery{
		MessageId: "msg-retry-001",
		Body:      body,
		Timestamp: time.Now(),
		Headers: amqp.Table{
			"retry_count": int32(2),
			"max_retries": int32(5),
		},
	}

	parsed, err := client.ParseMessage(delivery)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if parsed.RetryCount != 2 {
		t.Errorf("RetryCount = %v, want %v", parsed.RetryCount, 2)
	}

	if parsed.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want %v", parsed.MaxRetries, 5)
	}
}

// TestClient_ParseMessage_InvalidJSON 测试无效 JSON
func TestClient_ParseMessage_InvalidJSON(t *testing.T) {
	client := &Client{}

	delivery := amqp.Delivery{
		MessageId: "msg-invalid",
		Body:      []byte("invalid json {{{"),
		Timestamp: time.Now(),
	}

	_, err := client.ParseMessage(delivery)
	if err == nil {
		t.Error("解析无效 JSON 应该返回错误")
	}
}

// TestClient_ParseMessage_DefaultValues 测试默认值
func TestClient_ParseMessage_DefaultValues(t *testing.T) {
	client := &Client{}

	flatMsg := map[string]any{
		"task_id": "default-001",
	}

	body, _ := json.Marshal(flatMsg)

	delivery := amqp.Delivery{
		MessageId: "msg-default",
		Body:      body,
		Timestamp: time.Now(),
		// 没有 Headers
	}

	parsed, err := client.ParseMessage(delivery)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 验证默认值
	if parsed.RetryCount != 0 {
		t.Errorf("默认 RetryCount = %v, want %v", parsed.RetryCount, 0)
	}

	if parsed.MaxRetries != 3 {
		t.Errorf("默认 MaxRetries = %v, want %v", parsed.MaxRetries, 3)
	}
}

// TestPublishOptions_Defaults 测试发布选项
func TestPublishOptions_Defaults(t *testing.T) {
	opts := PublishOptions{
		Exchange:   "test-exchange",
		RoutingKey: "test.key",
		Priority:   5,
		Persistent: true,
	}

	if opts.Exchange != "test-exchange" {
		t.Errorf("Exchange = %v, want %v", opts.Exchange, "test-exchange")
	}

	if opts.Priority != 5 {
		t.Errorf("Priority = %v, want %v", opts.Priority, 5)
	}

	if !opts.Persistent {
		t.Error("Persistent 应该为 true")
	}
}

// TestConsumeOptions_Defaults 测试消费选项
func TestConsumeOptions_Defaults(t *testing.T) {
	opts := ConsumeOptions{
		Queue:     "test-queue",
		Consumer:  "test-consumer",
		AutoAck:   false,
		Exclusive: false,
	}

	if opts.Queue != "test-queue" {
		t.Errorf("Queue = %v, want %v", opts.Queue, "test-queue")
	}

	if opts.Consumer != "test-consumer" {
		t.Errorf("Consumer = %v, want %v", opts.Consumer, "test-consumer")
	}

	if opts.AutoAck {
		t.Error("AutoAck 应该为 false")
	}
}
