package productenrich

import (
	"context"
	"testing"

	"task-processor/internal/shared/aiidentity"
)

func TestCreateGenerateTaskCapturesExecutionEnvelope(t *testing.T) {
	repo := newMockTaskRepo()
	svc, err := NewProductService(&ProductServiceConfig{TaskRepo: repo, RedisClient: &mockRedisClient{}})
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a", TraceID: "trace-a"})
	task, err := svc.CreateGenerateTask(ctx, &GenerateRequest{Text: "product"})
	if err != nil {
		t.Fatalf("CreateGenerateTask: %v", err)
	}
	envelope, err := task.ExecutionEnvelope()
	if err != nil {
		t.Fatalf("ExecutionEnvelope: %v", err)
	}
	if envelope.Version != aiidentity.CurrentEnvelopeVersion || envelope.TenantID != "tenant-a" || envelope.UserID != "user-a" || envelope.BusinessTaskID != task.ID || envelope.SourcePlatform != "productenrich" || envelope.SourceTaskType != "product" {
		t.Fatalf("envelope = %+v", envelope)
	}
}
