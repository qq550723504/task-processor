package listingkit

import (
	"context"
)

type StudioBatchService interface {
	GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error)
}

type StudioBatchDetail struct {
	Batch *StudioBatchRecord      `json:"batch,omitempty"`
	Items []StudioBatchItemDetail `json:"items,omitempty"`
}

type StudioBatchItemDetail struct {
	Item     StudioBatchItemRecord            `json:"item"`
	Attempts []StudioGenerationAttemptRecord  `json:"attempts,omitempty"`
	Designs  []StudioMaterializedDesignRecord `json:"designs,omitempty"`
}

type ApproveStudioBatchDesignsRequest struct {
	DesignIDs []string `json:"design_ids,omitempty"`
}

type taskStudioBatchServiceConfig struct {
	repo              StudioBatchRepository
	studioSessionRepo StudioSessionRepository
	generator         studioBatchGenerator
}

func (s *service) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().GetStudioBatchDetail(ctx, batchID)
}

func (s *service) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().ApproveStudioBatchDesigns(ctx, batchID, req)
}

func (s *service) taskStudioBatchOrDefault() *taskStudioBatchService {
	if s.taskStudioBatch != nil {
		return s.taskStudioBatch
	}
	s.taskStudioBatch = newTaskStudioBatchService(buildTaskStudioBatchServiceConfig(s))
	return s.taskStudioBatch
}

func buildTaskStudioBatchServiceConfig(s *service) taskStudioBatchServiceConfig {
	if s == nil {
		return taskStudioBatchServiceConfig{}
	}
	return taskStudioBatchServiceConfig{
		repo:              s.studioBatchRepo,
		studioSessionRepo: s.studioSessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: s.studioBatchRepo,
			execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				return ExecuteStudioDesignBatch(ctx, s, input)
			},
		}),
	}
}
