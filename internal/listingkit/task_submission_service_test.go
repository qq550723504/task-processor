package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "task-processor/internal/core/errors"
	"task-processor/internal/listingkit/core"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestTaskSubmissionServiceSubmitTaskRoutesSheinPublishToWorkflow(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	var workflowCalls int
	var directCalls int
	submitter := newTaskSubmissionService(taskSubmissionServiceConfig{
		lockSubmit: func(string) func() { return func() {} },
		acquireSheinSubmitTask: func(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
			return task, nil, nil
		},
		shouldStartSheinPublishWorkflow: func(platform, action string) bool {
			return platform == "shein" && action == "publish"
		},
		submitSheinTaskWithWorkflow: func(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {
			workflowCalls++
			return &ListingKitPreview{TaskID: taskID}, nil
		},
		submitSheinTaskDirect: func(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
			directCalls++
			return nil, nil
		},
	})

	preview, err := submitter.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "temporal-route-123",
	})
	if err != nil {
		t.Fatalf("SubmitTask() error = %v", err)
	}
	if preview == nil || preview.TaskID != task.ID {
		t.Fatalf("preview = %+v, want preview for task", preview)
	}
	if workflowCalls != 1 {
		t.Fatalf("workflow calls = %d, want 1", workflowCalls)
	}
	if directCalls != 0 {
		t.Fatalf("direct calls = %d, want 0", directCalls)
	}
}

func TestTaskSubmissionServiceSubmitTaskUsesResolvedDefaultActionWhenRequestActionMissing(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	var acquiredAction string
	var directAction string
	submitter := newTaskSubmissionService(taskSubmissionServiceConfig{
		lockSubmit: func(string) func() { return func() {} },
		resolveDefaultSheinSubmitAction: func(_ context.Context, taskID string) (string, error) {
			if taskID != task.ID {
				t.Fatalf("taskID = %q, want %q", taskID, task.ID)
			}
			return "save_draft", nil
		},
		acquireSheinSubmitTask: func(_ context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
			acquiredAction = action
			return task, nil, nil
		},
		shouldStartSheinPublishWorkflow: func(platform, action string) bool {
			return false
		},
		submitSheinTaskDirect: func(_ context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
			directAction = opts.action
			return &ListingKitPreview{TaskID: taskID}, nil
		},
	})

	preview, err := submitter.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform: "shein",
	})
	if err != nil {
		t.Fatalf("SubmitTask() error = %v", err)
	}
	if preview == nil || preview.TaskID != task.ID {
		t.Fatalf("preview = %+v, want preview for task", preview)
	}
	if acquiredAction != "save_draft" {
		t.Fatalf("acquired action = %q, want save_draft", acquiredAction)
	}
	if directAction != "save_draft" {
		t.Fatalf("direct action = %q, want save_draft", directAction)
	}
}

func TestBuildWorkflowSubmitOptionsMapsAttemptFields(t *testing.T) {
	t.Parallel()

	startedAt := time.Now()
	opts := buildWorkflowSubmitOptions(&sheinSubmissionAttemptState{
		platform:  "shein",
		action:    "publish",
		requestID: "workflow-123",
		startedAt: startedAt,
	})

	if opts.platform != "shein" {
		t.Fatalf("platform = %q, want shein", opts.platform)
	}
	if opts.action != "publish" {
		t.Fatalf("action = %q, want publish", opts.action)
	}
	if opts.requestID != "workflow-123" {
		t.Fatalf("requestID = %q, want workflow-123", opts.requestID)
	}
	if !opts.startedAt.Equal(startedAt) {
		t.Fatalf("startedAt = %v, want %v", opts.startedAt, startedAt)
	}
}

func TestBuildDirectSubmitOptionsMapsAttemptFields(t *testing.T) {
	t.Parallel()

	startedAt := time.Now()
	opts := buildDirectSubmitOptions(&sheinSubmissionAttemptState{
		action:    "save_draft",
		requestID: "direct-123",
		startedAt: startedAt,
	})

	if opts.action != "save_draft" {
		t.Fatalf("action = %q, want save_draft", opts.action)
	}
	if opts.requestID != "direct-123" {
		t.Fatalf("requestID = %q, want direct-123", opts.requestID)
	}
	if !opts.startedAt.Equal(startedAt) {
		t.Fatalf("startedAt = %v, want %v", opts.startedAt, startedAt)
	}
}

func TestBuildSubmissionAttemptStateDerivesWorkflowRequestID(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 9, 14, 0, 0, 0, time.FixedZone("CST", 8*3600))
	attempt := buildSubmissionAttemptState("task-123", "shein", "publish", &SubmitTaskRequest{}, startedAt, func(platform, action string) bool {
		return platform == "shein" && action == "publish"
	})

	if attempt == nil {
		t.Fatal("attempt = nil")
	}
	if !attempt.useWorkflow {
		t.Fatal("useWorkflow = false, want true")
	}
	if attempt.requestID == "" {
		t.Fatal("requestID = empty, want derived workflow request id")
	}
	if attempt.requestID != derivedSheinSubmitRequestID("task-123", "publish", startedAt) {
		t.Fatalf("requestID = %q, want derived id", attempt.requestID)
	}
	if !attempt.startedAt.Equal(startedAt) {
		t.Fatalf("startedAt = %v, want %v", attempt.startedAt, startedAt)
	}
}

func TestBuildSubmissionAttemptStateKeepsExplicitRequestID(t *testing.T) {
	t.Parallel()

	startedAt := time.Now()
	attempt := buildSubmissionAttemptState("task-123", "shein", "publish", &SubmitTaskRequest{
		IdempotencyKey: "explicit-req-1",
	}, startedAt, func(string, string) bool { return true })

	if attempt == nil {
		t.Fatal("attempt = nil")
	}
	if attempt.requestID != "explicit-req-1" {
		t.Fatalf("requestID = %q, want explicit-req-1", attempt.requestID)
	}
}

func TestBuildSubmissionRefreshRemoteInputsPreservesCurrentFallbackBehavior(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		LastResult: &sheinpub.SubmissionResponse{
			Success: true,
			SPUName: "SPU-PUBLISH",
		},
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
			Result: &sheinpub.SubmissionResponse{
				Success: true,
				SPUName: "SPU-PUBLISH",
			},
		},
	}

	inputs := buildSubmissionRefreshRemoteInputs(task.Result.Shein, "publish", "SKC-1")
	if !inputs.defaultConfirmed {
		t.Fatal("defaultConfirmed = false, want true")
	}
	if got := len(inputs.lookupCodes); got == 0 {
		t.Fatal("lookupCodes = empty, want collected codes")
	}
	if inputs.spuName != "SPU-PUBLISH" {
		t.Fatalf("spuName = %q, want SPU-PUBLISH", inputs.spuName)
	}
	if inputs.fallbackMessage != "" {
		t.Fatalf("fallbackMessage = %q, want empty to preserve current submission-service behavior", inputs.fallbackMessage)
	}
}

func TestBuildSubmissionRefreshRequestIDTrimsRecordValue(t *testing.T) {
	t.Parallel()

	requestID := buildSubmissionRefreshRequestID(&sheinpub.SubmissionRecord{RequestID: "  refresh-123  "})
	if requestID != "refresh-123" {
		t.Fatalf("requestID = %q, want refresh-123", requestID)
	}
}

func TestBuildSubmissionRefreshRequestMapsSelectionAndRemoteInputs(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		LastResult: &sheinpub.SubmissionResponse{
			Success: true,
			SPUName: "SPU-PUBLISH",
		},
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "  refresh-123  ",
			SupplierCode: "SKC-1",
			StartedAt:    now,
			Result: &sheinpub.SubmissionResponse{
				Success: true,
				SPUName: "SPU-PUBLISH",
			},
		},
	}

	request := buildSubmissionRefreshRequest(task.Result.Shein, &sheinSubmissionRefreshSelection{
		action:       "publish",
		record:       task.Result.Shein.Submission.Publish,
		supplierCode: "SKC-1",
	})

	if request.action != "publish" {
		t.Fatalf("action = %q, want publish", request.action)
	}
	if request.requestID != "refresh-123" {
		t.Fatalf("requestID = %q, want refresh-123", request.requestID)
	}
	if len(request.remoteInputs.lookupCodes) == 0 {
		t.Fatal("lookupCodes = empty, want collected codes")
	}
	if !request.remoteInputs.defaultConfirmed {
		t.Fatal("defaultConfirmed = false, want true")
	}
	if request.remoteInputs.spuName != "SPU-PUBLISH" {
		t.Fatalf("spuName = %q, want SPU-PUBLISH", request.remoteInputs.spuName)
	}
	if request.remoteInputs.fallbackMessage != "" {
		t.Fatalf("fallbackMessage = %q, want empty", request.remoteInputs.fallbackMessage)
	}
}

func TestNewSubmissionRefreshStateMapsInputs(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	startedAt := time.Now()
	productAPI := stubSheinProductAPI{}
	otherAPI := stubSheinOtherAPI{}
	state := newSubmissionRefreshState(task, "publish", "refresh-123", startedAt, productAPI, otherAPI, sheinSubmissionRefreshRemoteInputs{
		lookupCodes:      []string{"SKC-1", "SKU-1"},
		spuName:          "SPU-PUBLISH",
		defaultConfirmed: true,
		fallbackMessage:  "",
	})

	if state == nil {
		t.Fatal("state = nil")
	}
	if state.task != task {
		t.Fatalf("task = %+v, want original task", state.task)
	}
	if state.action != "publish" {
		t.Fatalf("action = %q, want publish", state.action)
	}
	if state.requestID != "refresh-123" {
		t.Fatalf("requestID = %q, want refresh-123", state.requestID)
	}
	if !state.startedAt.Equal(startedAt) {
		t.Fatalf("startedAt = %v, want %v", state.startedAt, startedAt)
	}
	if len(state.lookupCodes) != 2 {
		t.Fatalf("lookupCodes = %+v, want 2 entries", state.lookupCodes)
	}
	if !state.defaultConfirmed {
		t.Fatal("defaultConfirmed = false, want true")
	}
	if state.spuName != "SPU-PUBLISH" {
		t.Fatalf("spuName = %q, want SPU-PUBLISH", state.spuName)
	}
	if state.productAPI == nil {
		t.Fatal("productAPI = nil, want assigned api")
	}
	if state.otherAPI == nil {
		t.Fatal("otherAPI = nil, want assigned api")
	}
}

func TestBuildSubmissionRefreshConfirmationRequestMapsRefreshState(t *testing.T) {
	t.Parallel()

	startedAt := time.Now()
	productAPI := stubSheinProductAPI{}
	otherAPI := stubSheinOtherAPI{}
	request, err := buildSubmissionRefreshConfirmationRequest("task-123", &sheinSubmissionRefreshState{
		action:           "publish",
		requestID:        "refresh-123",
		startedAt:        startedAt,
		lookupCodes:      []string{"SKC-1", "SKU-1"},
		defaultConfirmed: true,
		fallbackMessage:  "",
		productAPI:       productAPI,
		otherAPI:         otherAPI,
		spuName:          "SPU-PUBLISH",
	})
	if err != nil {
		t.Fatalf("buildSubmissionRefreshConfirmationRequest() error = %v", err)
	}
	if request == nil {
		t.Fatal("request = nil")
	}
	if request.taskID != "task-123" {
		t.Fatalf("taskID = %q, want task-123", request.taskID)
	}
	if request.action != "publish" {
		t.Fatalf("action = %q, want publish", request.action)
	}
	if request.requestID != "refresh-123" {
		t.Fatalf("requestID = %q, want refresh-123", request.requestID)
	}
	if len(request.lookupCodes) != 2 {
		t.Fatalf("lookupCodes = %+v, want 2 entries", request.lookupCodes)
	}
	if !request.defaultConfirmed {
		t.Fatal("defaultConfirmed = false, want true")
	}
	if request.fallbackMessage != "" {
		t.Fatalf("fallbackMessage = %q, want empty", request.fallbackMessage)
	}
	if request.spuName != "SPU-PUBLISH" {
		t.Fatalf("spuName = %q, want SPU-PUBLISH", request.spuName)
	}
	if request.productAPI == nil {
		t.Fatal("productAPI = nil, want assigned api")
	}
	if request.otherAPI == nil {
		t.Fatal("otherAPI = nil, want assigned api")
	}
	if !request.startedAt.Equal(startedAt) {
		t.Fatalf("startedAt = %v, want %v", request.startedAt, startedAt)
	}
}

func TestLoadSubmissionRefreshSelectionMapsFields(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now()
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}

	selection, err := loadSubmissionRefreshSelection(task.Result.Shein)
	if err != nil {
		t.Fatalf("loadSubmissionRefreshSelection() error = %v", err)
	}
	if selection == nil {
		t.Fatal("selection = nil")
	}
	if selection.action != "publish" {
		t.Fatalf("action = %q, want publish", selection.action)
	}
	if selection.record == nil || selection.record.RequestID != "refresh-123" {
		t.Fatalf("record = %+v, want request id refresh-123", selection.record)
	}
	if selection.supplierCode != "SKC-1" {
		t.Fatalf("supplierCode = %q, want SKC-1", selection.supplierCode)
	}
}

func TestLoadSubmissionRefreshSelectionFallsBackToPublishWhenLastActionMissing(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now()
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}

	selection, err := loadSubmissionRefreshSelection(task.Result.Shein)
	if err != nil {
		t.Fatalf("loadSubmissionRefreshSelection() error = %v", err)
	}
	if selection == nil {
		t.Fatal("selection = nil")
	}
	if selection.action != "publish" {
		t.Fatalf("action = %q, want publish", selection.action)
	}
	if selection.record == nil || selection.record.RequestID != "refresh-123" {
		t.Fatalf("record = %+v, want request id refresh-123", selection.record)
	}
}

func TestLoadSubmissionRefreshSelectionFallsBackToPackageSupplierCode(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now()
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:    "publish",
			RequestID: "refresh-123",
			StartedAt: now,
		},
	}

	selection, err := loadSubmissionRefreshSelection(task.Result.Shein)
	if err != nil {
		t.Fatalf("loadSubmissionRefreshSelection() error = %v", err)
	}
	if selection == nil {
		t.Fatal("selection = nil")
	}
	if selection.supplierCode != "SKC-1" {
		t.Fatalf("supplierCode = %q, want SKC-1", selection.supplierCode)
	}
}

func TestLoadSubmissionRefreshTaskPackageRejectsMissingSubmissionState(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.Result.Shein.Submission = nil

	pkg, err := loadSubmissionRefreshTaskPackage(task)
	if err == nil {
		t.Fatal("err = nil, want validation error")
	}
	if pkg != nil {
		t.Fatalf("pkg = %+v, want nil", pkg)
	}
}

func TestLoadSubmissionRefreshMutationPackageUsesSharedTaskPackageLoader(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now()
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}

	pkg, err := loadSubmissionRefreshMutationPackage(task)
	if err != nil {
		t.Fatalf("loadSubmissionRefreshMutationPackage() error = %v", err)
	}
	if pkg == nil || pkg.SubmissionState == nil {
		t.Fatalf("pkg = %+v, want submission state", pkg)
	}
}

func TestTaskSubmissionServiceFinishSubmissionRefreshReturnsRemoteErrorAfterPersisting(t *testing.T) {
	t.Parallel()

	var mutateCalls int
	var previewCalls int
	remoteErr := errors.New("remote refresh failed")
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}
	submitter := newTaskSubmissionService(taskSubmissionServiceConfig{
		mutateTaskResult: func(_ context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
			mutateCalls++
			if taskID != task.ID {
				t.Fatalf("taskID = %q, want %q", taskID, task.ID)
			}
			if mutate == nil {
				t.Fatal("expected mutation callback")
			}
			if err := mutate(task); err != nil {
				t.Fatalf("mutate(task) error = %v", err)
			}
			return task, nil
		},
		buildTaskPreview: func(context.Context, *Task, string) (*ListingKitPreview, error) {
			previewCalls++
			return &ListingKitPreview{TaskID: task.ID}, nil
		},
	})

	preview, err := submitter.finishSubmissionRefresh(context.Background(), task.ID, &sheinSubmissionRefreshState{
		action:    "publish",
		requestID: "refresh-123",
		startedAt: now,
	}, nil, remoteErr)
	if !errors.Is(err, remoteErr) {
		t.Fatalf("finishSubmissionRefresh() error = %v, want %v", err, remoteErr)
	}
	if preview != nil {
		t.Fatalf("preview = %+v, want nil", preview)
	}
	if mutateCalls != 1 {
		t.Fatalf("mutate calls = %d, want 1", mutateCalls)
	}
	if previewCalls != 0 {
		t.Fatalf("preview calls = %d, want 0", previewCalls)
	}
}

func TestTaskSubmissionServiceFinishSubmissionRefreshBuildsPreviewOnSuccess(t *testing.T) {
	t.Parallel()

	var mutateCalls int
	var previewCalls int
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	submitter := newTaskSubmissionService(taskSubmissionServiceConfig{
		mutateTaskResult: func(_ context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
			mutateCalls++
			if taskID != task.ID {
				t.Fatalf("taskID = %q, want %q", taskID, task.ID)
			}
			if err := mutate(task); err != nil {
				t.Fatalf("mutate(task) error = %v", err)
			}
			return task, nil
		},
		buildTaskPreview: func(_ context.Context, previewTask *Task, platform string) (*ListingKitPreview, error) {
			previewCalls++
			if previewTask != task {
				t.Fatalf("preview task = %+v, want original task", previewTask)
			}
			if platform != "shein" {
				t.Fatalf("platform = %q, want shein", platform)
			}
			return expectedPreview, nil
		},
	})

	preview, err := submitter.finishSubmissionRefresh(context.Background(), task.ID, &sheinSubmissionRefreshState{
		action:    "publish",
		requestID: "refresh-123",
		startedAt: now,
	}, nil, nil)
	if err != nil {
		t.Fatalf("finishSubmissionRefresh() error = %v", err)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}
	if mutateCalls != 1 {
		t.Fatalf("mutate calls = %d, want 1", mutateCalls)
	}
	if previewCalls != 1 {
		t.Fatalf("preview calls = %d, want 1", previewCalls)
	}
}

func TestTaskSubmissionServiceLoadSheinSubmissionRefreshStateMapsLoadedTask(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		LastResult: &sheinpub.SubmissionResponse{
			Success: true,
			SPUName: "SPU-PUBLISH",
		},
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
			Result: &sheinpub.SubmissionResponse{
				Success: true,
				SPUName: "SPU-PUBLISH",
			},
		},
	}
	productAPI := stubSheinProductAPI{}
	otherAPI := stubSheinOtherAPI{}
	submitter := newTaskSubmissionService(taskSubmissionServiceConfig{
		repo: &stubSubmitRepo{task: task},
		buildSheinSubmitProductAPI: func(context.Context, *Task) (sheinproduct.ProductAPI, error) {
			return productAPI, nil
		},
		buildSheinSubmitOtherAPI: func(context.Context, *Task) (sheinother.OtherAPI, error) {
			return otherAPI, nil
		},
	})

	state, err := submitter.loadSheinSubmissionRefreshState(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("loadSheinSubmissionRefreshState() error = %v", err)
	}
	if state == nil {
		t.Fatal("state = nil")
	}
	if state.task == nil || state.task.ID != task.ID {
		t.Fatalf("task = %+v, want task %q", state.task, task.ID)
	}
	if state.action != "publish" {
		t.Fatalf("action = %q, want publish", state.action)
	}
	if state.requestID != "refresh-123" {
		t.Fatalf("requestID = %q, want refresh-123", state.requestID)
	}
	if len(state.lookupCodes) == 0 {
		t.Fatal("lookupCodes = empty, want collected codes")
	}
	if !state.defaultConfirmed {
		t.Fatal("defaultConfirmed = false, want true")
	}
	if state.spuName != "SPU-PUBLISH" {
		t.Fatalf("spuName = %q, want SPU-PUBLISH", state.spuName)
	}
	if state.productAPI == nil {
		t.Fatal("productAPI = nil, want assigned api")
	}
	if state.otherAPI == nil {
		t.Fatal("otherAPI = nil, want assigned api")
	}
}

func TestTaskSubmissionServiceLoadSheinSubmissionRefreshStateWrapsProductAPIError(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}
	buildErr := errors.New("product api unavailable")
	submitter := newTaskSubmissionService(taskSubmissionServiceConfig{
		repo: &stubSubmitRepo{task: task},
		buildSheinSubmitProductAPI: func(context.Context, *Task) (sheinproduct.ProductAPI, error) {
			return nil, buildErr
		},
	})

	state, err := submitter.loadSheinSubmissionRefreshState(context.Background(), task.ID)
	if err == nil {
		t.Fatal("err = nil, want wrapped platform error")
	}
	if state != nil {
		t.Fatalf("state = %+v, want nil", state)
	}
	if !errors.Is(err, buildErr) {
		t.Fatalf("error = %v, want wrapped build error", err)
	}
	if !apperrors.IsCode(err, apperrors.ErrCodePlatformError) {
		t.Fatalf("error code = %q, want %q", apperrors.GetCode(err), apperrors.ErrCodePlatformError)
	}
}

func TestTaskSubmissionServiceResolveSubmissionRefreshConfirmationPassesRequestFields(t *testing.T) {
	t.Parallel()

	productAPI := stubSheinProductAPI{}
	otherAPI := stubSheinOtherAPI{}
	startedAt := time.Now()
	expected := &sheinRemoteConfirmation{message: "resolved"}
	submitter := newTaskSubmissionService(taskSubmissionServiceConfig{
		resolveRemoteStatus: func(gotProductAPI sheinproduct.ProductAPI, gotOtherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, gotStartedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
			if gotProductAPI == nil {
				t.Fatal("productAPI = nil, want assigned api")
			}
			if gotOtherAPI == nil {
				t.Fatal("otherAPI = nil, want assigned api")
			}
			if action != "publish" {
				t.Fatalf("action = %q, want publish", action)
			}
			if requestID != "refresh-123" {
				t.Fatalf("requestID = %q, want refresh-123", requestID)
			}
			if len(lookupCodes) != 2 {
				t.Fatalf("lookupCodes = %+v, want 2 entries", lookupCodes)
			}
			if spuName != "SPU-PUBLISH" {
				t.Fatalf("spuName = %q, want SPU-PUBLISH", spuName)
			}
			if !defaultConfirmed {
				t.Fatal("defaultConfirmed = false, want true")
			}
			if fallbackMessage != "SHEIN accepted publish request; remote record not yet visible" {
				t.Fatalf("fallbackMessage = %q, want publish fallback", fallbackMessage)
			}
			if !gotStartedAt.Equal(startedAt) {
				t.Fatalf("startedAt = %v, want %v", gotStartedAt, startedAt)
			}
			if taskID != "task-123" {
				t.Fatalf("taskID = %q, want task-123", taskID)
			}
			return expected, nil
		},
	})

	confirmation, err := submitter.resolveSubmissionRefreshConfirmation("task-123", &sheinSubmissionRefreshState{
		action:           "publish",
		requestID:        "refresh-123",
		startedAt:        startedAt,
		lookupCodes:      []string{"SKC-1", "SKU-1"},
		defaultConfirmed: true,
		fallbackMessage:  "",
		productAPI:       productAPI,
		otherAPI:         otherAPI,
		spuName:          "SPU-PUBLISH",
	})
	if err != nil {
		t.Fatalf("resolveSubmissionRefreshConfirmation() error = %v", err)
	}
	if confirmation != expected {
		t.Fatalf("confirmation = %+v, want %+v", confirmation, expected)
	}
}

func TestApplySubmissionRefreshConfirmationAppliesEventParts(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now().Add(-time.Minute)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-apply-event",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}

	confirmation := &sheinRemoteConfirmation{
		remoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
		record: &sheinproduct.RecordItem{
			RecordID:     "record-123",
			SupplierCode: "SKC-1",
			State:        4,
			AuditState:   5,
		},
		checkedAt: now.Add(time.Minute),
		message:   "confirmed remotely",
		event: &sheinpub.SubmissionEvent{
			TaskID:         task.ID,
			Action:         "publish",
			Phase:          sheinpub.SubmissionPhaseConfirmRemote,
			Status:         sheinpub.SubmissionRemoteStatusConfirmed,
			RequestID:      "refresh-apply-event",
			StartedAt:      now,
			RemoteRecordID: "record-123",
			Detail:         "confirmed remotely",
		},
	}

	applySubmissionRefreshConfirmation(task.Result.Shein, "publish", "refresh-apply-event", confirmation)

	record := sheinSubmissionRecordForAction(task.Result.Shein.SubmissionState, "publish")
	if record == nil {
		t.Fatal("expected publish record")
	}
	if record.RemoteRecordID != "record-123" {
		t.Fatalf("remote record id = %q, want record-123", record.RemoteRecordID)
	}
	if record.RemoteState != 4 || record.RemoteAuditState != 5 {
		t.Fatalf("remote state = (%d,%d), want (4,5)", record.RemoteState, record.RemoteAuditState)
	}
	if got := len(task.Result.Shein.SubmissionEvents); got != 1 {
		t.Fatalf("submission events = %d, want 1", got)
	}
	if task.Result.Shein.SubmissionEvents[0].RemoteRecordID != "record-123" {
		t.Fatalf("event remote record id = %q, want record-123", task.Result.Shein.SubmissionEvents[0].RemoteRecordID)
	}
}

func TestApplySubmissionRefreshConfirmationWithoutEventSetsRemoteRecordOnly(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now().Add(-time.Minute)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-apply-record",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}

	applySubmissionRefreshConfirmation(task.Result.Shein, "publish", "refresh-apply-record", &sheinRemoteConfirmation{
		remoteStatus: sheinpub.SubmissionRemoteStatusPending,
		record: &sheinproduct.RecordItem{
			RecordID:     "record-only",
			SupplierCode: "SKC-1",
			State:        1,
			AuditState:   2,
		},
		checkedAt: now.Add(time.Minute),
		message:   "pending remotely",
	})

	record := sheinSubmissionRecordForAction(task.Result.Shein.SubmissionState, "publish")
	if record == nil {
		t.Fatal("expected publish record")
	}
	if record.RemoteRecordID != "record-only" {
		t.Fatalf("remote record id = %q, want record-only", record.RemoteRecordID)
	}
	if record.RemoteMessage != "pending remotely" {
		t.Fatalf("remote message = %q, want pending remotely", record.RemoteMessage)
	}
	if got := len(task.Result.Shein.SubmissionEvents); got != 0 {
		t.Fatalf("submission events = %d, want 0", got)
	}
}

func TestBuildSubmissionRefreshMutationRequestMapsStateAndConfirmation(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().Add(-time.Minute)
	confirmation := &sheinRemoteConfirmation{
		remoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
		message:      "confirmed remotely",
	}
	request, err := buildSubmissionRefreshMutationRequest("task-123", &sheinSubmissionRefreshState{
		action:    "publish",
		requestID: "refresh-123",
		startedAt: startedAt,
	}, confirmation)
	if err != nil {
		t.Fatalf("buildSubmissionRefreshMutationRequest() error = %v", err)
	}
	if request == nil {
		t.Fatal("request = nil")
	}
	if request.taskID != "task-123" {
		t.Fatalf("taskID = %q, want task-123", request.taskID)
	}
	if request.action != "publish" {
		t.Fatalf("action = %q, want publish", request.action)
	}
	if request.requestID != "refresh-123" {
		t.Fatalf("requestID = %q, want refresh-123", request.requestID)
	}
	if !request.startedAt.Equal(startedAt) {
		t.Fatalf("startedAt = %v, want %v", request.startedAt, startedAt)
	}
	if request.confirmation != confirmation {
		t.Fatalf("confirmation = %+v, want %+v", request.confirmation, confirmation)
	}
}

func TestBuildSubmissionRefreshValidationRequestMapsFields(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	request := buildSubmissionRefreshValidationRequest(task, "publish", "refresh-123")
	if request == nil {
		t.Fatal("request = nil")
	}
	if request.task != task {
		t.Fatalf("task = %+v, want original task", request.task)
	}
	if request.action != "publish" {
		t.Fatalf("action = %q, want publish", request.action)
	}
	if request.requestID != "refresh-123" {
		t.Fatalf("requestID = %q, want refresh-123", request.requestID)
	}
}

func TestApplySubmissionRefreshMutationAppendsRunningEventBeforeConfirmation(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beforeUpdatedAt := task.Result.UpdatedAt
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    startedAt,
		},
	}
	confirmation := &sheinRemoteConfirmation{
		remoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
		record: &sheinproduct.RecordItem{
			RecordID:     "record-123",
			SupplierCode: "SKC-1",
			State:        4,
			AuditState:   5,
		},
		checkedAt: startedAt.Add(time.Minute),
		message:   "confirmed remotely",
		event: &sheinpub.SubmissionEvent{
			TaskID:         task.ID,
			Action:         "publish",
			Phase:          sheinpub.SubmissionPhaseConfirmRemote,
			Status:         sheinpub.SubmissionRemoteStatusConfirmed,
			RequestID:      "refresh-123",
			StartedAt:      startedAt,
			RemoteRecordID: "record-123",
			Detail:         "confirmed remotely",
		},
	}

	request, err := buildSubmissionRefreshMutationRequest(task.ID, &sheinSubmissionRefreshState{
		action:    "publish",
		requestID: "refresh-123",
		startedAt: startedAt,
	}, confirmation)
	if err != nil {
		t.Fatalf("buildSubmissionRefreshMutationRequest() error = %v", err)
	}

	err = applySubmissionRefreshMutation(task, request)
	if err != nil {
		t.Fatalf("applySubmissionRefreshMutation() error = %v", err)
	}

	record := sheinSubmissionRecordForAction(task.Result.Shein.SubmissionState, "publish")
	if record == nil {
		t.Fatal("expected publish record")
	}
	if record.RemoteRecordID != "record-123" {
		t.Fatalf("remote record id = %q, want record-123", record.RemoteRecordID)
	}
	if !task.Result.UpdatedAt.After(beforeUpdatedAt) {
		t.Fatalf("updatedAt = %v, want after %v", task.Result.UpdatedAt, beforeUpdatedAt)
	}
	if got := len(task.Result.Shein.SubmissionEvents); got != 2 {
		t.Fatalf("submission events = %d, want running + confirmed events", got)
	}
	if task.Result.Shein.SubmissionEvents[0].Phase != sheinpub.SubmissionPhaseConfirmRemote || task.Result.Shein.SubmissionEvents[0].Status != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("confirm event = %+v, want confirm_remote/confirmed", task.Result.Shein.SubmissionEvents[0])
	}
	if task.Result.Shein.SubmissionEvents[1].Phase != sheinpub.SubmissionPhaseConfirmRemote || task.Result.Shein.SubmissionEvents[1].Status != sheinpub.SubmissionStatusRunning {
		t.Fatalf("running event = %+v, want confirm_remote/running", task.Result.Shein.SubmissionEvents[1])
	}
	if task.Result.Shein.SubmissionEvents[0].RemoteRecordID != "record-123" {
		t.Fatalf("confirmation event remote record id = %q, want record-123", task.Result.Shein.SubmissionEvents[0].RemoteRecordID)
	}
}

func TestValidateSubmissionRefreshMutationRejectsMissingTaskResult(t *testing.T) {
	t.Parallel()

	task := &Task{ID: "task-no-result"}

	_, err := validateSubmissionRefreshMutation(task, "publish", "refresh-123")
	if err == nil {
		t.Fatal("err = nil, want validation error")
	}
	if !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("error = %v, want ErrSubmitBlocked", err)
	}
	if !apperrors.IsCode(err, apperrors.ErrCodeValidation) {
		t.Fatalf("error code = %q, want %q", apperrors.GetCode(err), apperrors.ErrCodeValidation)
	}
}

func TestValidateSubmissionRefreshActionRejectsMissingSubmissionState(t *testing.T) {
	t.Parallel()

	err := validateSubmissionRefreshAction(&SheinPackage{}, "publish")
	if err == nil {
		t.Fatal("err = nil, want validation error")
	}
	if !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("error = %v, want ErrSubmitBlocked", err)
	}
	if !apperrors.IsCode(err, apperrors.ErrCodeValidation) {
		t.Fatalf("error code = %q, want %q", apperrors.GetCode(err), apperrors.ErrCodeValidation)
	}
}

func TestValidateSubmissionRefreshMutationRejectsActionChange(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now().Add(-time.Minute)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "save_draft",
		SaveDraft: &sheinpub.SubmissionRecord{
			Action:       "save_draft",
			RequestID:    "refresh-save-draft",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}

	_, err := validateSubmissionRefreshMutation(task, "publish", "refresh-save-draft")
	if !errors.Is(err, core.ErrSubmitInProgress) {
		t.Fatalf("validateSubmissionRefreshMutation() error = %v, want core.ErrSubmitInProgress", err)
	}
}

func TestValidateSubmissionRefreshMutationRejectsRequestChange(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now().Add(-time.Minute)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction: "publish",
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			RequestID:    "refresh-original",
			SupplierCode: "SKC-1",
			StartedAt:    now,
		},
	}

	_, err := validateSubmissionRefreshMutation(task, "publish", "refresh-updated")
	if !errors.Is(err, core.ErrSubmitInProgress) {
		t.Fatalf("validateSubmissionRefreshMutation() error = %v, want core.ErrSubmitInProgress", err)
	}
}
