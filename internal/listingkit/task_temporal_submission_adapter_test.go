package listingkit

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTaskTemporalSubmissionAdapterUploadSheinPublishImagesReturnsInputWhenUploadNotNeeded(t *testing.T) {
	t.Parallel()

	adapter := newTaskTemporalSubmissionAdapter(taskTemporalSubmissionAdapterConfig{})
	input := &SheinPreparedSubmitPayload{
		TaskID:           "task-1",
		Action:           "publish",
		RequestID:        "req-1",
		Product:          makeReadySheinTask().Result.Shein.PreviewProduct,
		NeedsImageUpload: false,
	}

	out, err := adapter.UploadSheinPublishImages(context.Background(), input)
	if err != nil {
		t.Fatalf("UploadSheinPublishImages() error = %v", err)
	}
	if out != input {
		t.Fatalf("UploadSheinPublishImages() returned different payload pointer")
	}
}

func TestTaskTemporalSubmissionAdapterValidateReadinessBlocksOnFreshnessGate(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	adapter := newTaskTemporalSubmissionAdapter(taskTemporalSubmissionAdapterConfig{
		loadSheinPublishTask: func(context.Context, string) (*Task, *SheinPackage, error) {
			return task, task.Result.Shein, nil
		},
		normalizeSheinSubmitPackage: func(*Task, *SheinPackage, *SubmitTaskRequest, string) {},
		saveTaskResult:              func(context.Context, string, *ListingKitResult) error { return nil },
		validateSheinPublishFreshness: func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error) {
			return &SheinSubmitReadiness{
				Ready:  false,
				Status: "blocked",
				BlockingItems: []SheinReadinessItem{{
					Key:     sheinFreshnessCategoryKey,
					Label:   "类目模板新鲜度",
					Message: "当前类目模板已发生变化",
				}},
			}, nil
		},
	})

	err := adapter.ValidateSheinPublishReadiness(context.Background(), SheinPublishAttemptInput{
		TaskID:    task.ID,
		Action:    "publish",
		RequestID: "freshness-block-123",
	})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("validate readiness err = %v, want ErrSubmitBlocked", err)
	}
	if !strings.Contains(err.Error(), "当前类目模板已发生变化") {
		t.Fatalf("validate readiness err = %v, want freshness blocker message", err)
	}
}
