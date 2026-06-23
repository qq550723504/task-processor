package consumer

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	"task-processor/internal/taskstatus"
)

type stubDeadLetterRuntime struct {
	status  *taskstatus.TaskStatusSnapshot
	updates []*listingruntime.TaskStatusUpdate
}

func (s *stubDeadLetterRuntime) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	s.updates = append(s.updates, req)
	return nil
}

func (s *stubDeadLetterRuntime) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	return s.status, nil
}

func TestDeadLetterHandlerRecordsRabbitMQDeathHeaders(t *testing.T) {
	runtime := &stubDeadLetterRuntime{
		status: &taskstatus.TaskStatusSnapshot{
			TaskID:          8189411,
			StatusKey:       "QUEUED",
			CanonicalStatus: "pending",
		},
	}
	handler := NewDeadLetterHandler(DeadLetterHandlerConfig{Runtime: runtime})

	msg := &rabbitmq.Message{
		ID: "msg-1",
		Payload: map[string]any{
			"taskId":         float64(8189411),
			"tenantId":       float64(246),
			"storeId":        float64(976),
			"platform":       "amazon",
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0BW4B27VY",
			"status":         "queued",
		},
		Headers: map[string]any{
			"x-first-death-reason": "rejected",
			"x-first-death-queue":  "shein.tasks.store.976",
		},
	}

	if err := handler.HandleMessage(context.Background(), msg); err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if len(runtime.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(runtime.updates))
	}
	update := runtime.updates[0]
	if update.Status != model.TaskStatusPendingRetry.Int16() {
		t.Fatalf("status = %d, want %d", update.Status, model.TaskStatusPendingRetry.Int16())
	}
	if update.ReasonCode != "DEAD_LETTER" || update.Stage != "dead_letter" {
		t.Fatalf("reason/stage = %q/%q, want DEAD_LETTER/dead_letter", update.ReasonCode, update.Stage)
	}
	if !strings.Contains(update.ErrorMessage, "rejected") || !strings.Contains(update.ErrorMessage, "shein.tasks.store.976") {
		t.Fatalf("error message = %q, want RabbitMQ reason and source queue", update.ErrorMessage)
	}
}

func TestDeadLetterHandlerDoesNotOverwriteTerminalTask(t *testing.T) {
	runtime := &stubDeadLetterRuntime{
		status: &taskstatus.TaskStatusSnapshot{
			TaskID:          8189416,
			StatusKey:       "TERMINATED",
			CanonicalStatus: "failed",
			ErrorMessage:    "[NON_RETRYABLE_FAILURE] product brand was not found in SHEIN authorized brand list",
		},
	}
	handler := NewDeadLetterHandler(DeadLetterHandlerConfig{Runtime: runtime})

	msg := &rabbitmq.Message{
		ID: "msg-terminal",
		Payload: map[string]any{
			"taskId":         float64(8189416),
			"storeId":        float64(976),
			"platform":       "amazon",
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0CYR2ZPT8",
			"status":         "queued",
		},
		Headers: map[string]any{
			"x-first-death-reason": "rejected",
			"x-first-death-queue":  "shein.tasks.store.976",
		},
	}

	if err := handler.HandleMessage(context.Background(), msg); err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if len(runtime.updates) != 0 {
		t.Fatalf("updates = %d, want 0", len(runtime.updates))
	}
}
