package adapter

import (
	"context"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/sds/workflow"
)

// ApprovedAssetReader is the SDS-owned read port for approved product assets.
type ApprovedAssetReader interface {
	GetApprovedInventory(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error)
}

type designWorkflow interface {
	SyncDesignFromApprovedAssets(context.Context, workflow.SyncInput, productasset.ApprovedAssetInventory) (*workflow.SyncResult, error)
}

// Service reads approved product assets and delegates SDS design synchronization.
type Service struct {
	approvedAssets ApprovedAssetReader
	workflow       designWorkflow
}

// NewService creates the SDS approved-asset adapter.
func NewService(approvedAssets ApprovedAssetReader, wf *workflow.Service) *Service {
	return &Service{approvedAssets: approvedAssets, workflow: wf}
}

func newServiceWithDeps(approvedAssets ApprovedAssetReader, wf designWorkflow) *Service {
	return &Service{approvedAssets: approvedAssets, workflow: wf}
}

// SyncFromApprovedAssets reads the exact tenant/product inventory and refuses
// repository results that cross the requested scope.
func (s *Service) SyncFromApprovedAssets(ctx context.Context, input SyncFromApprovedAssetsInput) (*SyncResult, error) {
	if err := productasset.ValidateInventoryScope(input.Scope); err != nil {
		return nil, err
	}
	if s.approvedAssets == nil {
		return nil, productasset.ErrRepositoryUnavailable
	}
	inventory, err := s.approvedAssets.GetApprovedInventory(ctx, input.Scope)
	if err != nil {
		return nil, err
	}
	if inventory.Scope != input.Scope {
		return nil, productasset.ErrRepositoryStateInvalid
	}
	if s.workflow == nil {
		return nil, productasset.ErrRepositoryUnavailable
	}
	inventory = productasset.CloneApprovedAssetInventory(inventory)
	designSync, err := s.workflow.SyncDesignFromApprovedAssets(ctx, input.SyncInput, inventory)
	if err != nil {
		return nil, err
	}
	return &SyncResult{
		ApprovedAssets: productasset.CloneApprovedAssetInventory(inventory),
		DesignSync:     designSync,
	}, nil
}
