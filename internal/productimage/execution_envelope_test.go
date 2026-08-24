package productimage

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/shared/aiidentity"
)

type envelopeTaskSubmitter struct {
	submitted []string
}

func (s *envelopeTaskSubmitter) Submit(taskID string) error {
	s.submitted = append(s.submitted, taskID)
	return nil
}

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

func TestCreateProcessTaskRejectsPartialIdentityBeforePersistenceOrSubmission(t *testing.T) {
	cases := []struct {
		name     string
		identity AIIdentity
	}{
		{name: "tenant only", identity: AIIdentity{TenantID: "tenant-a"}},
		{name: "user only", identity: AIIdentity{UserID: "user-a"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &contextAwareTaskRepo{}
			submitter := &envelopeTaskSubmitter{}
			svc := &service{taskRepo: repo, taskSubmitter: submitter}

			ctx := WithAIIdentity(context.Background(), tc.identity)
			_, err := svc.CreateProcessTask(ctx, &ImageProcessRequest{ProductURL: "https://example.test/product", Marketplace: "shein"})
			if !errors.Is(err, aiidentity.ErrIdentityIntegrity) {
				t.Fatalf("CreateProcessTask() error = %v, want ErrIdentityIntegrity", err)
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

func TestCreateProcessTaskPreservesAnonymousLegacySubmission(t *testing.T) {
	repo := &contextAwareTaskRepo{}
	submitter := &envelopeTaskSubmitter{}
	svc := &service{taskRepo: repo, taskSubmitter: submitter}

	task, err := svc.CreateProcessTask(context.Background(), &ImageProcessRequest{ProductURL: "https://example.test/product", Marketplace: "shein"})
	if err != nil {
		t.Fatalf("CreateProcessTask: %v", err)
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
