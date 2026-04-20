package productimage

import (
	"context"
	"errors"
	"testing"
	"time"
)

type contextAwareTaskRepo struct {
	task *Task
}

func (r *contextAwareTaskRepo) CreateTask(_ context.Context, task *Task) error {
	cloned := *task
	r.task = &cloned
	return nil
}

func (r *contextAwareTaskRepo) GetTask(_ context.Context, taskID string) (*Task, error) {
	if r.task == nil || r.task.ID != taskID {
		return nil, ErrTaskNotFound
	}
	cloned := *r.task
	return &cloned, nil
}

func (r *contextAwareTaskRepo) MarkProcessing(_ context.Context, taskID string) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	if r.task.Status != TaskStatusPending {
		return ErrTaskNotPending
	}
	r.task.Status = TaskStatusProcessing
	r.task.Error = ""
	r.task.UpdatedAt = time.Now()
	return nil
}

func (r *contextAwareTaskRepo) MarkCompleted(ctx context.Context, taskID string, result *ImageProcessResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	return r.UpdateTaskStatus(ctx, taskID, TaskStatusCompleted)
}

func (r *contextAwareTaskRepo) MarkNeedsReview(ctx context.Context, taskID string, result *ImageProcessResult, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	if err := r.UpdateTaskStatus(ctx, taskID, TaskStatusNeedsReview); err != nil {
		return err
	}
	return r.UpdateTaskErrorMessage(ctx, taskID, reason)
}

func (r *contextAwareTaskRepo) MarkRejected(ctx context.Context, taskID string, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.UpdateTaskStatus(ctx, taskID, TaskStatusRejected); err != nil {
		return err
	}
	return r.UpdateTaskErrorMessage(ctx, taskID, reason)
}

func (r *contextAwareTaskRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.UpdateTaskError(ctx, taskID, errorMsg)
}

func (r *contextAwareTaskRepo) PrepareRetry(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.ResetForRetry(ctx, taskID)
}

func (r *contextAwareTaskRepo) UpdateTaskStatus(_ context.Context, taskID string, status TaskStatus) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Status = status
	r.task.UpdatedAt = time.Now()
	return nil
}

func (r *contextAwareTaskRepo) UpdateTaskError(_ context.Context, taskID string, errorMsg string) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Status = TaskStatusFailed
	r.task.Error = errorMsg
	r.task.UpdatedAt = time.Now()
	return nil
}

func (r *contextAwareTaskRepo) UpdateTaskErrorMessage(_ context.Context, taskID string, errorMsg string) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Error = errorMsg
	r.task.UpdatedAt = time.Now()
	return nil
}

func (r *contextAwareTaskRepo) SaveTaskResult(_ context.Context, taskID string, result *ImageProcessResult) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Result = result
	r.task.UpdatedAt = time.Now()
	return nil
}

func (r *contextAwareTaskRepo) IncrementRetryCount(_ context.Context, taskID string) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.RetryCount++
	r.task.UpdatedAt = time.Now()
	return nil
}

func (r *contextAwareTaskRepo) ResetForRetry(_ context.Context, taskID string) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Status = TaskStatusPending
	r.task.Error = ""
	r.task.UpdatedAt = time.Now()
	return nil
}

type cancelingSubjectExtractor struct {
	cancel func()
	err    error
}

func (e *cancelingSubjectExtractor) Extract(_ context.Context, _ string, _ *ProductContext) (*ImageAsset, error) {
	if e.cancel != nil {
		e.cancel()
	}
	return nil, e.err
}

type staticSourceParser struct {
	source *SourceBundle
}

func (p *staticSourceParser) Parse(_ context.Context, _ *ImageProcessRequest) (*SourceBundle, error) {
	return p.source, nil
}

type staticContextAnalyzer struct {
	productContext *ProductContext
}

func (a *staticContextAnalyzer) Analyze(_ context.Context, _ *SourceBundle) (*ProductContext, error) {
	return a.productContext, nil
}

func TestServiceProcessMarksTaskFailedWhenRequestContextCanceled(t *testing.T) {
	repo := &contextAwareTaskRepo{}
	cancelErr := errors.New("subject extraction boom")
	ctx, cancel := context.WithCancel(context.Background())

	svcIface, err := NewService(&ServiceConfig{
		TaskRepo:         repo,
		SourceParser:     &staticSourceParser{source: &SourceBundle{Images: []string{"https://example.com/a.jpg"}}},
		ContextAnalyzer:  &staticContextAnalyzer{productContext: &ProductContext{ProductType: "shoes"}},
		SubjectExtractor: &cancelingSubjectExtractor{cancel: cancel, err: cancelErr},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	svc := svcIface.(*service)

	task, err := svc.CreateProcessTask(context.Background(), &ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/a.jpg"},
		Marketplace: "shein",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() error = %v", err)
	}

	_, err = svc.ProcessImages(ctx, task)
	if err == nil || !errors.Is(err, cancelErr) {
		t.Fatalf("ProcessImages() error = %v, want wrapped subject failure", err)
	}

	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != TaskStatusFailed {
		t.Fatalf("stored status = %q, want failed", stored.Status)
	}
	if stored.Error == "" {
		t.Fatal("expected stored error message")
	}
}
