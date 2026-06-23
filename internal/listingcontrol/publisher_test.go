package listingcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"task-processor/internal/model"

	amqp "github.com/rabbitmq/amqp091-go"
)

type fakeAMQPPublisher struct {
	exchange  string
	key       string
	mandatory bool
	immediate bool
	msg       amqp.Publishing
	err       error
	calls     int
}

func (f *fakeAMQPPublisher) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	f.calls++
	f.exchange = exchange
	f.key = key
	f.mandatory = mandatory
	f.immediate = immediate
	f.msg = msg
	return f.err
}

func TestDispatchPublisherPublishesTaskMessageToStoreQueue(t *testing.T) {
	channel := &fakeAMQPPublisher{}
	publisher := NewDispatchPublisher(channel, "")
	input := &model.Task{
		ID:             12345,
		TenantID:       88,
		StoreID:        976,
		Platform:       "shein",
		SourcePlatform: "amazon",
		ProductID:      "B0CONTROL",
		Status:         1,
		Priority:       3,
	}

	dispatch, err := publisher.PublishTask(context.Background(), input)
	if err != nil {
		t.Fatalf("PublishTask returned error: %v", err)
	}

	if channel.calls != 1 {
		t.Fatalf("expected 1 publish call, got %d", channel.calls)
	}
	if channel.exchange != "" {
		t.Fatalf("expected default exchange, got %q", channel.exchange)
	}
	if channel.key != "shein.tasks.store.976" {
		t.Fatalf("expected store queue, got %q", channel.key)
	}
	if channel.mandatory {
		t.Fatal("expected mandatory=false")
	}
	if channel.immediate {
		t.Fatal("expected immediate=false")
	}
	if channel.msg.ContentType != "application/json" {
		t.Fatalf("expected content type application/json, got %q", channel.msg.ContentType)
	}
	if channel.msg.DeliveryMode != 2 {
		t.Fatalf("expected delivery mode 2, got %d", channel.msg.DeliveryMode)
	}
	if channel.msg.MessageId != "12345" {
		t.Fatalf("expected message id 12345, got %q", channel.msg.MessageId)
	}
	if channel.msg.Type != "task" {
		t.Fatalf("expected type task, got %q", channel.msg.Type)
	}
	if channel.msg.Priority != 8 {
		t.Fatalf("expected AMQP priority 8, got %d", channel.msg.Priority)
	}

	var payload map[string]any
	if err := json.Unmarshal(channel.msg.Body, &payload); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}

	assertJSONValue(t, payload, "taskId", "12345")
	assertJSONValue(t, payload, "tenantId", float64(88))
	assertJSONValue(t, payload, "storeId", float64(976))
	assertJSONValue(t, payload, "sourcePlatform", "amazon")
	assertJSONValue(t, payload, "targetPlatform", "shein")
	assertJSONValue(t, payload, "productId", "B0CONTROL")
	if _, ok := payload["status"]; !ok {
		t.Fatal("expected payload to contain status")
	}

	if dispatch.Queue != "shein.tasks.store.976" {
		t.Fatalf("expected dispatch queue, got %q", dispatch.Queue)
	}
	if dispatch.MessageID != "12345" {
		t.Fatalf("expected dispatch message id, got %q", dispatch.MessageID)
	}
	if dispatch.Priority != 8 {
		t.Fatalf("expected dispatch priority 8, got %d", dispatch.Priority)
	}
}

func TestDispatchPublisherSurfacesPublishError(t *testing.T) {
	publishErr := errors.New("rabbit unavailable")
	channel := &fakeAMQPPublisher{err: publishErr}
	publisher := NewDispatchPublisher(channel, "shein")

	_, err := publisher.PublishTask(context.Background(), &model.Task{
		ID:        55,
		TenantID:  10,
		StoreID:   976,
		Platform:  "shein",
		ProductID: "B0FAIL",
		Priority:  5,
	})
	if !errors.Is(err, publishErr) {
		t.Fatalf("expected publish error to be surfaced, got %v", err)
	}
}

func TestDispatchPublisherRejectsNilInputs(t *testing.T) {
	if _, err := (*DispatchPublisher)(nil).PublishTask(context.Background(), &model.Task{}); err == nil {
		t.Fatal("expected nil publisher error")
	}
	if _, err := NewDispatchPublisher(nil, "shein").PublishTask(context.Background(), &model.Task{}); err == nil {
		t.Fatal("expected nil channel error")
	}
	if _, err := NewDispatchPublisher(&fakeAMQPPublisher{}, "shein").PublishTask(context.Background(), nil); err == nil {
		t.Fatal("expected nil task error")
	}
}

func assertJSONValue(t *testing.T, payload map[string]any, key string, want any) {
	t.Helper()

	got, ok := payload[key]
	if !ok {
		t.Fatalf("expected payload to contain %q", key)
	}
	if got != want {
		t.Fatalf("expected payload[%q]=%v, got %v", key, want, got)
	}
}
