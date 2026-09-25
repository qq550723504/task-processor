package asset

import "context"

type ApprovedInventoryReader interface {
	GetApprovedInventory(context.Context, InventoryScope) (ApprovedAssetInventory, error)
}

// ApprovalCommitReader reads an immutable, exact action receipt. It does not
// consult the mutable current-inventory head or create an approval.
type ApprovalCommitReader interface {
	ReadApprovalCommit(context.Context, string, string) (ApprovalCommit, error)
}

type Repository interface {
	ApprovedInventoryReader
	CommitApproval(context.Context, ApprovalCommit) (ApprovalReceipt, error)
}
