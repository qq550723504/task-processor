package usecase

import (
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/sds/workflow"
)

// SyncInput represents the SDS design synchronization parameters.
type SyncInput = workflow.SyncInput

// ApprovedAssetsInput identifies the approved product-asset inventory to use.
type ApprovedAssetsInput struct {
	Sync  SyncInput
	Scope productasset.InventoryScope
}
