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

func (b *sdsBaselineService) GetCachedBaseline(ctx context.Context, task *Task) (*canonical.Product, bool, error) {
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
	if entry == nil || !strings.EqualFold(strings.TrimSpace(entry.Status), SDSBaselineStatusBaselineCached) {
		return nil, false, nil
	}
	if entry.CanonicalProductBase == nil {
		return nil, false, fmt.Errorf("sds baseline %q is cached but missing canonical payload", baselineKey)
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
			CacheStatus:      SDSBaselineStatusFailed,
			ValidationStatus: SDSBaselineValidationStatusUnknown,
			ReasonCode:       SDSBaselineReasonCodeCacheRepositoryUnavailable,
			Status:           SDSBaselineStatusFailed,
			Reason:           "SDS baseline cache repository is unavailable.",
		}, nil
	}
	tenantID := resolveSDSBaselineReadinessTenant(ctx, query.TenantID)
	baselineKey := sdsBaselineKey(tenantID, query.BaselineOptions())
	if baselineKey == "" {
		return nil, fmt.Errorf("unable to derive SDS baseline key from query")
	}

	readiness := &SDSBaselineReadiness{
		BaselineKey:      baselineKey,
		CacheStatus:      SDSBaselineStatusMissing,
		ValidationStatus: SDSBaselineValidationStatusUnknown,
		ReasonCode:       SDSBaselineReasonCodeCacheUnavailable,
		Status:           SDSBaselineStatusMissing,
		Reason:           "No baseline cache entry exists for this SDS selection.",
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
	case SDSBaselineStatusBaselineCached:
		readiness.CacheStatus = SDSBaselineStatusBaselineCached
		readiness.ValidationStatus = normalizedSDSBaselineValidationStatus(entry.ValidationStatus)
		if entry.CanonicalProductBase == nil {
			readiness.CacheStatus = SDSBaselineStatusFailed
			readiness.Status = SDSBaselineStatusFailed
			readiness.ReasonCode = SDSBaselineReasonCodeCachePayloadMissing
			readiness.Reason = "Baseline cache entry is marked baseline_cached but missing canonical payload."
			return readiness, nil
		}
		product, productErr := entry.CanonicalProduct()
		if productErr != nil {
			readiness.CacheStatus = SDSBaselineStatusFailed
			readiness.Status = SDSBaselineStatusFailed
			readiness.ReasonCode = SDSBaselineReasonCodeCachePayloadInvalid
			readiness.Reason = fmt.Sprintf("Baseline cache payload is invalid: %v", productErr)
			return readiness, nil
		}
		if product == nil {
			readiness.CacheStatus = SDSBaselineStatusFailed
			readiness.Status = SDSBaselineStatusFailed
			readiness.ReasonCode = SDSBaselineReasonCodeCachePayloadEmpty
			readiness.Reason = "Baseline cache entry resolved to an empty canonical product."
			return readiness, nil
		}
		readiness.Status, readiness.ReasonCode, readiness.Reason = deriveSDSBaselineOverallStatus(
			readiness.ValidationStatus,
			strings.TrimSpace(entry.ValidationReasonCode),
			strings.TrimSpace(entry.ValidationReason),
		)
		return readiness, nil
	case "", "pending", "processing", "queued", "building":
		readiness.CacheStatus = firstNonEmpty(status, SDSBaselineStatusMissing)
		readiness.Reason = firstNonEmpty(
			fmt.Sprintf("Baseline cache is not available yet (status: %s).", firstNonEmpty(status, "unknown")),
			readiness.Reason,
		)
		return readiness, nil
	default:
		readiness.CacheStatus = SDSBaselineStatusFailed
		readiness.Status = SDSBaselineStatusFailed
		readiness.ReasonCode = SDSBaselineReasonCodeCacheUnavailable
		readiness.Reason = fmt.Sprintf("Baseline cache is not usable for grouped SDS create (status: %s).", status)
		return readiness, nil
	}
}

func normalizedSDSBaselineValidationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SDSBaselineValidationStatusReady:
		return SDSBaselineValidationStatusReady
	case SDSBaselineValidationStatusBlocked:
		return SDSBaselineValidationStatusBlocked
	case SDSBaselineValidationStatusFailed:
		return SDSBaselineValidationStatusFailed
	default:
		return SDSBaselineValidationStatusUnknown
	}
}

func deriveSDSBaselineOverallStatus(validationStatus string, validationReasonCode string, validationReason string) (string, string, string) {
	switch normalizedSDSBaselineValidationStatus(validationStatus) {
	case SDSBaselineValidationStatusReady:
		return SDSBaselineStatusReady, "", ""
	case SDSBaselineValidationStatusBlocked:
		return SDSBaselineStatusBlocked, validationReasonCode, firstNonEmpty(validationReason, "SDS baseline validation is blocked.")
	case SDSBaselineValidationStatusFailed:
		return SDSBaselineStatusFailed, validationReasonCode, firstNonEmpty(validationReason, "SDS baseline validation failed.")
	default:
		return SDSBaselineStatusBaselineCached, firstNonEmpty(validationReasonCode, ""), firstNonEmpty(validationReason, "")
	}
}
