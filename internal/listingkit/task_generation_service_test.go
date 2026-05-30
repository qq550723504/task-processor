package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
)

func TestTaskGenerationServiceGetTaskGenerationTasksAppliesFilters(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	task := &Task{
		ID:        "task-generation-service-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Result:    &ListingKitResult{TaskID: "task-generation-service-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	var listCalls int
	generation := newTaskGenerationService(taskGenerationServiceConfig{
		repo: repo,
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			listCalls++
			return []assetgeneration.Task{
				{TaskID: taskID, ID: "amazon:amazon-lifestyle", Platform: "amazon", Slot: "auxiliary", ExecutionMode: assetgeneration.ExecutionModeRendererBacked, ExecutionStatus: "completed", SatisfiedBy: assetgeneration.ExecutionModeGeneratedAsset, CanExecute: true},
				{TaskID: taskID, ID: "shein:shein-main-model", Platform: "shein", Slot: "main", ExecutionMode: assetgeneration.ExecutionModeDeferredStub, ExecutionStatus: "completed", SatisfiedBy: "fallback_asset", CanExecute: true},
			}, nil
		},
	})

	page, err := generation.GetTaskGenerationTasks(context.Background(), task.ID, &GenerationTaskQuery{
		Platform:    "shein",
		Slot:        "main",
		SatisfiedBy: "fallback_asset",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationTasks() error = %v", err)
	}
	if listCalls != 1 {
		t.Fatalf("list calls = %d, want 1", listCalls)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != "shein:shein-main-model" {
		t.Fatalf("tasks = %+v, want filtered shein main task", page.Tasks)
	}
	if page.Summary == nil || page.Summary.FallbackTasks != 1 {
		t.Fatalf("summary = %+v, want fallback summary", page.Summary)
	}
}

func TestTaskGenerationTasksReadSnapshotRunUsesSingleCurrentSnapshot(t *testing.T) {
	t.Parallel()

	const taskID = "task-generation-tasks-snapshot-1"
	updatedAt := time.Date(2026, 5, 30, 11, 0, 0, 0, time.UTC)
	repo := &sequencedTaskSnapshotsRepo{
		snapshots: []*Task{
			{
				ID:        taskID,
				UpdatedAt: updatedAt,
				Result:    &ListingKitResult{TaskID: taskID},
			},
			{
				ID:        taskID,
				UpdatedAt: updatedAt.Add(2 * time.Hour),
				Result:    &ListingKitResult{TaskID: taskID},
			},
		},
	}
	listCalls := 0
	svc := &taskGenerationService{
		repo: repo,
		listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
			listCalls++
			if listCalls == 1 {
				return []assetgeneration.Task{{
					TaskID:          taskID,
					ID:              "amazon:amazon-main-white-bg",
					Platform:        "amazon",
					ExecutionStatus: "planned",
				}}, nil
			}
			return []assetgeneration.Task{{
				TaskID:          taskID,
				ID:              "shein:shein-main-model",
				Platform:        "shein",
				ExecutionStatus: "completed",
			}}, nil
		},
	}

	snapshot, err := buildTaskGenerationTasksReadSnapshotPhase(svc).run(context.Background(), taskID)
	if err != nil {
		t.Fatalf("taskGenerationTasksReadSnapshotPhase.run() error = %v", err)
	}
	if snapshot == nil || snapshot.task == nil {
		t.Fatalf("snapshot = %+v, want hydrated task snapshot", snapshot)
	}
	if repo.getCalls != 1 {
		t.Fatalf("repo.getCalls = %d, want single current snapshot read", repo.getCalls)
	}
	if listCalls != 1 {
		t.Fatalf("listAssetGenerationTasks calls = %d, want single persisted task read", listCalls)
	}
	if !snapshot.task.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("snapshot.task.UpdatedAt = %v, want %v", snapshot.task.UpdatedAt, updatedAt)
	}
	if len(snapshot.tasks) != 1 || snapshot.tasks[0].ID != "amazon:amazon-main-white-bg" {
		t.Fatalf("snapshot.tasks = %+v, want first persisted generation task snapshot", snapshot.tasks)
	}
}

func TestTaskGenerationTasksReadSnapshotRunPropagatesLoadErrors(t *testing.T) {
	t.Parallel()

	taskListErr := errors.New("generation task snapshot load failed")
	tests := []struct {
		name    string
		service *taskGenerationService
		taskID  string
		wantErr error
	}{
		{
			name:    "repo get task",
			service: &taskGenerationService{repo: &stubGenerationRepo{}},
			taskID:  "task-generation-tasks-snapshot-missing-1",
			wantErr: ErrTaskNotFound,
		},
		{
			name: "list asset generation tasks",
			service: &taskGenerationService{
				repo: &stubGenerationRepo{task: &Task{
					ID:     "task-generation-tasks-snapshot-error-1",
					Result: &ListingKitResult{TaskID: "task-generation-tasks-snapshot-error-1"},
				}},
				listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
					return nil, taskListErr
				},
			},
			taskID:  "task-generation-tasks-snapshot-error-1",
			wantErr: taskListErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTaskGenerationTasksReadSnapshotPhase(tc.service).run(context.Background(), tc.taskID)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("taskGenerationTasksReadSnapshotPhase.run() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestTaskGenerationServiceGetTaskGenerationTasksUsesSingleReadSnapshotHandoff(t *testing.T) {
	t.Parallel()

	const taskID = "task-generation-service-handoff-1"
	firstUpdatedAt := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	repo := &sequencedTaskSnapshotsRepo{
		snapshots: []*Task{
			{
				ID:        taskID,
				UpdatedAt: firstUpdatedAt,
				Result:    &ListingKitResult{TaskID: taskID},
			},
			{
				ID:        taskID,
				UpdatedAt: firstUpdatedAt.Add(3 * time.Hour),
				Result:    &ListingKitResult{TaskID: taskID},
			},
		},
	}
	listCalls := 0
	generation := newTaskGenerationService(taskGenerationServiceConfig{
		repo: repo,
		listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
			listCalls++
			if listCalls == 1 {
				return []assetgeneration.Task{
					{TaskID: taskID, ID: "shein:shein-main-model", Platform: "shein", Slot: "main", ExecutionStatus: "completed", SatisfiedBy: "fallback_asset"},
				}, nil
			}
			return []assetgeneration.Task{
				{TaskID: taskID, ID: "amazon:amazon-main-white-bg", Platform: "amazon", Slot: "main", ExecutionStatus: "planned"},
			}, nil
		},
	})

	page, err := generation.GetTaskGenerationTasks(context.Background(), taskID, &GenerationTaskQuery{
		Platform: "shein",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationTasks() error = %v", err)
	}
	if repo.getCalls != 1 {
		t.Fatalf("repo.getCalls = %d, want single task snapshot acquisition", repo.getCalls)
	}
	if listCalls != 1 {
		t.Fatalf("listAssetGenerationTasks calls = %d, want single persisted task acquisition", listCalls)
	}
	if page == nil {
		t.Fatal("page = nil, want generation task page")
	}
	if page.TaskID != taskID {
		t.Fatalf("page.TaskID = %q, want %q", page.TaskID, taskID)
	}
	if !page.UpdatedAt.Equal(firstUpdatedAt) {
		t.Fatalf("page.UpdatedAt = %v, want %v", page.UpdatedAt, firstUpdatedAt)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != "shein:shein-main-model" {
		t.Fatalf("page.Tasks = %+v, want first acquired persisted task snapshot", page.Tasks)
	}
}

func TestTaskGenerationTasksReadPagePhaseReturnsStableEmptyShape(t *testing.T) {
	t.Parallel()

	page := buildTaskGenerationTasksReadPagePhase().run(&taskGenerationTasksReadSnapshot{
		task: &Task{
			ID:        "task-generation-read-page-empty-1",
			UpdatedAt: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC),
		},
	}, &GenerationTaskQuery{Page: 3, PageSize: 250})

	if page == nil {
		t.Fatal("page = nil, want generation task page")
	}
	if page.TaskID != "task-generation-read-page-empty-1" {
		t.Fatalf("page.TaskID = %q, want task-generation-read-page-empty-1", page.TaskID)
	}
	if page.Page != 3 || page.PageSize != maxGenerationTaskPageSize || page.Total != 0 {
		t.Fatalf("page meta = %+v, want page=3 page_size=%d total=0", page, maxGenerationTaskPageSize)
	}
	if page.Summary == nil || page.Summary.TotalTasks != 0 {
		t.Fatalf("page.Summary = %+v, want empty summary", page.Summary)
	}
	if len(page.Tasks) != 0 {
		t.Fatalf("page.Tasks = %+v, want empty task slice", page.Tasks)
	}
}

func TestTaskGenerationTasksReadPagePhaseUsesFilteredSetForSummaryAndPagedSetForTasks(t *testing.T) {
	t.Parallel()

	snapshot := &taskGenerationTasksReadSnapshot{
		task: &Task{
			ID:        "task-generation-read-page-filtered-summary-1",
			UpdatedAt: time.Date(2026, 5, 30, 14, 0, 0, 0, time.UTC),
		},
		tasks: []assetgeneration.Task{
			{TaskID: "task-generation-read-page-filtered-summary-1", ID: "shein:shein-main-model", Platform: "shein", Slot: "main", ExecutionStatus: "completed", SatisfiedBy: "fallback_asset"},
			{TaskID: "task-generation-read-page-filtered-summary-1", ID: "amazon:amazon-lifestyle", Platform: "amazon", Slot: "auxiliary", ExecutionMode: assetgeneration.ExecutionModeRendererBacked, ExecutionStatus: "completed", SatisfiedBy: assetgeneration.ExecutionModeGeneratedAsset},
			{TaskID: "task-generation-read-page-filtered-summary-1", ID: "amazon:amazon-main-white-bg", Platform: "amazon", Slot: "main", ExecutionStatus: "planned"},
		},
	}

	page := buildTaskGenerationTasksReadPagePhase().run(snapshot, &GenerationTaskQuery{
		Page:      2,
		PageSize:  1,
		SortBy:    "platform",
		SortOrder: "asc",
	})

	if page == nil {
		t.Fatal("page = nil, want generation task page")
	}
	if page.Total != 3 {
		t.Fatalf("page.Total = %d, want 3 filtered tasks before pagination", page.Total)
	}
	if page.Page != 2 || page.PageSize != 1 {
		t.Fatalf("page meta = %+v, want page=2 page_size=1", page)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != "amazon:amazon-main-white-bg" {
		t.Fatalf("page.Tasks = %+v, want second task in sorted paged set", page.Tasks)
	}
	if page.Summary == nil || page.Summary.TotalTasks != 3 {
		t.Fatalf("page.Summary = %+v, want summary built from filtered set", page.Summary)
	}
	if page.Summary.RendererBackedTasks != 1 {
		t.Fatalf("page.Summary = %+v, want renderer-backed count from full filtered set", page.Summary)
	}
	if len(page.Summary.Platforms) != 2 {
		t.Fatalf("page.Summary.Platforms = %+v, want platforms from full filtered set", page.Summary.Platforms)
	}
}

func TestTaskGenerationServiceRetryTaskGenerationTasksReturnsEmptyPageWithoutSelection(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	task := &Task{
		ID:        "task-generation-retry-service-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result:    &ListingKitResult{TaskID: "task-generation-retry-service-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
	}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}

	generator := &stubWorkflowAssetGenerator{
		dispatchResult: &assetgeneration.Result{},
		dispatchErrAt: map[int]error{
			1: context.Canceled,
		},
	}
	generation := newTaskGenerationService(taskGenerationServiceConfig{
		repo:           repo,
		assetRepo:      assetRepository,
		assetGenerator: generator,
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			return []assetgeneration.Task{}, nil
		},
		buildRetryGenerationTaskSelection: func(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
			return nil, nil
		},
	})

	page, err := generation.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if generator.dispatchCalls != 0 {
		t.Fatalf("dispatch calls = %d, want 0", generator.dispatchCalls)
	}
	if page == nil || page.Total != 0 || page.MatchedQueue == nil || page.ExecutedQueue == nil {
		t.Fatalf("page = %+v, want empty retry page with queue placeholders", page)
	}
}
