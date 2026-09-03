package listingkit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"task-processor/internal/listingkit/core"
)

type duplicateRejectingRepo struct {
	*stubSubmitRepo
	mu            sync.Mutex
	tasks         map[string]*Task
	submitTracker *countingTaskSubmitter
}

func newDuplicateRejectingRepo(submitTracker *countingTaskSubmitter) *duplicateRejectingRepo {
	return &duplicateRejectingRepo{stubSubmitRepo: &stubSubmitRepo{}, tasks: map[string]*Task{}, submitTracker: submitTracker}
}

func (r *duplicateRejectingRepo) CreateTask(ctx context.Context, task *Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tasks[task.ID]; exists {
		return fmt.Errorf("task %s already exists", task.ID)
	}
	copied := *task
	r.tasks[task.ID] = &copied
	return nil
}

func (r *duplicateRejectingRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, core.ErrTaskNotFound
	}
	copied := *task
	return &copied, nil
}

func (r *duplicateRejectingRepo) submittedTaskIDs() []string {
	return r.submitTracker.submitted()
}

type countingTaskSubmitter struct {
	mu   sync.Mutex
	ids  []string
	fail bool
}

func (s *countingTaskSubmitter) Submit(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("submitter unavailable")
	}
	s.ids = append(s.ids, taskID)
	return nil
}

func (s *countingTaskSubmitter) submitted() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ids...)
}

func TestCreateGenerateTaskReplaysSourceIdempotencyKeyWithoutDuplicateDispatch(t *testing.T) {
	submitter := &countingTaskSubmitter{}
	repo := newDuplicateRejectingRepo(submitter)
	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo:          repo,
		taskSubmitter: func() TaskSubmitter { return submitter },
	})
	ctx := WithTenantID(context.Background(), "101")

	first, err := lifecycle.CreateGenerateTask(ctx, &GenerateRequest{
		TenantID: "101", ProductKey: "crawler:1688:777", Platforms: []string{"amazon"},
		IdempotencyKey: "source-run:run-1",
	})
	if err != nil {
		t.Fatalf("first CreateGenerateTask() error = %v", err)
	}
	if first == nil || first.ID == "" {
		t.Fatalf("first CreateGenerateTask() task = %+v, want created task", first)
	}

	second, err := lifecycle.CreateGenerateTask(ctx, &GenerateRequest{
		TenantID: "101", ProductKey: "crawler:1688:777", Platforms: []string{"amazon"},
		IdempotencyKey: "source-run:run-1",
	})
	if err != nil {
		t.Fatalf("replayed CreateGenerateTask() error = %v, want idempotent replay", err)
	}
	if second == nil || second.ID != first.ID {
		t.Fatalf("replayed task ID = %v, want %v", secondTaskID(second), first.ID)
	}
	if submitted := repo.submittedTaskIDs(); len(submitted) != 1 {
		t.Fatalf("submitted task IDs = %v, want exactly one dispatch", submitted)
	}
}

func TestCreateGenerateTaskRejectsSameIdempotencyKeyWithDifferentPayload(t *testing.T) {
	submitter := &countingTaskSubmitter{}
	repo := newDuplicateRejectingRepo(submitter)
	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo:          repo,
		taskSubmitter: func() TaskSubmitter { return submitter },
	})
	ctx := WithTenantID(context.Background(), "101")

	if _, err := lifecycle.CreateGenerateTask(ctx, &GenerateRequest{
		TenantID: "101", ProductKey: "crawler:1688:777", Platforms: []string{"amazon"},
		IdempotencyKey: "source-run:run-1",
	}); err != nil {
		t.Fatalf("first CreateGenerateTask() error = %v", err)
	}

	_, err := lifecycle.CreateGenerateTask(ctx, &GenerateRequest{
		TenantID: "101", ProductKey: "crawler:1688:999", Platforms: []string{"amazon"},
		IdempotencyKey: "source-run:run-1",
	})
	if !errors.Is(err, ErrGenerateTaskIdempotencyConflict) {
		t.Fatalf("conflicting CreateGenerateTask() error = %v, want ErrGenerateTaskIdempotencyConflict", err)
	}
	if submitted := repo.submittedTaskIDs(); len(submitted) != 1 {
		t.Fatalf("submitted task IDs = %v, want no dispatch for the conflicting replay", submitted)
	}
}

func TestCreateGenerateTaskWithoutIdempotencyKeyKeepsUniqueTaskIDs(t *testing.T) {
	submitter := &countingTaskSubmitter{}
	repo := newDuplicateRejectingRepo(submitter)
	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo:          repo,
		taskSubmitter: func() TaskSubmitter { return submitter },
	})
	ctx := WithTenantID(context.Background(), "101")

	first, err := lifecycle.CreateGenerateTask(ctx, &GenerateRequest{
		TenantID: "101", ProductKey: "crawler:1688:777", Platforms: []string{"amazon"},
	})
	if err != nil {
		t.Fatalf("first CreateGenerateTask() error = %v", err)
	}
	second, err := lifecycle.CreateGenerateTask(ctx, &GenerateRequest{
		TenantID: "101", ProductKey: "crawler:1688:777", Platforms: []string{"amazon"},
	})
	if err != nil {
		t.Fatalf("second CreateGenerateTask() error = %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("task IDs both = %s, want unique IDs without an idempotency key", first.ID)
	}
}

func TestCreateGenerateTaskDerivesStableTaskIDFromIdempotencyKey(t *testing.T) {
	var firstID string
	for attempt := 0; attempt < 2; attempt++ {
		submitter := &countingTaskSubmitter{fail: true}
		repo := newDuplicateRejectingRepo(submitter)
		lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
			repo:          repo,
			taskSubmitter: func() TaskSubmitter { return submitter },
		})
		ctx := WithTenantID(context.Background(), "101")
		task, err := lifecycle.CreateGenerateTask(ctx, &GenerateRequest{
			TenantID: "101", ProductKey: "crawler:1688:777", Platforms: []string{"amazon"},
			IdempotencyKey: "source-run:run-1",
		})
		if err == nil || task == nil {
			t.Fatalf("CreateGenerateTask() with failed dispatch = task %v err %v, want created task with dispatch error", task, err)
		}
		if task.ID == "" {
			t.Fatalf("task ID empty, want deterministic id")
		}
		if attempt == 0 {
			firstID = task.ID
			continue
		}
		if task.ID != firstID {
			t.Fatalf("task IDs differ across replays: %s vs %s, want stable deterministic id", task.ID, firstID)
		}
	}
}

func TestCreateGenerateTaskRejectsSameIdempotencyKeyWithDifferentTargetPayload(t *testing.T) {
	baseRequest := func() *GenerateRequest {
		return &GenerateRequest{
			TenantID: "101", ProductKey: "crawler:1688:777", Platforms: []string{"amazon"},
			IdempotencyKey: "source-run:run-1",
			UserID:         "user-1", BillingTenantID: "101",
			Country: "us", Language: "en",
			Source:             &SourceReference{Type: "a1688", ID: "777"},
			TargetCategoryHint: "bags", BrandHint: "ListingKit",
			Options: &GenerateOptions{SDS: &SDSSyncOptions{VariantID: 42, ProductName: "Tote"}},
		}
	}

	tests := []struct {
		name   string
		mutate func(*GenerateRequest)
	}{
		{name: "different shein store", mutate: func(r *GenerateRequest) { r.SheinStoreID = 2002 }},
		{name: "different country", mutate: func(r *GenerateRequest) { r.Country = "de" }},
		{name: "different language", mutate: func(r *GenerateRequest) { r.Language = "de" }},
		{name: "different category hint", mutate: func(r *GenerateRequest) { r.TargetCategoryHint = "shoes" }},
		{name: "different brand hint", mutate: func(r *GenerateRequest) { r.BrandHint = "Other" }},
		{name: "different user", mutate: func(r *GenerateRequest) { r.UserID = "user-2" }},
		{name: "different billing tenant", mutate: func(r *GenerateRequest) { r.BillingTenantID = "202" }},
		{name: "different text", mutate: func(r *GenerateRequest) { r.Text = "regenerate with focus on durability" }},
		{name: "different source", mutate: func(r *GenerateRequest) { r.Source = &SourceReference{Type: "a1688", ID: "999"} }},
		{name: "missing source", mutate: func(r *GenerateRequest) { r.Source = nil }},
		{name: "different options", mutate: func(r *GenerateRequest) { r.Options = &GenerateOptions{SDS: &SDSSyncOptions{VariantID: 43}} }},
		{name: "missing options payload", mutate: func(r *GenerateRequest) { r.Options = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			submitter := &countingTaskSubmitter{}
			repo := newDuplicateRejectingRepo(submitter)
			lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
				repo:          repo,
				taskSubmitter: func() TaskSubmitter { return submitter },
			})
			ctx := WithTenantID(context.Background(), "101")

			if _, err := lifecycle.CreateGenerateTask(ctx, baseRequest()); err != nil {
				t.Fatalf("first CreateGenerateTask() error = %v", err)
			}
			replayed := baseRequest()
			tt.mutate(replayed)
			_, err := lifecycle.CreateGenerateTask(ctx, replayed)
			if !errors.Is(err, ErrGenerateTaskIdempotencyConflict) {
				t.Fatalf("conflicting CreateGenerateTask() error = %v, want ErrGenerateTaskIdempotencyConflict", err)
			}
			if submitted := repo.submittedTaskIDs(); len(submitted) != 1 {
				t.Fatalf("submitted task IDs = %v, want no dispatch for the conflicting replay", submitted)
			}
		})
	}
}

func TestCreateGenerateTaskReplaysSameIdempotencyKeyWithIdenticalTargetPayload(t *testing.T) {
	submitter := &countingTaskSubmitter{}
	repo := newDuplicateRejectingRepo(submitter)
	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo:          repo,
		taskSubmitter: func() TaskSubmitter { return submitter },
	})
	ctx := WithTenantID(context.Background(), "101")
	request := &GenerateRequest{
		TenantID: "101", ProductKey: "crawler:1688:777", Platforms: []string{"amazon"},
		IdempotencyKey: "source-run:run-1",
		UserID:         "user-1", BillingTenantID: "101",
		Country: "us", Language: "en",
		Source:             &SourceReference{Type: "a1688", ID: "777"},
		TargetCategoryHint: "bags", BrandHint: "ListingKit",
		Options: &GenerateOptions{SDS: &SDSSyncOptions{VariantID: 42, ProductName: "Tote"}},
	}

	first, err := lifecycle.CreateGenerateTask(ctx, request)
	if err != nil {
		t.Fatalf("first CreateGenerateTask() error = %v", err)
	}
	second, err := lifecycle.CreateGenerateTask(ctx, request)
	if err != nil {
		t.Fatalf("replayed CreateGenerateTask() error = %v, want idempotent replay", err)
	}
	if second == nil || second.ID != first.ID {
		t.Fatalf("replayed task ID = %v, want %v", secondTaskID(second), first.ID)
	}
	if submitted := repo.submittedTaskIDs(); len(submitted) != 1 {
		t.Fatalf("submitted task IDs = %v, want exactly one dispatch", submitted)
	}
}

func secondTaskID(task *Task) string {
	if task == nil {
		return ""
	}
	return task.ID
}
