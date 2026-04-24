package usecase

import (
	"context"
	"fmt"

	"task-processor/internal/productimage"
	"task-processor/internal/sds/adapter"
	"task-processor/internal/sds/client"
	"task-processor/internal/sds/design"
	"task-processor/internal/sds/workflow"
)

// Service 暴露 SDS 设计同步的正式用例入口。
type Service interface {
	SyncFromRemoteImage(ctx context.Context, input RemoteImageInput) (*workflow.SyncResult, error)
	SyncFromLocalFile(ctx context.Context, input LocalFileInput) (*workflow.SyncResult, error)
	SyncFromImageResult(ctx context.Context, input ImageResultInput) (*adapter.SyncResult, error)
	SyncFromImageRequest(ctx context.Context, input ImageRequestInput) (*adapter.SyncResult, error)
}

type workflowService interface {
	SyncDesignFromURL(ctx context.Context, input workflow.SyncInput, source workflow.ImageSource) (*workflow.SyncResult, error)
	SyncDesignFromFile(ctx context.Context, input workflow.SyncInput, source workflow.FileSource) (*workflow.SyncResult, error)
}

type adapterService interface {
	SyncFromImageResult(ctx context.Context, input workflow.SyncInput, result *productimage.ImageProcessResult) (*adapter.SyncResult, error)
	SyncFromImageRequest(ctx context.Context, input adapter.SyncFromImageRequestInput) (*adapter.SyncResult, error)
}

type service struct {
	workflow workflowService
	adapter  adapterService
}

// Config 表示 usecase 依赖配置。
type Config struct {
	SDSClient       *client.Client
	ImageService    productimage.Service
	WorkflowService *workflow.Service
	AdapterService  *adapter.Service
}

// NewService 创建 SDS 用例服务。
func NewService(cfg Config) (Service, error) {
	if cfg.AdapterService != nil {
		return &service{
			workflow: cfg.WorkflowService,
			adapter:  cfg.AdapterService,
		}, nil
	}

	wf := cfg.WorkflowService
	if wf == nil {
		if cfg.SDSClient == nil {
			return nil, fmt.Errorf("sds client is required")
		}
		wf = workflow.NewService(design.NewService(cfg.SDSClient))
	}

	adp := adapter.NewService(cfg.ImageService, wf)
	return &service{
		workflow: wf,
		adapter:  adp,
	}, nil
}

func (s *service) SyncFromRemoteImage(ctx context.Context, input RemoteImageInput) (*workflow.SyncResult, error) {
	if s.workflow == nil {
		return nil, fmt.Errorf("workflow service is not configured")
	}
	return s.workflow.SyncDesignFromURL(ctx, input.Sync, input.Image)
}

func (s *service) SyncFromLocalFile(ctx context.Context, input LocalFileInput) (*workflow.SyncResult, error) {
	if s.workflow == nil {
		return nil, fmt.Errorf("workflow service is not configured")
	}
	return s.workflow.SyncDesignFromFile(ctx, input.Sync, input.File)
}

func (s *service) SyncFromImageResult(ctx context.Context, input ImageResultInput) (*adapter.SyncResult, error) {
	if s.adapter == nil {
		return nil, fmt.Errorf("adapter service is not configured")
	}
	return s.adapter.SyncFromImageResult(ctx, input.Sync, input.ImageResult)
}

func (s *service) SyncFromImageRequest(ctx context.Context, input ImageRequestInput) (*adapter.SyncResult, error) {
	if s.adapter == nil {
		return nil, fmt.Errorf("adapter service is not configured")
	}
	return s.adapter.SyncFromImageRequest(ctx, adapter.SyncFromImageRequestInput{
		SyncInput:    input.Sync,
		ImageRequest: input.ImageRequest,
	})
}
