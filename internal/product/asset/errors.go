package asset

import "errors"

var (
	ErrInvalidApproval        = errors.New("invalid approved asset commit")
	ErrInvalidInventoryScope  = errors.New("invalid approved asset inventory scope")
	ErrApprovalConflict       = errors.New("approved asset commit conflicts with existing approval")
	ErrApprovedAssetsNotReady = errors.New("approved product assets are not ready")
	ErrRepositoryUnavailable  = errors.New("approved asset repository is unavailable")
	ErrRepositoryStateInvalid = errors.New("approved asset repository state is invalid")
)
