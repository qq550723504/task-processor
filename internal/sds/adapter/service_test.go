package adapter

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
	"task-processor/internal/sds/workflow"
)

type stubImageService struct {
	createTask    *productimage.Task
	createErr     error
	processResult *productimage.ImageProcessResult
	processErr    error
	taskResult    *productimage.TaskResult
	taskErr       error
	lastRequest   *productimage.ImageProcessRequest
	lastTask      *productimage.Task
}

func (s *stubImageService) CreateProcessTask(_ context.Context, req *productimage.ImageProcessRequest) (*productimage.Task, error) {
	s.lastRequest = req
	return s.createTask, s.createErr
}

func (s *stubImageService) GetTaskResult(_ context.Context, _ string) (*productimage.TaskResult, error) {
	return s.taskResult, s.taskErr
}

func (s *stubImageService) ProcessImages(_ context.Context, task *productimage.Task) (*productimage.ImageProcessResult, error) {
	s.lastTask = task
	return s.processResult, s.processErr
}

type stubWorkflowService struct {
	lastInput  workflow.SyncInput
	lastResult *productimage.ImageProcessResult
	result     *workflow.SyncResult
	err        error
}

func (s *stubWorkflowService) SyncDesignFromProcessResult(_ context.Context, input workflow.SyncInput, result *productimage.ImageProcessResult) (*workflow.SyncResult, error) {
	s.lastInput = input
	s.lastResult = result
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &workflow.SyncResult{}, nil
}

func TestSyncFromImageRequestRunsInlineFlow(t *testing.T) {
	t.Parallel()

	imgSvc := &stubImageService{
		createTask: &productimage.Task{ID: "img-task-1"},
		processResult: &productimage.ImageProcessResult{
			WhiteBgImage: &productimage.ImageAsset{URL: "white.png"},
		},
	}
	wfSvc := &stubWorkflowService{result: &workflow.SyncResult{}}
	svc := newServiceWithDeps(imgSvc, wfSvc)

	result, err := svc.SyncFromImageRequest(context.Background(), SyncFromImageRequestInput{
		SyncInput: workflow.SyncInput{VariantID: 89764},
		ImageRequest: &productimage.ImageProcessRequest{
			ImageURLs:   []string{"https://example.com/a.jpg"},
			Marketplace: "amazon",
		},
	})
	if err != nil {
		t.Fatalf("SyncFromImageRequest() error = %v", err)
	}
	if result == nil || result.ImageTask == nil || result.ImageTask.ID != "img-task-1" {
		t.Fatalf("unexpected image task result: %+v", result)
	}
	if imgSvc.lastRequest == nil {
		t.Fatal("expected image request to be passed through")
	}
	if imgSvc.lastTask == nil || imgSvc.lastTask.ID != "img-task-1" {
		t.Fatalf("unexpected processed task: %+v", imgSvc.lastTask)
	}
	if wfSvc.lastInput.VariantID != 89764 {
		t.Fatalf("workflow variant id = %d, want 89764", wfSvc.lastInput.VariantID)
	}
}

func TestSyncFromExistingImageTaskUsesStoredResult(t *testing.T) {
	t.Parallel()

	imgResult := &productimage.ImageProcessResult{
		MainImage: &productimage.ImageAsset{URL: "main.jpg"},
	}
	imgSvc := &stubImageService{
		taskResult: &productimage.TaskResult{
			TaskID: "img-task-2",
			Status: productimage.TaskStatusCompleted,
			Result: imgResult,
		},
	}
	wfSvc := &stubWorkflowService{result: &workflow.SyncResult{}}
	svc := newServiceWithDeps(imgSvc, wfSvc)

	result, err := svc.SyncFromExistingImageTask(context.Background(), workflow.SyncInput{VariantID: 89765}, "img-task-2")
	if err != nil {
		t.Fatalf("SyncFromExistingImageTask() error = %v", err)
	}
	if result.ImageTask == nil || result.ImageTask.ID != "img-task-2" {
		t.Fatalf("unexpected image task: %+v", result.ImageTask)
	}
	if wfSvc.lastResult != imgResult {
		t.Fatalf("workflow received unexpected image result")
	}
}

func TestSyncFromExistingImageTaskRejectsMissingResult(t *testing.T) {
	t.Parallel()

	svc := newServiceWithDeps(&stubImageService{
		taskResult: &productimage.TaskResult{
			TaskID: "img-task-3",
			Status: productimage.TaskStatusPending,
		},
	}, &stubWorkflowService{})

	_, err := svc.SyncFromExistingImageTask(context.Background(), workflow.SyncInput{VariantID: 1}, "img-task-3")
	if err == nil {
		t.Fatal("expected error for missing image result")
	}
}

func TestSyncFromImageResultDelegatesToWorkflow(t *testing.T) {
	t.Parallel()

	imgResult := &productimage.ImageProcessResult{
		WhiteBgImage: &productimage.ImageAsset{URL: "white.jpg"},
	}
	wfSvc := &stubWorkflowService{result: &workflow.SyncResult{}}
	svc := newServiceWithDeps(nil, wfSvc)

	result, err := svc.SyncFromImageResult(context.Background(), workflow.SyncInput{VariantID: 89766}, imgResult)
	if err != nil {
		t.Fatalf("SyncFromImageResult() error = %v", err)
	}
	if result == nil || result.ImageResult != imgResult {
		t.Fatalf("unexpected sync result: %+v", result)
	}
	if wfSvc.lastInput.VariantID != 89766 {
		t.Fatalf("workflow variant id = %d, want 89766", wfSvc.lastInput.VariantID)
	}
}
