package amazonlisting

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/shared/aiidentity"
)

type spyTaskSubmitter struct {
	submitted []string
}

func (s *spyTaskSubmitter) Submit(taskID string) error {
	s.submitted = append(s.submitted, taskID)
	return nil
}

func TestCreateGenerateTaskCapturesExecutionEnvelope(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{Repository: repo})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a", TraceID: "trace-a"})
	task, err := svc.CreateGenerateTask(ctx, &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"})
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
				Repository:    repo,
				TaskSubmitter: submitter,
			})
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}

			ctx := aiidentity.WithIdentity(context.Background(), tc.identity)
			_, err = svc.CreateGenerateTask(ctx, &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"})
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

func TestCreateGenerateTaskRejectsMissingIdentity(t *testing.T) {
	repo := &stubRepository{}
	submitter := &spyTaskSubmitter{}
	svc, err := NewService(&ServiceConfig{
		Repository:    repo,
		TaskSubmitter: submitter,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.CreateGenerateTask(context.Background(), &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"})
	if !errors.Is(err, aiidentity.ErrMissingIdentity) {
		t.Fatalf("CreateGenerateTask() error = %v, want ErrMissingIdentity", err)
	}
	if repo.task != nil || len(submitter.submitted) != 0 {
		t.Fatalf("missing-identity task persisted or submitted: task=%+v submissions=%v", repo.task, submitter.submitted)
	}
}
