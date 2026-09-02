package usecase

import (
	"context"
	"fmt"

	"task-processor/internal/sds/adapter"
	"task-processor/internal/sds/client"
	"task-processor/internal/sds/design"
	"task-processor/internal/sds/workflow"
)

// Service exposes the approved-product-asset SDS synchronization use case.
type Service interface {
	SyncFromApprovedAssets(context.Context, ApprovedAssetsInput) (*adapter.SyncResult, error)
}

type adapterService interface {
	SyncFromApprovedAssets(context.Context, adapter.SyncFromApprovedAssetsInput) (*adapter.SyncResult, error)
}

type service struct {
	adapter adapterService
}

// Config defines use-case dependencies.
type Config struct {
	SDSClient       *client.Client
	ApprovedAssets  adapter.ApprovedAssetReader
	WorkflowService *workflow.Service
	AdapterService  *adapter.Service
}

// NewService creates the SDS use-case service.
func NewService(cfg Config) (Service, error) {
	if cfg.AdapterService != nil {
		return &service{adapter: cfg.AdapterService}, nil
	}

	wf := cfg.WorkflowService
	if wf == nil {
		if cfg.SDSClient == nil {
			return nil, fmt.Errorf("sds client is required")
		}
		wf = workflow.NewService(design.NewService(cfg.SDSClient))
	}

	return &service{adapter: adapter.NewService(cfg.ApprovedAssets, wf)}, nil
}

func (s *service) SyncFromApprovedAssets(ctx context.Context, input ApprovedAssetsInput) (*adapter.SyncResult, error) {
	if s.adapter == nil {
		return nil, fmt.Errorf("approved asset adapter is not configured")
	}
	return s.adapter.SyncFromApprovedAssets(ctx, adapter.SyncFromApprovedAssetsInput{
		SyncInput: input.Sync,
		Scope:     input.Scope,
	})
}
