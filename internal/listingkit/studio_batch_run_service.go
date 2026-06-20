package listingkit

import (
	"context"
)

type CreateStudioBatchRunRequest struct {
	BatchIDs []string `json:"batch_ids"`
	Mode     string   `json:"mode,omitempty"`
}

type StudioBatchRunService interface {
	CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error)
	GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error)
	ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error)
	CancelStudioBatchRun(ctx context.Context, runID string) error
	RecoverStudioBatchRun(ctx context.Context, runID string) error
}
