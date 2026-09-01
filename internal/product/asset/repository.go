package asset

import "context"

type Repository interface {
	CommitApproval(context.Context, ApprovalCommit) (ApprovalReceipt, error)
	GetApprovedInventory(context.Context, InventoryScope) (ApprovedAssetInventory, error)
}
