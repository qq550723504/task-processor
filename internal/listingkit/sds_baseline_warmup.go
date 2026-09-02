package listingkit

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/product/catalog/canonical"
)

func (s *service) warmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if req.SDS == nil {
		return nil, fmt.Errorf("sds options cannot be nil")
	}
	query := &SDSBaselineReadinessQuery{
		TenantID:           strings.TrimSpace(req.TenantID),
		ParentProductID:    req.SDS.ParentProductID,
		PrototypeGroupID:   req.SDS.PrototypeGroupID,
		VariantID:          req.SDS.VariantID,
		SelectedVariantIDs: selectedVariantIDsFromOptions(req.SDS),
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if req.TenantID != "" {
		ctx = WithTenantID(ctx, req.TenantID)
	}
	if readiness, err := s.GetSDSBaselineReadiness(ctx, query); err == nil && readiness != nil &&
		readiness.CacheStatus == SDSBaselineStatusBaselineCached &&
		readiness.ValidationStatus != SDSBaselineValidationStatusUnknown {
		return readiness, nil
	}

	task := buildSDSBaselineWarmTask(ctx, req)
	product := newSDSBaselineCanonicalProduct(task.Request.Text, req.SDS)
	if product == nil {
		return nil, fmt.Errorf("failed to build SDS baseline canonical product")
	}
	if err := s.persistSDSBaselineCanonical(ctx, task, product); err != nil {
		return nil, err
	}
	if err := s.persistSDSBaselineValidation(ctx, task); err != nil {
		return nil, err
	}
	return s.GetSDSBaselineReadiness(ctx, query)
}

func (s *service) persistSDSBaselineIfEligible(ctx context.Context, task *Task) error {
	product := buildSDSBaselineCanonicalProduct(task)
	if product == nil {
		return nil
	}
	return s.persistSDSBaselineCanonical(ctx, task, product)
}

func (s *service) persistSDSBaselineCanonical(ctx context.Context, task *Task, product *canonical.Product) error {
	cacheRepo, ok := s.repo.(SDSBaselineCacheRepository)
	if !ok || task == nil || task.Request == nil || task.Request.Options == nil || task.Request.Options.SDS == nil || product == nil {
		return nil
	}
	tenantID := strings.TrimSpace(task.Request.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(task.TenantID)
	}
	baselineKey := sdsBaselineKey(tenantID, task.Request.Options.SDS)
	if baselineKey == "" {
		return nil
	}
	payload, err := newSDSBaselineCanonicalProductPayload(product)
	if err != nil {
		return err
	}
	return cacheRepo.SaveSDSBaselineCache(ctx, &SDSBaselineCacheEntry{
		TenantID:             tenantID,
		BaselineKey:          baselineKey,
		Status:               SDSBaselineStatusBaselineCached,
		Version:              1,
		SourceTaskID:         task.ID,
		Identity:             sdsBaselineIdentityFromOptions(task.Request.Options.SDS),
		CanonicalProductBase: payload,
	})
}

func buildSDSBaselineWarmTask(ctx context.Context, req *WarmSDSBaselineRequest) *Task {
	tenantID := strings.TrimSpace(req.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(TenantIDFromContext(ctx))
	}
	return &Task{
		ID:       "sds-baseline-warmup",
		TenantID: tenantID,
		Request: &GenerateRequest{
			TenantID:  tenantID,
			Text:      firstNonEmptyString(req.SDS.ProductName, req.SDS.ProductEnglishName),
			Platforms: []string{"shein"},
			Options: &GenerateOptions{
				SDS: req.SDS,
			},
		},
	}
}

func buildSDSBaselineCanonicalProduct(task *Task) *canonical.Product {
	if task == nil || task.Request == nil || task.Request.Options == nil || task.Request.Options.SDS == nil {
		return nil
	}
	return newSDSBaselineCanonicalProduct(
		firstNonEmptyString(task.Request.Options.SDS.ProductName, task.Request.Text, task.Request.Options.SDS.ProductEnglishName),
		task.Request.Options.SDS,
	)
}

func selectedVariantIDsFromOptions(options *SDSSyncOptions) []int64 {
	if options == nil {
		return nil
	}
	result := make([]int64, 0, len(options.Variants))
	for _, variant := range options.Variants {
		if variant.VariantID > 0 {
			result = append(result, variant.VariantID)
		}
	}
	return result
}
