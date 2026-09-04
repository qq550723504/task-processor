package adapter

import (
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/sds/workflow"
)

// SyncFromApprovedAssetsInput 表示从产品批准资产同步 SDS 的输入。
type SyncFromApprovedAssetsInput struct {
	SyncInput workflow.SyncInput
	Scope     productasset.InventoryScope
}

// SyncResult 表示适配层返回的完整上下文。
type SyncResult struct {
	ApprovedAssets productasset.ApprovedAssetInventory
	DesignSync     *workflow.SyncResult
}
