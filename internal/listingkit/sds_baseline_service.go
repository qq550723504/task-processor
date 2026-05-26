package listingkit

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/catalog/canonical"
)

type sdsBaselineService struct {
	repo Repository
}

func (s *service) sdsBaselineOrDefault() *sdsBaselineService {
	if s == nil {
		return &sdsBaselineService{}
	}
	return &sdsBaselineService{repo: s.repo}
}

func (b *sdsBaselineService) GetReadyBaseline(ctx context.Context, task *Task) (*canonical.Product, bool, error) {
	if b == nil || task == nil || task.Request == nil || task.Request.Options == nil {
		return nil, false, nil
	}
	cacheRepo, ok := b.repo.(SDSBaselineCacheRepository)
	if !ok {
		return nil, false, nil
	}
	sdsOptions := task.Request.Options.SDS
	tenantID := strings.TrimSpace(task.Request.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(task.TenantID)
	}
	baselineKey := sdsBaselineKey(tenantID, sdsOptions)
	if baselineKey == "" {
		return nil, false, nil
	}
	entry, err := cacheRepo.GetSDSBaselineCache(ctx, tenantID, baselineKey)
	if err != nil {
		return nil, false, err
	}
	if entry == nil || !strings.EqualFold(strings.TrimSpace(entry.Status), "ready") {
		return nil, false, nil
	}
	if entry.CanonicalProductBase == nil {
		return nil, false, fmt.Errorf("sds baseline %q is ready but missing canonical payload", baselineKey)
	}
	product, err := entry.CanonicalProduct()
	if err != nil {
		return nil, false, err
	}
	if product == nil {
		return nil, false, fmt.Errorf("sds baseline %q resolved to empty canonical product", baselineKey)
	}
	return product, true, nil
}

func (b *sdsBaselineService) GetReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {
	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	cacheRepo, ok := b.repo.(SDSBaselineCacheRepository)
	if !ok {
		return &SDSBaselineReadiness{
			Status: "failed",
			Reason: "SDS baseline cache repository is unavailable.",
		}, nil
	}
	tenantID := resolveSDSBaselineReadinessTenant(ctx, query.TenantID)
	baselineKey := sdsBaselineKey(tenantID, query.BaselineOptions())
	if baselineKey == "" {
		return nil, fmt.Errorf("unable to derive SDS baseline key from query")
	}

	readiness := &SDSBaselineReadiness{
		BaselineKey: baselineKey,
		Status:      "missing",
		Reason:      "No baseline cache entry exists for this SDS selection.",
	}
	entry, err := cacheRepo.GetSDSBaselineCache(ctx, tenantID, baselineKey)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return readiness, nil
	}

	status := strings.ToLower(strings.TrimSpace(entry.Status))
	switch status {
	case "ready":
		if entry.CanonicalProductBase == nil {
			readiness.Status = "failed"
			readiness.Reason = "Baseline cache entry is marked ready but missing canonical payload."
			return readiness, nil
		}
		product, productErr := entry.CanonicalProduct()
		if productErr != nil {
			readiness.Status = "failed"
			readiness.Reason = fmt.Sprintf("Baseline cache payload is invalid: %v", productErr)
			return readiness, nil
		}
		if product == nil {
			readiness.Status = "failed"
			readiness.Reason = "Baseline cache entry resolved to an empty canonical product."
			return readiness, nil
		}
		readiness.Status = "ready"
		readiness.Reason = ""
		return readiness, nil
	case "", "pending", "processing", "queued", "building":
		readiness.Reason = firstNonEmpty(
			fmt.Sprintf("Baseline cache is not ready yet (status: %s).", firstNonEmpty(status, "unknown")),
			readiness.Reason,
		)
		return readiness, nil
	default:
		readiness.Status = "failed"
		readiness.Reason = fmt.Sprintf("Baseline cache is not usable for grouped create (status: %s).", status)
		return readiness, nil
	}
}
