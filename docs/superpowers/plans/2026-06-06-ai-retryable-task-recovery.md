# AI Retryable Task Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn retryable upstream dependency failures such as OpenAI credit exhaustion, rate limits, and transient upstream outages into durable recoverable ListingKit task states that can auto-resume or be manually recovered without recreating tasks.

**Architecture:** Add an explicit `blocked_retryable` task status plus durable retry metadata on the task record, then route retryable failures through a centralized classifier instead of terminal `failed` or business `needs_review`. Build a small recovery service that can requeue blocked tasks with bounded backoff, expose single-task and bulk recovery APIs, and teach the frontend to render “等待依赖恢复” with retry metadata and recovery actions.

**Tech Stack:** Go, GORM, Gin, React, Next.js, TypeScript, TanStack Query, Vitest, Testing Library

---

## File Structure

- Modify: `internal/listingkit/model.go`
  - Add `TaskStatusBlockedRetryable` and shared retryable error sentinels.
- Modify: `internal/listingkit/model_task.go`
  - Persist retryable block metadata on `Task`, expose it on task result/list payloads, and extend list filters.
- Modify: `internal/listingkit/model_result.go`
  - Carry retryable block metadata into `ListingKitResult` read payloads.
- Create: `internal/listingkit/retryable_block.go`
  - Define `RetryableBlock`, reason codes, backoff calculation, and helper constructors.
- Create: `internal/listingkit/retryable_classifier.go`
  - Centralize classification of OpenAI insufficient credits, 429, transient network, and queue backpressure failures.
- Modify: `internal/listingkit/interfaces.go`
  - Extend `Repository`, `TaskLifecycleService`, and `Service` with retryable recovery operations.
- Modify: `internal/listingkit/task_lifecycle_service.go`
  - Replace direct `MarkFailed(...)` on retryable submit/runtime failures with `MarkBlockedRetryable(...)` and recovery scheduling.
- Modify: `internal/listingkit/service_process_flow.go`
  - Route workflow errors through classifier-aware persistence.
- Modify: `internal/listingkit/task_result_support.go`
  - Include retryable block payload in task detail responses and completion semantics.
- Modify: `internal/listingkit/task_state_support.go`
  - Derive queue/action status for `blocked_retryable` distinctly from `failed`.
- Modify: `internal/listingkit/preview_builder.go`
  - Surface retryable-blocked state in preview/status summaries.
- Modify: `internal/listingkit/store/task_repo.go`
  - Persist retryable block fields, add queries for eligible blocked tasks, and support bulk recovery operations.
- Modify: `internal/listingkit/store/mem_store.go`
  - Keep in-memory behavior aligned with DB repository behavior.
- Create: `internal/listingkit/task_recovery_service.go`
  - Implement auto-recovery runner, single-task recover-now flow, and bulk recovery flow.
- Modify: `internal/listingkit/httpapi/builders.go`
  - Auto-migrate new `Task` fields and wire the recovery service into runtime construction.
- Modify: `internal/listingkit/api/handler.go`
  - Register new task recovery handler dependencies.
- Create: `internal/listingkit/api/task_recovery_handler.go`
  - Add `POST /tasks/:task_id/recover` and `POST /tasks/recover`.
- Modify: `internal/listingkit/api/*_test.go`
  - Add handler coverage for recover-now and bulk recover.
- Modify: `web/listingkit-ui/src/lib/types/listingkit/tasks.ts`
  - Add `retryable_block` payload types and list query fields.
- Modify: `web/listingkit-ui/src/lib/api/listingkit-response-schema.ts`
  - Parse `retryable_block` fields from task detail payloads.
- Modify: `web/listingkit-ui/src/lib/api/task-list-schema.ts`
  - Parse blocked counts and new query/filter fields.
- Create: `web/listingkit-ui/src/lib/api/task-recovery.ts`
  - Call single-task and bulk recovery endpoints.
- Create: `web/listingkit-ui/src/lib/query/use-task-recovery.ts`
  - Mutations for recover-now and bulk recover.
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-query.ts`
  - Keep polling `blocked_retryable` tasks while auto-recovery is active.
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-panel.tsx`
  - Render blocked-retryable state, next retry time, attempt count, and action buttons.
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.tsx`
  - Thread recover-now action through the detail screen.
- Modify: `web/listingkit-ui/src/components/listingkit/queue/queue-screen-sections.tsx`
  - Add bulk recovery entry point and blocked filters in queue/list UI.
- Create: `web/listingkit-ui/src/lib/api/task-recovery.test.ts`
  - API client coverage for new recovery endpoints.
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-panel.test.tsx`
  - UI coverage for blocked-retryable rendering and retry CTA.
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.test.tsx`
  - Detail-page recover-now flow coverage.
- Modify: `web/listingkit-ui/src/lib/query/use-task-result.ts`
  - Preserve polling semantics for blocked tasks.
- Modify: `cmd/listingkit-schema-migrate/main.go`
  - No behavior change expected, but this command is the documented migration entrypoint for rollout verification.

## Task 1: Add Domain Types and Red Tests for Retryable Blocking

**Files:**
- Create: `internal/listingkit/retryable_block.go`
- Modify: `internal/listingkit/model.go`
- Modify: `internal/listingkit/model_task.go`
- Create: `internal/listingkit/retryable_classifier_test.go`
- Create: `internal/listingkit/task_recovery_model_test.go`

- [ ] **Step 1: Write the failing domain tests**

```go
func TestClassifyRetryableTaskFailure_OpenAIInsufficientCredits(t *testing.T) {
	info := classifyRetryableTaskFailure(context.Background(), fmt.Errorf("openai: insufficient credits"))
	if info == nil {
		t.Fatal("classifyRetryableTaskFailure() = nil, want retryable block")
	}
	if info.ReasonCode != RetryableReasonOpenAIInsufficientCredits {
		t.Fatalf("reason code = %q, want %q", info.ReasonCode, RetryableReasonOpenAIInsufficientCredits)
	}
	if !info.AutoResumeEnabled {
		t.Fatal("auto resume = false, want true")
	}
}

func TestTaskStatusBlockedRetryable_IsNotTerminal(t *testing.T) {
	if taskStatusIsTerminal(TaskStatusBlockedRetryable) {
		t.Fatal("blocked_retryable should not be terminal")
	}
}

func TestTaskResultLifecycleCarriesRetryableBlock(t *testing.T) {
	now := time.Date(2026, 6, 6, 14, 0, 0, 0, time.UTC)
	nextRetryAt := now.Add(5 * time.Minute)
	result := buildTaskResult(&Task{
		ID:        "task-1",
		TenantID:  "tenant-1",
		Status:    TaskStatusBlockedRetryable,
		CreatedAt: now,
		RetryableBlock: &RetryableBlock{
			ReasonCode:        RetryableReasonOpenAIInsufficientCredits,
			ReasonMessage:     "OpenAI credits exhausted",
			BlockedAt:         now,
			NextRetryAt:       &nextRetryAt,
			RetryAttempts:     2,
			AutoResumeEnabled: true,
		},
	}, nil)
	if result.RetryableBlock == nil {
		t.Fatal("retryable block = nil, want payload")
	}
	if result.CompletedAt != nil {
		t.Fatal("completed_at != nil, blocked retryable should be non-terminal")
	}
}
```

- [ ] **Step 2: Run the domain tests to verify they fail**

Run: `go test ./internal/listingkit -run "TestClassifyRetryableTaskFailure_OpenAIInsufficientCredits|TestTaskStatusBlockedRetryable_IsNotTerminal|TestTaskResultLifecycleCarriesRetryableBlock" -count=1`

Expected: FAIL because `RetryableBlock`, `TaskStatusBlockedRetryable`, and classifier helpers do not exist yet.

- [ ] **Step 3: Add the minimal retryable block types and task fields**

```go
const (
	TaskStatusPending          TaskStatus = "pending"
	TaskStatusProcessing       TaskStatus = "processing"
	TaskStatusBlockedRetryable TaskStatus = "blocked_retryable"
	TaskStatusCompleted        TaskStatus = "completed"
	TaskStatusNeedsReview      TaskStatus = "needs_review"
	TaskStatusFailed           TaskStatus = "failed"
)

type RetryableBlock struct {
	ReasonCode           string     `json:"reason_code,omitempty"`
	ReasonMessage        string     `json:"reason_message,omitempty"`
	BlockedAt            time.Time  `json:"blocked_at,omitempty"`
	LastRetryAt          *time.Time `json:"last_retry_at,omitempty"`
	NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
	RetryAttempts        int        `json:"retry_attempts,omitempty"`
	MaxAutoRetryAttempts int        `json:"max_auto_retry_attempts,omitempty"`
	RecoveryScope        string     `json:"recovery_scope,omitempty"`
	AutoResumeEnabled    bool       `json:"auto_resume_enabled,omitempty"`
	AutoRetryPaused      bool       `json:"auto_retry_paused,omitempty"`
}

type Task struct {
	ID             string          `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID       string          `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	Status         TaskStatus      `json:"status" gorm:"type:varchar(32);index"`
	Result         *ListingKitResult `json:"result,omitempty" gorm:"type:text"`
	RetryableBlock *RetryableBlock `json:"retryable_block,omitempty" gorm:"type:text"`
	Error          string          `json:"error,omitempty" gorm:"type:text"`
}

type TaskResult struct {
	TaskIdentityFields
	TaskResultLifecycleFields
	SheinSubmissionStatusFields
	Result          *ListingKitResult `json:"result,omitempty"`
	RetryableBlock  *RetryableBlock   `json:"retryable_block,omitempty"`
	ReviewReasons   []string          `json:"review_reasons,omitempty"`
}
```

- [ ] **Step 4: Add the first-pass classifier**

```go
const (
	RetryableReasonOpenAIInsufficientCredits = "openai_insufficient_credits"
	RetryableReasonOpenAIRateLimited         = "openai_rate_limited"
	RetryableReasonUpstreamTimeout           = "upstream_timeout"
	RetryableReasonUpstreamNetworkError      = "upstream_network_error"
	RetryableReasonWorkerQueueBackpressure   = "worker_queue_backpressure"
)

func classifyRetryableTaskFailure(_ context.Context, err error) *RetryableBlock {
	if err == nil {
		return nil
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "insufficient credits"):
		return newRetryableBlock(RetryableReasonOpenAIInsufficientCredits, err.Error(), "full_task")
	case strings.Contains(message, "rate limit"), strings.Contains(message, "429"):
		return newRetryableBlock(RetryableReasonOpenAIRateLimited, err.Error(), "full_task")
	case strings.Contains(message, "queue full"), strings.Contains(message, "工作队列已满"):
		return newRetryableBlock(RetryableReasonWorkerQueueBackpressure, err.Error(), "full_task")
	case strings.Contains(message, "timeout"):
		return newRetryableBlock(RetryableReasonUpstreamTimeout, err.Error(), "full_task")
	case strings.Contains(message, "connection reset"), strings.Contains(message, "temporarily unavailable"):
		return newRetryableBlock(RetryableReasonUpstreamNetworkError, err.Error(), "full_task")
	default:
		return nil
	}
}
```

- [ ] **Step 5: Re-run the domain tests**

Run: `go test ./internal/listingkit -run "TestClassifyRetryableTaskFailure_OpenAIInsufficientCredits|TestTaskStatusBlockedRetryable_IsNotTerminal|TestTaskResultLifecycleCarriesRetryableBlock" -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/model.go internal/listingkit/model_task.go internal/listingkit/retryable_block.go internal/listingkit/retryable_classifier_test.go internal/listingkit/task_recovery_model_test.go
git commit -m "feat: add retryable task block domain model"
```

## Task 2: Persist and Query Blocked-Retryable Tasks in Both Repositories

**Files:**
- Modify: `internal/listingkit/interfaces.go`
- Modify: `internal/listingkit/store/task_repo.go`
- Modify: `internal/listingkit/store/mem_store.go`
- Create: `internal/listingkit/store/task_repo_retryable_test.go`

- [ ] **Step 1: Write repository tests for block persistence and eligible-task listing**

```go
func openRetryableRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func TestTaskRepositoryMarkBlockedRetryablePersistsMetadata(t *testing.T) {
	repo := NewTaskRepository(openRetryableRepoDB(t))
	ctx := listingkit.WithTenantID(context.Background(), "tenant-1")
	task := &listingkit.Task{
		ID:        "task-blocked",
		TenantID:  "tenant-1",
		Status:    listingkit.TaskStatusPending,
		Request:   &listingkit.GenerateRequest{TenantID: "tenant-1", Platforms: []string{"shein"}, Text: "drawer knob"},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	nextRetryAt := time.Now().UTC().Add(time.Minute)
	block := &listingkit.RetryableBlock{
		ReasonCode:           listingkit.RetryableReasonOpenAIInsufficientCredits,
		ReasonMessage:        "OpenAI credits exhausted",
		BlockedAt:            time.Now().UTC(),
		NextRetryAt:          &nextRetryAt,
		MaxAutoRetryAttempts: 8,
		AutoResumeEnabled:    true,
	}
	if err := repo.MarkBlockedRetryable(ctx, task.ID, nil, block, block.ReasonMessage); err != nil {
		t.Fatalf("MarkBlockedRetryable() error = %v", err)
	}
	stored, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != listingkit.TaskStatusBlockedRetryable {
		t.Fatalf("status = %q, want %q", stored.Status, listingkit.TaskStatusBlockedRetryable)
	}
	if stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != listingkit.RetryableReasonOpenAIInsufficientCredits {
		t.Fatalf("retryable block = %+v, want persisted reason", stored.RetryableBlock)
	}
}

func TestTaskRepositoryListRecoverableTasksReturnsDueItemsOnly(t *testing.T) {
	repo := NewTaskRepository(openRetryableRepoDB(t))
	ctx := listingkit.WithTenantID(context.Background(), "tenant-1")
	now := time.Now().UTC()
	dueRetryAt := now.Add(-time.Minute)
	laterRetryAt := now.Add(10 * time.Minute)
	tasks := []*listingkit.Task{
		{
			ID:        "task-due",
			TenantID:  "tenant-1",
			Status:    listingkit.TaskStatusPending,
			Request:   &listingkit.GenerateRequest{TenantID: "tenant-1", Platforms: []string{"shein"}, Text: "drawer knob"},
			CreatedAt: now.Add(-2 * time.Minute),
			UpdatedAt: now.Add(-2 * time.Minute),
			RetryableBlock: &listingkit.RetryableBlock{
				ReasonCode:           listingkit.RetryableReasonOpenAIInsufficientCredits,
				ReasonMessage:        "OpenAI credits exhausted",
				BlockedAt:            now.Add(-2 * time.Minute),
				NextRetryAt:          &dueRetryAt,
				MaxAutoRetryAttempts: 8,
				AutoResumeEnabled:    true,
			},
		},
		{
			ID:        "task-later",
			TenantID:  "tenant-1",
			Status:    listingkit.TaskStatusPending,
			Request:   &listingkit.GenerateRequest{TenantID: "tenant-1", Platforms: []string{"shein"}, Text: "drawer knob"},
			CreatedAt: now.Add(-2 * time.Minute),
			UpdatedAt: now.Add(-2 * time.Minute),
			RetryableBlock: &listingkit.RetryableBlock{
				ReasonCode:           listingkit.RetryableReasonOpenAIInsufficientCredits,
				ReasonMessage:        "OpenAI credits exhausted",
				BlockedAt:            now.Add(-2 * time.Minute),
				NextRetryAt:          &laterRetryAt,
				MaxAutoRetryAttempts: 8,
				AutoResumeEnabled:    true,
			},
		},
	}
	for _, task := range tasks {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
		}
		if err := repo.MarkBlockedRetryable(ctx, task.ID, nil, task.RetryableBlock, task.RetryableBlock.ReasonMessage); err != nil {
			t.Fatalf("MarkBlockedRetryable(%s) error = %v", task.ID, err)
		}
	}
	items, err := repo.ListRecoverableTasks(ctx, listingkit.RecoverableTaskQuery{DueBefore: now, Limit: 10})
	if err != nil {
		t.Fatalf("ListRecoverableTasks() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != "task-due" {
		t.Fatalf("items = %+v, want only task-due", items)
	}
}
```

- [ ] **Step 2: Run repository tests to verify they fail**

Run: `go test ./internal/listingkit/store -run "TestTaskRepositoryMarkBlockedRetryablePersistsMetadata|TestTaskRepositoryListRecoverableTasksReturnsDueItemsOnly" -count=1`

Expected: FAIL because the repository interface and implementations do not yet expose blocked-retryable operations.

- [ ] **Step 3: Extend repository interfaces and implementations**

```go
type RecoverableTaskQuery struct {
	TenantID    string
	ReasonCode  string
	DueBefore   time.Time
	Limit       int
}

type Repository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)
	ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error)
	MarkProcessing(ctx context.Context, taskID string) error
	MarkBlockedRetryable(ctx context.Context, taskID string, result *ListingKitResult, block *RetryableBlock, errorMsg string) error
	ListRecoverableTasks(ctx context.Context, query RecoverableTaskQuery) ([]Task, error)
	RecoverBlockedTaskNow(ctx context.Context, taskID string, nextRetryAt time.Time) error
	BulkRecoverBlockedTasks(ctx context.Context, query RecoverableTaskQuery, nextRetryAt time.Time) (int, error)
}
```

- [ ] **Step 4: Persist the new state in GORM and memory repositories**

```go
func (r *taskRepository) MarkBlockedRetryable(ctx context.Context, taskID string, result *listingkit.ListingKitResult, block *listingkit.RetryableBlock, errorMsg string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result":           result,
		"status":           listingkit.TaskStatusBlockedRetryable,
		"retryable_block":  block,
		"error":            errorMsg,
	})
}

func (r *MemTaskRepository) MarkBlockedRetryable(ctx context.Context, taskID string, result *listingkit.ListingKitResult, block *listingkit.RetryableBlock, errorMsg string) error {
	task, err := r.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored := r.tasks[task.ID]
	stored.Status = listingkit.TaskStatusBlockedRetryable
	stored.Result = result
	stored.RetryableBlock = cloneRetryableBlock(block)
	stored.Error = errorMsg
	stored.UpdatedAt = time.Now()
	return nil
}
```

- [ ] **Step 5: Re-run repository tests**

Run: `go test ./internal/listingkit/store -run "TestTaskRepositoryMarkBlockedRetryablePersistsMetadata|TestTaskRepositoryListRecoverableTasksReturnsDueItemsOnly" -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/interfaces.go internal/listingkit/store/task_repo.go internal/listingkit/store/mem_store.go internal/listingkit/store/task_repo_retryable_test.go
git commit -m "feat: persist retryable task block state"
```

## Task 3: Route Retryable Workflow Failures Into Blocked State

**Files:**
- Modify: `internal/listingkit/task_lifecycle_service.go`
- Modify: `internal/listingkit/service_process_flow.go`
- Modify: `internal/listingkit/task_result_support.go`
- Create: `internal/listingkit/task_lifecycle_retryable_test.go`

- [ ] **Step 1: Write failing lifecycle tests for submit and process failures**

```go
type submitterFunc func(string) error

func (f submitterFunc) Submit(taskID string) error { return f(taskID) }

func TestCreateGenerateTaskMarksQueueBackpressureAsBlockedRetryable(t *testing.T) {
	repo := store.NewMemTaskRepository()
	service := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskSubmitterFunc(func(string) error { return fmt.Errorf("failed to submit task: 工作队列已满") })
		},
	})
	task, err := service.CreateGenerateTask(context.Background(), &GenerateRequest{Platforms: []string{"shein"}, Text: "drawer knob"})
	if err != nil {
		t.Fatalf("CreateGenerateTask() error = %v", err)
	}
	if task.Status != TaskStatusBlockedRetryable {
		t.Fatalf("status = %q, want %q", task.Status, TaskStatusBlockedRetryable)
	}
	if task.RetryableBlock == nil || task.RetryableBlock.ReasonCode != RetryableReasonWorkerQueueBackpressure {
		t.Fatalf("retryable block = %+v, want queue backpressure", task.RetryableBlock)
	}
}

func TestProcessFlowMarksOpenAICreditFailureAsBlockedRetryable(t *testing.T) {
	repo := store.NewMemTaskRepository()
	service := &service{
		repo: repo,
	}
	service.runWorkflow = func(context.Context, *Task) (*ListingKitResult, error) {
		return &ListingKitResult{TaskID: "task-openai", Status: string(TaskStatusProcessing)}, fmt.Errorf("openai request failed: insufficient credits")
	}
	task := &Task{
		ID:        "task-openai",
		TenantID:  "tenant-1",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{TenantID: "tenant-1", Platforms: []string{"shein"}, Text: "drawer knob"},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.CreateTask(WithTenantID(context.Background(), "tenant-1"), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err := buildListingKitProcessFlow(service).run(context.Background(), task, logrus.NewEntry(logrus.New())); err == nil {
		t.Fatal("run() error = nil, want workflow error")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != TaskStatusBlockedRetryable {
		t.Fatalf("status = %q, want %q", stored.Status, TaskStatusBlockedRetryable)
	}
}
```

- [ ] **Step 2: Run lifecycle tests to verify they fail**

Run: `go test ./internal/listingkit -run "TestCreateGenerateTaskMarksQueueBackpressureAsBlockedRetryable|TestProcessFlowMarksOpenAICreditFailureAsBlockedRetryable" -count=1`

Expected: FAIL because retryable failures still become `failed` or remain `pending`.

- [ ] **Step 3: Route retryable failures through the classifier**

```go
func (s *taskLifecycleService) handleRetryableFailure(ctx context.Context, taskID string, result *ListingKitResult, err error) error {
	block := classifyRetryableTaskFailure(ctx, err)
	if block == nil {
		return s.repo.MarkFailed(ctx, taskID, err.Error())
	}
	now := time.Now().UTC()
	block.BlockedAt = now
	nextRetryAt := now.Add(nextRetryDelay(0))
	block.NextRetryAt = &nextRetryAt
	block.MaxAutoRetryAttempts = 8
	block.AutoResumeEnabled = true
	return s.repo.MarkBlockedRetryable(ctx, taskID, result, block, err.Error())
}

func (s *taskLifecycleService) enqueueGenerateTask(ctx context.Context, task *Task) error {
	if err := s.taskSubmitter().Submit(task.ID); err != nil {
		if block := classifyRetryableTaskFailure(ctx, err); block != nil {
			if markErr := s.repo.MarkBlockedRetryable(ctx, task.ID, task.Result, block, err.Error()); markErr != nil {
				return fmt.Errorf("failed to persist blocked retryable task: %w", markErr)
			}
			return nil
		}
		_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err))
		return fmt.Errorf("failed to submit task: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Keep blocked tasks non-terminal in result builders**

```go
func buildTaskResult(task *Task, resultPayload *ListingKitResult) *TaskResult {
	// existing fields omitted
	result := &TaskResult{
		TaskIdentityFields: TaskIdentityFields{TaskID: task.ID, TenantID: task.TenantID},
		TaskResultLifecycleFields: TaskResultLifecycleFields{Status: task.Status, Error: task.Error, CreatedAt: task.CreatedAt},
		Result:         resultPayload,
		RetryableBlock: cloneRetryableBlock(task.RetryableBlock),
		ReviewReasons:  reviewReasons,
	}
	if taskStatusIsTerminal(task.Status) {
		result.CompletedAt = &task.UpdatedAt
	}
	return result
}
```

- [ ] **Step 5: Re-run lifecycle tests**

Run: `go test ./internal/listingkit -run "TestCreateGenerateTaskMarksQueueBackpressureAsBlockedRetryable|TestProcessFlowMarksOpenAICreditFailureAsBlockedRetryable" -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_lifecycle_service.go internal/listingkit/service_process_flow.go internal/listingkit/task_result_support.go internal/listingkit/task_lifecycle_retryable_test.go
git commit -m "feat: classify retryable workflow failures"
```

## Task 4: Add Recover-Now and Background Auto-Recovery Service

**Files:**
- Create: `internal/listingkit/task_recovery_service.go`
- Modify: `internal/listingkit/interfaces.go`
- Modify: `internal/listingkit/httpapi/builders.go`
- Create: `internal/listingkit/task_recovery_service_test.go`

- [ ] **Step 1: Write failing service tests for manual and automatic recovery**

```go
type submitterFunc func(string) error

func (f submitterFunc) Submit(taskID string) error { return f(taskID) }

func TestRecoverTaskNowRequeuesBlockedTask(t *testing.T) {
	repo := store.NewMemTaskRepository()
	submitted := make([]string, 0, 1)
	svc := newTaskRecoveryService(repo, taskSubmitterFunc(func(taskID string) error {
		submitted = append(submitted, taskID)
		return nil
	}))
	now := time.Now().UTC()
	nextRetryAt := now.Add(time.Minute)
	task := &Task{
		ID:        "task-1",
		TenantID:  "tenant-1",
		Status:    TaskStatusBlockedRetryable,
		Request:   &GenerateRequest{TenantID: "tenant-1", Platforms: []string{"shein"}, Text: "drawer knob"},
		RetryableBlock: &RetryableBlock{
			ReasonCode:           RetryableReasonOpenAIInsufficientCredits,
			ReasonMessage:        "OpenAI credits exhausted",
			BlockedAt:            now,
			NextRetryAt:          &nextRetryAt,
			MaxAutoRetryAttempts: 8,
			AutoResumeEnabled:    true,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateTask(WithTenantID(context.Background(), "tenant-1"), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	result, err := svc.RecoverTaskNow(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("RecoverTaskNow() error = %v", err)
	}
	if result.Status != TaskStatusPending {
		t.Fatalf("status = %q, want %q", result.Status, TaskStatusPending)
	}
	if len(submitted) != 1 || submitted[0] != "task-1" {
		t.Fatalf("submitted = %+v, want task-1", submitted)
	}
}

func TestRunRecoverySweepRequeuesDueBlockedTasksOnly(t *testing.T) {
	repo := store.NewMemTaskRepository()
	submitted := []string{}
	svc := newTaskRecoveryService(repo, taskSubmitterFunc(func(taskID string) error {
		submitted = append(submitted, taskID)
		return nil
	}))
	now := time.Now().UTC()
	dueRetryAt := now.Add(-time.Minute)
	laterRetryAt := now.Add(10 * time.Minute)
	for _, task := range []*Task{
		{
			ID:        "task-due",
			TenantID:  "tenant-1",
			Status:    TaskStatusBlockedRetryable,
			Request:   &GenerateRequest{TenantID: "tenant-1", Platforms: []string{"shein"}, Text: "drawer knob"},
			RetryableBlock: &RetryableBlock{
				ReasonCode:           RetryableReasonOpenAIInsufficientCredits,
				ReasonMessage:        "OpenAI credits exhausted",
				BlockedAt:            now.Add(-2 * time.Minute),
				NextRetryAt:          &dueRetryAt,
				MaxAutoRetryAttempts: 8,
				AutoResumeEnabled:    true,
			},
			CreatedAt: now.Add(-2 * time.Minute),
			UpdatedAt: now.Add(-2 * time.Minute),
		},
		{
			ID:        "task-later",
			TenantID:  "tenant-1",
			Status:    TaskStatusBlockedRetryable,
			Request:   &GenerateRequest{TenantID: "tenant-1", Platforms: []string{"shein"}, Text: "drawer knob"},
			RetryableBlock: &RetryableBlock{
				ReasonCode:           RetryableReasonOpenAIInsufficientCredits,
				ReasonMessage:        "OpenAI credits exhausted",
				BlockedAt:            now.Add(-2 * time.Minute),
				NextRetryAt:          &laterRetryAt,
				MaxAutoRetryAttempts: 8,
				AutoResumeEnabled:    true,
			},
			CreatedAt: now.Add(-2 * time.Minute),
			UpdatedAt: now.Add(-2 * time.Minute),
		},
	} {
		if err := repo.CreateTask(WithTenantID(context.Background(), "tenant-1"), task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
		}
	}
	if err := svc.RunRecoverySweep(context.Background(), time.Now().UTC(), 20); err != nil {
		t.Fatalf("RunRecoverySweep() error = %v", err)
	}
	if diff := cmp.Diff([]string{"task-due"}, submitted); diff != "" {
		t.Fatalf("submitted mismatch (-want +got):\n%s", diff)
	}
}
```

- [ ] **Step 2: Run recovery-service tests to verify they fail**

Run: `go test ./internal/listingkit -run "TestRecoverTaskNowRequeuesBlockedTask|TestRunRecoverySweepRequeuesDueBlockedTasksOnly" -count=1`

Expected: FAIL because there is no recovery service or recovery API surface yet.

- [ ] **Step 3: Add the recovery service with bounded backoff**

```go
type taskRecoveryService struct {
	repo          Repository
	taskSubmitter func() TaskSubmitter
}

func nextRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 0:
		return time.Minute
	case 1:
		return 5 * time.Minute
	case 2:
		return 15 * time.Minute
	case 3:
		return 30 * time.Minute
	default:
		return time.Hour
	}
}

func (s *taskRecoveryService) RecoverTaskNow(ctx context.Context, taskID string) (*Task, error) {
	now := time.Now().UTC()
	if err := s.repo.RecoverBlockedTaskNow(ctx, taskID, now); err != nil {
		return nil, err
	}
	if s.taskSubmitter != nil && s.taskSubmitter() != nil {
		if err := s.taskSubmitter().Submit(taskID); err != nil {
			return nil, err
		}
	}
	return s.repo.GetTask(ctx, taskID)
}
```

- [ ] **Step 4: Wire the service into the runtime builder**

```go
type Service interface {
	TaskLifecycleService
	GenerationTaskService
	StudioBatchRunService
	StudioMediaService
	StoreAdminService
	InternalListingKitService
	TaskRecoveryService
}
```

- [ ] **Step 5: Re-run recovery-service tests**

Run: `go test ./internal/listingkit -run "TestRecoverTaskNowRequeuesBlockedTask|TestRunRecoverySweepRequeuesDueBlockedTasksOnly" -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_recovery_service.go internal/listingkit/interfaces.go internal/listingkit/httpapi/builders.go internal/listingkit/task_recovery_service_test.go
git commit -m "feat: add retryable task recovery service"
```

## Task 5: Expose Recovery APIs and Schema-Safe Rollout Path

**Files:**
- Create: `internal/listingkit/api/task_recovery_handler.go`
- Modify: `internal/listingkit/api/handler.go`
- Create: `internal/listingkit/api/task_recovery_handler_test.go`
- Modify: `internal/listingkit/httpapi/builders_test.go`

- [ ] **Step 1: Write failing handler tests**

```go
type stubRecoveryService struct {
	taskID   string
	bulkReq  *listingkit.RecoverTasksRequest
	result   *listingkit.Task
	count    int
}

func (s *stubRecoveryService) RecoverTaskNow(_ context.Context, taskID string) (*listingkit.Task, error) {
	s.taskID = taskID
	return s.result, nil
}

func (s *stubRecoveryService) BulkRecoverTasks(_ context.Context, req *listingkit.RecoverTasksRequest) (int, error) {
	s.bulkReq = req
	return s.count, nil
}

func TestRecoverTaskNowHandlerBindsTaskIDAndReturnsTask(t *testing.T) {
	svc := &stubRecoveryService{result: &listingkit.Task{ID: "task-1", Status: listingkit.TaskStatusPending}}
	h, _ := NewHandler(svc)
	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/recover", h.RecoverTaskNow)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/recover", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.taskID != "task-1" {
		t.Fatalf("taskID = %q, want task-1", svc.taskID)
	}
}

func TestBulkRecoverTasksHandlerBindsReasonCode(t *testing.T) {
	svc := &stubRecoveryService{count: 4}
	h, _ := NewHandler(svc)
	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/recover", h.BulkRecoverTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/recover", strings.NewReader(`{"reason_code":"openai_insufficient_credits","tenant_id":"tenant-1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.bulkReq == nil || svc.bulkReq.ReasonCode != "openai_insufficient_credits" {
		t.Fatalf("bulk req = %+v, want reason code bound", svc.bulkReq)
	}
}
```

- [ ] **Step 2: Run handler tests to verify they fail**

Run: `go test ./internal/listingkit/api -run "TestRecoverTaskNowHandlerBindsTaskIDAndReturnsTask|TestBulkRecoverTasksHandlerBindsReasonCode" -count=1`

Expected: FAIL because the handlers and service interface methods do not yet exist.

- [ ] **Step 3: Add recover-now and bulk-recover handlers**

```go
func (h *handler) RecoverTaskNow(c *gin.Context) {
	task, err := h.taskRecoveryService.RecoverTaskNow(requestContext(c), c.Param("task_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "task_recover_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_id": task.ID, "tenant_id": task.TenantID, "status": task.Status})
}

func (h *handler) BulkRecoverTasks(c *gin.Context) {
	var req listingkit.RecoverTasksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c, req.TenantID)
	count, err := h.taskRecoveryService.BulkRecoverTasks(requestContext(c, req.TenantID), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bulk_task_recover_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recovered_count": count})
}
```

- [ ] **Step 4: Verify schema migration still covers `Task` changes**

```go
func TestAutoMigrateListingKitTaskRepositoryIncludesRetryableBlockColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := autoMigrateListingKitTaskRepository(db); err != nil {
		t.Fatalf("autoMigrateListingKitTaskRepository() error = %v", err)
	}
	if !db.Migrator().HasColumn(&listingkit.Task{}, "retryable_block") {
		t.Fatal("retryable_block column missing after migration")
	}
	if !db.Migrator().HasColumn(&listingkit.Task{}, "status") {
		t.Fatal("status column missing after migration")
	}
}
```

- [ ] **Step 5: Re-run handler and migration tests**

Run: `go test ./internal/listingkit/api -run "TestRecoverTaskNowHandlerBindsTaskIDAndReturnsTask|TestBulkRecoverTasksHandlerBindsReasonCode" -count=1`

Run: `go test ./internal/listingkit/httpapi -run TestAutoMigrateListingKitTaskRepositoryIncludesRetryableBlockColumns -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/api/task_recovery_handler.go internal/listingkit/api/handler.go internal/listingkit/api/task_recovery_handler_test.go internal/listingkit/httpapi/builders_test.go
git commit -m "feat: expose retryable task recovery APIs"
```

## Task 6: Render Blocked-Retryable State and Recovery Actions in the Frontend

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/listingkit/tasks.ts`
- Modify: `web/listingkit-ui/src/lib/api/listingkit-response-schema.ts`
- Create: `web/listingkit-ui/src/lib/api/task-recovery.ts`
- Create: `web/listingkit-ui/src/lib/query/use-task-recovery.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-query.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-panel.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/queue/queue-screen-sections.tsx`
- Create: `web/listingkit-ui/src/lib/api/task-recovery.test.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-panel.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.test.tsx`

- [ ] **Step 1: Write failing frontend tests**

```tsx
it("renders blocked retryable task details and recover action", () => {
  render(
    <TaskStatusPanel
      task={{
        task_id: "task-1",
        status: "blocked_retryable",
        error: "OpenAI credits exhausted",
        retryable_block: {
          reason_code: "openai_insufficient_credits",
          reason_message: "OpenAI credits exhausted",
          retry_attempts: 2,
          next_retry_at: "2026-06-06T07:05:00Z",
          auto_resume_enabled: true,
        },
      }}
    />,
  );

  expect(screen.getByText("等待依赖恢复")).toBeInTheDocument();
  expect(screen.getByText(/OpenAI credits exhausted/)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "立即重试" })).toBeInTheDocument();
  expect(screen.getByText(/下一次自动重试/)).toBeInTheDocument();
});
```

- [ ] **Step 2: Run frontend tests to verify they fail**

Run: `npm test -- --run src/components/listingkit/tasks/task-status-panel.test.tsx src/components/listingkit/tasks/task-status-screen.test.tsx src/lib/api/task-recovery.test.ts`

Expected: FAIL because the type/schema/action plumbing does not exist and the UI does not recognize `blocked_retryable`.

- [ ] **Step 3: Add the new frontend types, parser, and API clients**

```ts
export type RetryableBlock = {
  reason_code?: string;
  reason_message?: string;
  blocked_at?: string;
  last_retry_at?: string;
  next_retry_at?: string;
  retry_attempts?: number;
  max_auto_retry_attempts?: number;
  recovery_scope?: string;
  auto_resume_enabled?: boolean;
  auto_retry_paused?: boolean;
};

export async function recoverListingKitTask(taskId: string) {
  return apiRequest<{ task_id: string; status: string }>(`/tasks/${taskId}/recover`, {
    method: "POST",
  });
}

export async function bulkRecoverListingKitTasks(body: {
  tenant_id?: string;
  reason_code?: string;
}) {
  return apiRequest<{ recovered_count: number }>("/tasks/recover", {
    method: "POST",
    body,
  });
}
```

- [ ] **Step 4: Render the blocked-retryable UI and keep it polling**

```ts
export function shouldPollTaskResult(status?: string) {
  return (
    status === "pending" ||
    status === "processing" ||
    status === "queued" ||
    status === "running" ||
    status === "blocked_retryable"
  );
}
```

```tsx
if (task.status === "blocked_retryable") {
  return (
    <Card className="border-amber-200 bg-amber-50/70 p-5">
      <div className="space-y-3">
        <div className="text-sm font-semibold text-amber-900">等待依赖恢复</div>
        <p className="text-sm leading-6 text-amber-900/80">
          {task.retryable_block?.reason_message ?? task.error ?? "上游依赖暂时不可用，系统会自动重试。"}
        </p>
        <div className="text-xs text-amber-800">
          下一次自动重试：{formatTaskDate(task.retryable_block?.next_retry_at) ?? "待调度"}
        </div>
        <Button onClick={() => onRecoverTaskNow?.()} type="button">立即重试</Button>
      </div>
    </Card>
  );
}
```

- [ ] **Step 5: Re-run frontend tests**

Run: `npm test -- --run src/components/listingkit/tasks/task-status-panel.test.tsx src/components/listingkit/tasks/task-status-screen.test.tsx src/lib/api/task-recovery.test.ts`

Run: `npm run typecheck`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/lib/types/listingkit/tasks.ts web/listingkit-ui/src/lib/api/listingkit-response-schema.ts web/listingkit-ui/src/lib/api/task-recovery.ts web/listingkit-ui/src/lib/query/use-task-recovery.ts web/listingkit-ui/src/components/listingkit/tasks/task-status-query.ts web/listingkit-ui/src/components/listingkit/tasks/task-status-panel.tsx web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.tsx web/listingkit-ui/src/components/listingkit/queue/queue-screen-sections.tsx web/listingkit-ui/src/lib/api/task-recovery.test.ts web/listingkit-ui/src/components/listingkit/tasks/task-status-panel.test.tsx web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.test.tsx
git commit -m "feat: show and recover blocked retryable tasks"
```

## Task 7: Rollout Verification and Safe Legacy Backfill Script

**Files:**
- Modify: `internal/listingkit/task_recovery_service.go`
- Create: `internal/listingkit/task_recovery_backfill_test.go`
- Reference: `cmd/listingkit-schema-migrate/main.go`

- [ ] **Step 1: Write the failing backfill classifier test**

```go
func TestBackfillRetryableFailuresReclassifiesKnownCreditErrors(t *testing.T) {
	repo := store.NewMemTaskRepository()
	ctx := context.Background()
	task := &Task{
		ID:       "task-legacy",
		Status:   TaskStatusFailed,
		Error:    "openai request failed: insufficient credits",
		Request:  &GenerateRequest{Platforms: []string{"shein"}},
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now().Add(-time.Hour),
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	count, err := backfillRetryableBlockedTasks(ctx, repo, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("backfillRetryableBlockedTasks() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	stored, _ := repo.GetTask(ctx, task.ID)
	if stored.Status != TaskStatusBlockedRetryable {
		t.Fatalf("status = %q, want %q", stored.Status, TaskStatusBlockedRetryable)
	}
}
```

- [ ] **Step 2: Run the backfill test to verify it fails**

Run: `go test ./internal/listingkit -run TestBackfillRetryableFailuresReclassifiesKnownCreditErrors -count=1`

Expected: FAIL because there is no backfill helper yet.

- [ ] **Step 3: Add a bounded backfill helper for rollout use**

```go
func backfillRetryableBlockedTasks(ctx context.Context, repo Repository, createdAfter time.Time) (int, error) {
	page, err := (&taskLifecycleService{repo: repo}).ListTasks(ctx, &TaskListQuery{
		Status:   string(TaskStatusFailed),
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range page.Items {
		if item.CreatedAt.Before(createdAfter) {
			continue
		}
		task, err := repo.GetTask(ctx, item.TaskID)
		if err != nil || task == nil {
			continue
		}
		if block := classifyRetryableTaskFailure(ctx, errors.New(task.Error)); block != nil {
			if err := repo.MarkBlockedRetryable(ctx, task.ID, task.Result, block, task.Error); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}
```

- [ ] **Step 4: Re-run the backfill test and final backend suite slice**

Run: `go test ./internal/listingkit -run "TestBackfillRetryableFailuresReclassifiesKnownCreditErrors|TestRecoverTaskNowRequeuesBlockedTask|TestCreateGenerateTaskMarksQueueBackpressureAsBlockedRetryable" -count=1`

Run: `go test ./internal/listingkit/api -run "TestRecoverTaskNowHandlerBindsTaskIDAndReturnsTask|TestBulkRecoverTasksHandlerBindsReasonCode" -count=1`

Expected: PASS

- [ ] **Step 5: Run final migration and frontend verification commands**

Run: `go run ./cmd/listingkit-schema-migrate -config config/config-dev.yaml`

Expected: `listingkit schema migration completed using config/config-dev.yaml`

Run: `npm test -- --run src/components/listingkit/tasks/task-status-panel.test.tsx src/components/listingkit/tasks/task-status-screen.test.tsx src/lib/api/task-recovery.test.ts`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_recovery_service.go internal/listingkit/task_recovery_backfill_test.go
git commit -m "feat: add retryable task recovery backfill"
```
