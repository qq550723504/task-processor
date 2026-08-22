package amazonlisting

import (
	"context"
	"testing"

	"task-processor/internal/shared/aiidentity"
)

func TestCreateGenerateTaskCapturesExecutionEnvelope(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{Repository: repo, ProductService: &stubProductService{}})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a", TraceID: "trace-a"})
	task, err := svc.CreateGenerateTask(ctx, &GenerateRequest{Marketplace: "amazon", ProductURL: "https://example.com/product"})
	if err != nil {
		t.Fatalf("CreateGenerateTask: %v", err)
	}
	envelope, err := task.ExecutionEnvelope()
	if err != nil {
		t.Fatalf("ExecutionEnvelope: %v", err)
	}
	if envelope.Version != aiidentity.CurrentEnvelopeVersion || envelope.TenantID != "tenant-a" || envelope.UserID != "user-a" || envelope.BusinessTaskID != task.ID || envelope.SourcePlatform != "amazon" || envelope.SourceTaskType != "listing" {
		t.Fatalf("envelope = %+v", envelope)
	}
}
