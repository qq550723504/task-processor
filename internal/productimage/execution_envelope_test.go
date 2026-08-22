package productimage

import (
	"context"
	"testing"

	"task-processor/internal/shared/aiidentity"
)

func TestCreateProcessTaskCapturesExecutionEnvelope(t *testing.T) {
	repo := &contextAwareTaskRepo{}
	svc := &service{taskRepo: repo, requireAIIdentity: true}
	ctx := WithAIIdentity(context.Background(), AIIdentity{TenantID: "tenant-a", UserID: "user-a", TraceID: "trace-a"})
	task, err := svc.CreateProcessTask(ctx, &ImageProcessRequest{ProductURL: "https://example.test/product", Marketplace: "shein"})
	if err != nil {
		t.Fatalf("CreateProcessTask: %v", err)
	}
	envelope, err := task.ExecutionEnvelope()
	if err != nil {
		t.Fatalf("ExecutionEnvelope: %v", err)
	}
	if envelope.Version != aiidentity.CurrentEnvelopeVersion || envelope.TenantID != "tenant-a" || envelope.UserID != "user-a" || envelope.BusinessTaskID != task.ID || envelope.SourcePlatform != "productimage" || envelope.SourceTaskType != "image" {
		t.Fatalf("envelope = %+v", envelope)
	}
}
