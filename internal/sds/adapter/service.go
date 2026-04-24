package adapter

import (
	"context"
	"fmt"

	"task-processor/internal/productimage"
	"task-processor/internal/sds/workflow"
)

type imageService interface {
	CreateProcessTask(ctx context.Context, req *productimage.ImageProcessRequest) (*productimage.Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*productimage.TaskResult, error)
	ProcessImages(ctx context.Context, task *productimage.Task) (*productimage.ImageProcessResult, error)
}

type designWorkflow interface {
	SyncDesignFromProcessResult(ctx context.Context, input workflow.SyncInput, result *productimage.ImageProcessResult) (*workflow.SyncResult, error)
}

// Service 负责把 productimage 服务与 SDS workflow 串起来。
type Service struct {
	images   imageService
	workflow designWorkflow
}

// NewService 创建 SDS 适配服务。
func NewService(images imageService, wf *workflow.Service) *Service {
	return &Service{
		images:   images,
		workflow: wf,
	}
}

func newServiceWithDeps(images imageService, wf designWorkflow) *Service {
	return &Service{
		images:   images,
		workflow: wf,
	}
}

// SyncFromImageRequest 创建图片处理任务、同步执行并保存到 SDS。
func (s *Service) SyncFromImageRequest(ctx context.Context, input SyncFromImageRequestInput) (*SyncResult, error) {
	if s.images == nil {
		return nil, fmt.Errorf("image service is not configured")
	}
	if input.ImageRequest == nil {
		return nil, fmt.Errorf("image request is required")
	}

	task, err := s.images.CreateProcessTask(productimage.WithInlineTaskExecution(ctx), input.ImageRequest)
	if err != nil {
		return nil, err
	}

	result, err := s.images.ProcessImages(ctx, task)
	if err != nil {
		return nil, err
	}

	syncResult, err := s.syncResult(ctx, input.SyncInput, result)
	if err != nil {
		return nil, err
	}

	return &SyncResult{
		ImageTask:   task,
		ImageResult: result,
		DesignSync:  syncResult,
	}, nil
}

// SyncFromExistingImageTask 读取已有图片任务结果并同步到 SDS。
func (s *Service) SyncFromExistingImageTask(ctx context.Context, input workflow.SyncInput, taskID string) (*SyncResult, error) {
	if s.images == nil {
		return nil, fmt.Errorf("image service is not configured")
	}
	if taskID == "" {
		return nil, fmt.Errorf("taskID is required")
	}

	taskResult, err := s.images.GetTaskResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if taskResult == nil {
		return nil, fmt.Errorf("image task result is nil")
	}
	if taskResult.Result == nil {
		return nil, fmt.Errorf("image task %s has no result yet", taskID)
	}

	syncResult, err := s.syncResult(ctx, input, taskResult.Result)
	if err != nil {
		return nil, err
	}

	return &SyncResult{
		ImageTask: &productimage.Task{
			ID:     taskID,
			Status: taskResult.Status,
			Result: taskResult.Result,
			Error:  taskResult.Error,
		},
		ImageResult: taskResult.Result,
		DesignSync:  syncResult,
	}, nil
}

// SyncFromImageResult 直接使用现成的 productimage 结果同步到 SDS。
func (s *Service) SyncFromImageResult(ctx context.Context, input workflow.SyncInput, result *productimage.ImageProcessResult) (*SyncResult, error) {
	if result == nil {
		return nil, fmt.Errorf("image result is required")
	}

	syncResult, err := s.syncResult(ctx, input, result)
	if err != nil {
		return nil, err
	}

	return &SyncResult{
		ImageResult: result,
		DesignSync:  syncResult,
	}, nil
}

func (s *Service) syncResult(ctx context.Context, input workflow.SyncInput, result *productimage.ImageProcessResult) (*workflow.SyncResult, error) {
	if s.workflow == nil {
		return nil, fmt.Errorf("sds workflow is not configured")
	}
	return s.workflow.SyncDesignFromProcessResult(ctx, input, result)
}
