package studio

import (
	"context"
	"fmt"
)

type BatchRetryPrepareService[SourceDetail any, Item any, Result any] struct {
	loadDetail  func(context.Context, string) (*SourceDetail, error)
	selectItems func(*SourceDetail, []string) ([]Item, error)
	resetItems  func(context.Context, []Item) error
	loadResult  func(context.Context, string) (*Result, error)
}

type BatchRetryPrepareServiceConfig[SourceDetail any, Item any, Result any] struct {
	LoadDetail  func(context.Context, string) (*SourceDetail, error)
	SelectItems func(*SourceDetail, []string) ([]Item, error)
	ResetItems  func(context.Context, []Item) error
	LoadResult  func(context.Context, string) (*Result, error)
}

func NewBatchRetryPrepareService[SourceDetail any, Item any, Result any](
	config BatchRetryPrepareServiceConfig[SourceDetail, Item, Result],
) *BatchRetryPrepareService[SourceDetail, Item, Result] {
	return &BatchRetryPrepareService[SourceDetail, Item, Result]{
		loadDetail:  config.LoadDetail,
		selectItems: config.SelectItems,
		resetItems:  config.ResetItems,
		loadResult:  config.LoadResult,
	}
}

func (s *BatchRetryPrepareService[SourceDetail, Item, Result]) PrepareRetryItems(
	ctx context.Context,
	batchID string,
	itemIDs []string,
) (*Result, error) {
	if s == nil || s.loadDetail == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	if s.selectItems == nil || s.resetItems == nil || s.loadResult == nil {
		return nil, fmt.Errorf("studio batch retry prepare service is not configured")
	}

	detail, err := s.loadDetail(ctx, batchID)
	if err != nil {
		return nil, err
	}
	items, err := s.selectItems(detail, itemIDs)
	if err != nil {
		return nil, err
	}
	if err := s.resetItems(ctx, items); err != nil {
		return nil, err
	}
	return s.loadResult(ctx, batchID)
}
