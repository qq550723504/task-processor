package studio

import (
	"context"
	"fmt"
	"strings"
)

type BatchGenerationService[Detail any] struct {
	refreshGraph         func(context.Context, string) error
	ensureGraphForResume func(context.Context, string) error
	continueGeneration   func(context.Context, string) (*Detail, error)
	loadDetail           func(context.Context, string) (*Detail, error)
	prepareRetryItems    func(context.Context, string, []string) (*Detail, error)
}

type BatchGenerationServiceConfig[Detail any] struct {
	RefreshGraph         func(context.Context, string) error
	EnsureGraphForResume func(context.Context, string) error
	ContinueGeneration   func(context.Context, string) (*Detail, error)
	LoadDetail           func(context.Context, string) (*Detail, error)
	PrepareRetryItems    func(context.Context, string, []string) (*Detail, error)
}

func NewBatchGenerationService[Detail any](
	config BatchGenerationServiceConfig[Detail],
) *BatchGenerationService[Detail] {
	return &BatchGenerationService[Detail]{
		refreshGraph:         config.RefreshGraph,
		ensureGraphForResume: config.EnsureGraphForResume,
		continueGeneration:   config.ContinueGeneration,
		loadDetail:           config.LoadDetail,
		prepareRetryItems:    config.PrepareRetryItems,
	}
}

func (s *BatchGenerationService[Detail]) StartGeneration(ctx context.Context, batchID string) (*Detail, error) {
	if s == nil || s.refreshGraph == nil || s.continueGeneration == nil {
		return nil, fmt.Errorf("studio batch generation service is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	if err := s.refreshGraph(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	return s.continueGeneration(ctx, normalizedBatchID)
}

func (s *BatchGenerationService[Detail]) PrepareGeneration(ctx context.Context, batchID string) (*Detail, error) {
	if s == nil || s.refreshGraph == nil || s.loadDetail == nil {
		return nil, fmt.Errorf("studio batch generation service is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	if err := s.refreshGraph(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	return s.loadDetail(ctx, normalizedBatchID)
}

func (s *BatchGenerationService[Detail]) ResumeGeneration(ctx context.Context, batchID string) (*Detail, error) {
	if s == nil || s.ensureGraphForResume == nil || s.continueGeneration == nil {
		return nil, fmt.Errorf("studio batch generation service is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	if err := s.ensureGraphForResume(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	return s.continueGeneration(ctx, normalizedBatchID)
}

func (s *BatchGenerationService[Detail]) PrepareRetryItems(ctx context.Context, batchID string, itemIDs []string) (*Detail, error) {
	if s == nil || s.prepareRetryItems == nil {
		return nil, fmt.Errorf("studio batch retry prepare service is not configured")
	}
	return s.prepareRetryItems(ctx, strings.TrimSpace(batchID), append([]string(nil), itemIDs...))
}

func (s *BatchGenerationService[Detail]) RetryItems(ctx context.Context, batchID string, itemIDs []string) (*Detail, error) {
	if s == nil || s.prepareRetryItems == nil || s.continueGeneration == nil {
		return nil, fmt.Errorf("studio batch generation service is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	detail, err := s.prepareRetryItems(ctx, normalizedBatchID, append([]string(nil), itemIDs...))
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, fmt.Errorf("studio batch retry prepare service returned nil detail")
	}
	return s.continueGeneration(ctx, normalizedBatchID)
}
