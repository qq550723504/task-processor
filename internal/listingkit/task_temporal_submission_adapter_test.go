package listingkit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"task-processor/internal/listingkit/submission"
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

func TestTaskTemporalSubmissionAdapterStartWorkflowAttemptReturnsPreviewOnSuccess(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	var started SheinPublishWorkflowStartInput
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	adapter := newTaskTemporalSubmissionAdapter(taskTemporalSubmissionAdapterConfig{
		startSheinPublishWorkflow: func(_ context.Context, in SheinPublishWorkflowStartInput) error {
			started = in
			return nil
		},
		getTaskPreview: func(_ context.Context, taskID string, platform string) (*ListingKitPreview, error) {
			if taskID != task.ID {
				t.Fatalf("taskID = %q, want %q", taskID, task.ID)
			}
			if platform != "shein" {
				t.Fatalf("platform = %q, want shein", platform)
			}
			return expectedPreview, nil
		},
	})

	startedAt := time.Now()
	preview, err := adapter.startSheinPublishWorkflowAttempt(context.Background(), task.ID, task, &SubmitTaskRequest{
		ConfirmedFinal: true,
	}, sheinWorkflowSubmitOptions{
		platform:  "shein",
		action:    "publish",
		requestID: "workflow-start-123",
		startedAt: startedAt,
	})
	if err != nil {
		t.Fatalf("startSheinPublishWorkflowAttempt() error = %v", err)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}
	if started.TaskID != task.ID || started.Platform != "shein" || started.Action != "publish" || started.RequestID != "workflow-start-123" {
		t.Fatalf("started input = %+v, want mapped workflow start input", started)
	}
	if !started.ConfirmedFinal {
		t.Fatal("confirmed final = false, want true")
	}
	if !started.RequestedAt.Equal(startedAt) {
		t.Fatalf("requestedAt = %v, want %v", started.RequestedAt, startedAt)
	}
}

func TestTaskTemporalSubmissionAdapterStartWorkflowAttemptBuildsReplayPreview(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	adapter := newTaskTemporalSubmissionAdapter(taskTemporalSubmissionAdapterConfig{
		startSheinPublishWorkflow: func(context.Context, SheinPublishWorkflowStartInput) error {
			return &submission.SubmitInProgressError{
				Platform:  "shein",
				Action:    "publish",
				RequestID: "workflow-replay-123",
			}
		},
		getTaskPreview: func(_ context.Context, taskID string, platform string) (*ListingKitPreview, error) {
			if taskID != task.ID || platform != "shein" {
				t.Fatalf("getTaskPreview args = %q/%q, want %q/shein", taskID, platform, task.ID)
			}
			return expectedPreview, nil
		},
	})

	preview, err := adapter.startSheinPublishWorkflowAttempt(context.Background(), task.ID, task, nil, sheinWorkflowSubmitOptions{
		platform:  "shein",
		action:    "publish",
		requestID: "workflow-replay-123",
		startedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("startSheinPublishWorkflowAttempt() error = %v", err)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}
}

func TestTaskTemporalSubmissionAdapterStartWorkflowAttemptDelegatesFailureCleanup(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	startErr := errors.New("workflow start failed")
	var handled bool
	adapter := newTaskTemporalSubmissionAdapter(taskTemporalSubmissionAdapterConfig{
		startSheinPublishWorkflow: func(context.Context, SheinPublishWorkflowStartInput) error {
			return startErr
		},
		handleWorkflowStartFailure: func(_ context.Context, taskID string, gotTask *Task, opts sheinWorkflowSubmitOptions, err error) error {
			handled = true
			if taskID != task.ID {
				t.Fatalf("taskID = %q, want %q", taskID, task.ID)
			}
			if gotTask != task {
				t.Fatalf("task = %+v, want original task", gotTask)
			}
			if opts.action != "publish" || opts.requestID != "workflow-fail-123" {
				t.Fatalf("opts = %+v, want publish/workflow-fail-123", opts)
			}
			if !errors.Is(err, startErr) {
				t.Fatalf("err = %v, want %v", err, startErr)
			}
			return err
		},
	})

	preview, err := adapter.startSheinPublishWorkflowAttempt(context.Background(), task.ID, task, nil, sheinWorkflowSubmitOptions{
		platform:  "shein",
		action:    "publish",
		requestID: "workflow-fail-123",
		startedAt: time.Now(),
	})
	if preview != nil {
		t.Fatalf("preview = %+v, want nil", preview)
	}
	if !errors.Is(err, startErr) {
		t.Fatalf("startSheinPublishWorkflowAttempt() error = %v, want %v", err, startErr)
	}
	if !handled {
		t.Fatal("expected workflow start failure handler to be called")
	}
}
