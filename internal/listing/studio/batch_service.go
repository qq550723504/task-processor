package studio

import (
	"context"
	"fmt"
	"strings"
)

type BatchService[Detail any, ApproveRequest any, RetryRequest any] struct {
	getDetail         func(context.Context, string) (*Detail, error)
	startGeneration   func(context.Context, string) (*Detail, error)
	prepareGeneration func(context.Context, string) (*Detail, error)
	resumeGeneration  func(context.Context, string) (*Detail, error)
	approveDesigns    func(context.Context, string, []string) (*Detail, error)
	approvedDesignIDs func(*ApproveRequest) []string
	retryItems        func(context.Context, string, []string) (*Detail, error)
	prepareRetryItems func(context.Context, string, []string) (*Detail, error)
	retryItemIDs      func(*RetryRequest) []string
}

type BatchServiceConfig[Detail any, ApproveRequest any, RetryRequest any] struct {
	GetDetail         func(context.Context, string) (*Detail, error)
	StartGeneration   func(context.Context, string) (*Detail, error)
	PrepareGeneration func(context.Context, string) (*Detail, error)
	ResumeGeneration  func(context.Context, string) (*Detail, error)
	ApproveDesigns    func(context.Context, string, []string) (*Detail, error)
	ApprovedDesignIDs func(*ApproveRequest) []string
	RetryItems        func(context.Context, string, []string) (*Detail, error)
	PrepareRetryItems func(context.Context, string, []string) (*Detail, error)
	RetryItemIDs      func(*RetryRequest) []string
}

func NewBatchService[Detail any, ApproveRequest any, RetryRequest any](
	config BatchServiceConfig[Detail, ApproveRequest, RetryRequest],
) *BatchService[Detail, ApproveRequest, RetryRequest] {
	return &BatchService[Detail, ApproveRequest, RetryRequest]{
		getDetail:         config.GetDetail,
		startGeneration:   config.StartGeneration,
		prepareGeneration: config.PrepareGeneration,
		resumeGeneration:  config.ResumeGeneration,
		approveDesigns:    config.ApproveDesigns,
		approvedDesignIDs: config.ApprovedDesignIDs,
		retryItems:        config.RetryItems,
		prepareRetryItems: config.PrepareRetryItems,
		retryItemIDs:      config.RetryItemIDs,
	}
}

func (s *BatchService[Detail, ApproveRequest, RetryRequest]) GetDetail(ctx context.Context, batchID string) (*Detail, error) {
	if s == nil || s.getDetail == nil {
		return nil, fmt.Errorf("studio batch detail service is not configured")
	}
	return s.getDetail(ctx, strings.TrimSpace(batchID))
}

func (s *BatchService[Detail, ApproveRequest, RetryRequest]) StartGeneration(ctx context.Context, batchID string) (*Detail, error) {
	if s == nil || s.startGeneration == nil {
		return nil, fmt.Errorf("studio batch generation service is not configured")
	}
	return s.startGeneration(ctx, strings.TrimSpace(batchID))
}

func (s *BatchService[Detail, ApproveRequest, RetryRequest]) PrepareGeneration(ctx context.Context, batchID string) (*Detail, error) {
	if s == nil || s.prepareGeneration == nil {
		return nil, fmt.Errorf("studio batch generation service is not configured")
	}
	return s.prepareGeneration(ctx, strings.TrimSpace(batchID))
}

func (s *BatchService[Detail, ApproveRequest, RetryRequest]) ResumeGeneration(ctx context.Context, batchID string) (*Detail, error) {
	if s == nil || s.resumeGeneration == nil {
		return nil, fmt.Errorf("studio batch generation service is not configured")
	}
	return s.resumeGeneration(ctx, strings.TrimSpace(batchID))
}

func (s *BatchService[Detail, ApproveRequest, RetryRequest]) ApproveDesigns(
	ctx context.Context,
	batchID string,
	req *ApproveRequest,
) (*Detail, error) {
	if s == nil || s.approveDesigns == nil {
		return nil, fmt.Errorf("studio batch review service is not configured")
	}
	var designIDs []string
	if s.approvedDesignIDs != nil {
		designIDs = s.approvedDesignIDs(req)
	}
	return s.approveDesigns(ctx, strings.TrimSpace(batchID), append([]string(nil), designIDs...))
}

func (s *BatchService[Detail, ApproveRequest, RetryRequest]) RetryItems(
	ctx context.Context,
	batchID string,
	req *RetryRequest,
) (*Detail, error) {
	if s == nil || s.retryItems == nil {
		return nil, fmt.Errorf("studio batch generation service is not configured")
	}
	var itemIDs []string
	if s.retryItemIDs != nil {
		itemIDs = s.retryItemIDs(req)
	}
	return s.retryItems(ctx, strings.TrimSpace(batchID), append([]string(nil), itemIDs...))
}

func (s *BatchService[Detail, ApproveRequest, RetryRequest]) PrepareRetryItems(
	ctx context.Context,
	batchID string,
	req *RetryRequest,
) (*Detail, error) {
	if s == nil || s.prepareRetryItems == nil {
		return nil, fmt.Errorf("studio batch retry prepare service is not configured")
	}
	var itemIDs []string
	if s.retryItemIDs != nil {
		itemIDs = s.retryItemIDs(req)
	}
	return s.prepareRetryItems(ctx, strings.TrimSpace(batchID), append([]string(nil), itemIDs...))
}
