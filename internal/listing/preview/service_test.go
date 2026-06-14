package preview

import (
	"context"
	"errors"
	"testing"
)

type stubTaskRepository[T any] struct {
	getTask func(context.Context, string) (*T, error)
}

func (r stubTaskRepository[T]) GetTask(ctx context.Context, taskID string) (*T, error) {
	return r.getTask(ctx, taskID)
}

func TestTaskPreviewServiceGetTaskPreview(t *testing.T) {
	t.Parallel()

	type testTask struct {
		ID string
	}
	type testPreview struct {
		TaskID     string
		Platform   string
		Finalized  bool
		Decorators []string
	}

	task := &testTask{ID: "task-1"}
	svc := NewTaskPreviewService(TaskPreviewServiceConfig[testTask, testPreview]{
		Repository: stubTaskRepository[testTask]{
			getTask: func(_ context.Context, taskID string) (*testTask, error) {
				if taskID != task.ID {
					t.Fatalf("GetTask taskID = %q, want %q", taskID, task.ID)
				}
				return task, nil
			},
		},
		BuildPreview: func(_ context.Context, gotTask *testTask, platform string) (*testPreview, error) {
			if gotTask != task {
				t.Fatalf("BuildPreview task = %+v, want original task", gotTask)
			}
			if platform != "shein" {
				t.Fatalf("BuildPreview platform = %q, want shein", platform)
			}
			return &testPreview{TaskID: gotTask.ID, Platform: platform}, nil
		},
		FinalizePreview: func(_ context.Context, gotTask *testTask, preview *testPreview) error {
			if gotTask != task {
				t.Fatalf("FinalizePreview task = %+v, want original task", gotTask)
			}
			preview.Finalized = true
			preview.Decorators = append(preview.Decorators, "asset-generation", "store-resolution")
			return nil
		},
	})

	preview, err := svc.GetTaskPreview(context.Background(), task.ID, "shein")
	if err != nil {
		t.Fatalf("GetTaskPreview error = %v", err)
	}
	if preview == nil {
		t.Fatal("GetTaskPreview preview = nil, want populated preview")
	}
	if !preview.Finalized {
		t.Fatal("preview finalized = false, want true")
	}
	if len(preview.Decorators) != 2 {
		t.Fatalf("preview decorators = %+v, want finalized decorators", preview.Decorators)
	}
}

func TestTaskPreviewServiceGetTaskPreviewPropagatesFinalizeError(t *testing.T) {
	t.Parallel()

	type testTask struct {
		ID string
	}
	type testPreview struct{}

	wantErr := errors.New("finalize preview")
	svc := NewTaskPreviewService(TaskPreviewServiceConfig[testTask, testPreview]{
		Repository: stubTaskRepository[testTask]{
			getTask: func(_ context.Context, _ string) (*testTask, error) {
				return &testTask{ID: "task-2"}, nil
			},
		},
		BuildPreview: func(_ context.Context, _ *testTask, _ string) (*testPreview, error) {
			return &testPreview{}, nil
		},
		FinalizePreview: func(_ context.Context, _ *testTask, _ *testPreview) error {
			return wantErr
		},
	})

	_, err := svc.GetTaskPreview(context.Background(), "task-2", "amazon")
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetTaskPreview error = %v, want %v", err, wantErr)
	}
}
