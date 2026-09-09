package asset

import "context"

type ApprovedInventoryReader interface {
	GetApprovedInventory(context.Context, InventoryScope) (ApprovedAssetInventory, error)
}

type Repository interface {
	ApprovedInventoryReader
	CommitApproval(context.Context, ApprovalCommit) (ApprovalReceipt, error)
}
