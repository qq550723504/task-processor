package rabbitmq

import (
	"encoding/json"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestParseDeliveryMessage_StandardFormat(t *testing.T) {
	// 准备测试数据
	payload := map[string]any{
		"task_id": "test-123",
		"action":  "process",
	}
	body, _ := json.Marshal(payload)

	delivery := amqp.Delivery{
		MessageId: "msg-001",
		Type:      "task.process",
		Body:      body,
		Timestamp: time.Now(),
		Priority:  5,
		Headers: amqp.Table{
			"retry_count": int32(2),
			"max_retries": int32(5),
		},
	}

	// 解析消息
	msg, err := parseDeliveryMessage(delivery)

	// 验证结果
	if err != nil {
		t.Fatalf("解析消息失败: %v", err)
	}

	if msg.ID != "msg-001" {
		t.Errorf("ID = %v, want %v", msg.ID, "msg-001")
	}

	if msg.Type != "task.process" {
		t.Errorf("Type = %v, want %v", msg.Type, "task.process")
	}

	if msg.Priority != 5 {
		t.Errorf("Priority = %v, want %v", msg.Priority, 5)
	}

	if msg.RetryCount != 2 {
		t.Errorf("RetryCount = %v, want %v", msg.RetryCount, 2)
	}

	if msg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want %v", msg.MaxRetries, 5)
	}

	if msg.Payload == nil {
		t.Fatal("Payload 不应该为 nil")
	}

	if msg.Payload["task_id"] != "test-123" {
		t.Errorf("Payload[task_id] = %v, want %v", msg.Payload["task_id"], "test-123")
	}
}

func TestParseDeliveryMessage_EmptyBody(t *testing.T) {
	delivery := amqp.Delivery{
		MessageId: "msg-002",
		Type:      "empty",
		Body:      []byte{},
		Timestamp: time.Now(),
	}

	msg, err := parseDeliveryMessage(delivery)

	if err != nil {
		t.Fatalf("解析空消息失败: %v", err)
	}

	if msg.Payload != nil {
		t.Errorf("空消息的 Payload 应该为 nil, got %v", msg.Payload)
	}

	// 验证默认值
	if msg.MaxRetries != 3 {
		t.Errorf("默认 MaxRetries = %v, want %v", msg.MaxRetries, 3)
	}
}

func TestParseDeliveryMessage_InvalidJSON(t *testing.T) {
	delivery := amqp.Delivery{
		MessageId: "msg-003",
		Body:      []byte("invalid json {{{"),
		Timestamp: time.Now(),
	}

	_, err := parseDeliveryMessage(delivery)

	if err == nil {
		t.Error("解析无效 JSON 应该返回错误")
	}
}

func TestParseDeliveryMessage_NoHeaders(t *testing.T) {
	payload := map[string]any{"test": "data"}
	body, _ := json.Marshal(payload)

	delivery := amqp.Delivery{
		MessageId: "msg-004",
		Body:      body,
		Timestamp: time.Now(),
		Headers:   nil, // 没有 headers
	}

	msg, err := parseDeliveryMessage(delivery)

	if err != nil {
		t.Fatalf("解析消息失败: %v", err)
	}

	// 验证默认值
	if msg.RetryCount != 0 {
		t.Errorf("默认 RetryCount = %v, want %v", msg.RetryCount, 0)
	}

	if msg.MaxRetries != 3 {
		t.Errorf("默认 MaxRetries = %v, want %v", msg.MaxRetries, 3)
	}
}

func TestParseDeliveryMessage_PartialHeaders(t *testing.T) {
	payload := map[string]any{"test": "data"}
	body, _ := json.Marshal(payload)

	delivery := amqp.Delivery{
		MessageId: "msg-005",
		Body:      body,
		Timestamp: time.Now(),
		Headers: amqp.Table{
			"retry_count": int32(1),
			// 没有 max_retries
		},
	}

	msg, err := parseDeliveryMessage(delivery)

	if err != nil {
		t.Fatalf("解析消息失败: %v", err)
	}

	if msg.RetryCount != 1 {
		t.Errorf("RetryCount = %v, want %v", msg.RetryCount, 1)
	}

	// 应该使用默认值
	if msg.MaxRetries != 3 {
		t.Errorf("默认 MaxRetries = %v, want %v", msg.MaxRetries, 3)
	}
}

func TestParseDeliveryMessage_ComplexPayload(t *testing.T) {
	// 测试复杂的嵌套 payload
	payload := map[string]any{
		"task_id": "complex-001",
		"metadata": map[string]any{
			"user_id":   123,
			"timestamp": time.Now().Unix(),
		},
		"items": []any{
			"item1",
			"item2",
			map[string]any{"id": 1, "name": "test"},
		},
	}
	body, _ := json.Marshal(payload)

	delivery := amqp.Delivery{
		MessageId: "msg-006",
		Body:      body,
		Timestamp: time.Now(),
	}

	msg, err := parseDeliveryMessage(delivery)

	if err != nil {
		t.Fatalf("解析复杂消息失败: %v", err)
	}

	if msg.Payload == nil {
		t.Fatal("Payload 不应该为 nil")
	}

	// 验证嵌套结构
	metadata, ok := msg.Payload["metadata"].(map[string]any)
	if !ok {
		t.Error("metadata 应该是 map[string]any")
	}

	if metadata["user_id"].(float64) != 123 {
		t.Errorf("metadata.user_id = %v, want %v", metadata["user_id"], 123)
	}

	items, ok := msg.Payload["items"].([]any)
	if !ok {
		t.Error("items 应该是 []any")
	}

	if len(items) != 3 {
		t.Errorf("items 长度 = %v, want %v", len(items), 3)
	}
}
