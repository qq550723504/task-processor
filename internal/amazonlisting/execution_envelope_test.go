package amazonlisting

import (
	"context"
	"errors"
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

func TestCreateGenerateTaskRejectsPartialIdentityBeforePersistenceOrSubmission(t *testing.T) {
	cases := []struct {
		name     string
		identity aiidentity.Identity
	}{
		{name: "tenant only", identity: aiidentity.Identity{TenantID: "tenant-a"}},
		{name: "user only", identity: aiidentity.Identity{UserID: "user-a"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubRepository{}
			submitter := &spyTaskSubmitter{}
			svc, err := NewService(&ServiceConfig{
				Repository:     repo,
				ProductService: &stubProductService{},
				TaskSubmitter:  submitter,
			})
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}

			ctx := aiidentity.WithIdentity(context.Background(), tc.identity)
			_, err = svc.CreateGenerateTask(ctx, &GenerateRequest{Marketplace: "amazon", ProductURL: "https://example.com/product"})
			if !errors.Is(err, aiidentity.ErrIdentityIntegrity) {
				t.Fatalf("CreateGenerateTask() error = %v, want ErrIdentityIntegrity", err)
			}
			if repo.task != nil {
				t.Fatalf("repository persisted partial-identity task %+v", repo.task)
			}
			if len(submitter.submitted) != 0 {
				t.Fatalf("submitted task IDs = %v, want none", submitter.submitted)
			}
		})
	}
}

func TestCreateGenerateTaskPreservesAnonymousLegacySubmission(t *testing.T) {
	repo := &stubRepository{}
	submitter := &spyTaskSubmitter{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		TaskSubmitter:  submitter,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{Marketplace: "amazon", ProductURL: "https://example.com/product"})
	if err != nil {
		t.Fatalf("CreateGenerateTask: %v", err)
	}
	if repo.task == nil || repo.task.ID != task.ID {
		t.Fatalf("persisted task = %+v, want task %q", repo.task, task.ID)
	}
	if len(submitter.submitted) != 1 || submitter.submitted[0] != task.ID {
		t.Fatalf("submitted task IDs = %v, want [%s]", submitter.submitted, task.ID)
	}
	envelope, err := task.ExecutionEnvelope()
	if err != nil {
		t.Fatalf("ExecutionEnvelope: %v", err)
	}
	if envelope != (aiidentity.ExecutionEnvelope{}) {
		t.Fatalf("legacy envelope = %+v, want empty", envelope)
	}
}
