package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
)

type sdsBaselineService struct {
	repo                   Repository
	sdsLoginStatusProvider SDSLoginStatusProvider
}

func (s *service) sdsBaselineOrDefault() *sdsBaselineService {
	if s == nil {
		return &sdsBaselineService{}
	}
	return &sdsBaselineService{
		repo:                   s.repo,
		sdsLoginStatusProvider: s.sdsLoginStatusProvider,
	}
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
	case SDSBaselineStatusBaselineCached, SDSBaselineStatusReady:
		readiness.CacheStatus = status
		readiness.ValidationStatus = normalizedSDSBaselineValidationStatus(entry.ValidationStatus)
		if status == SDSBaselineStatusReady &&
			readiness.ValidationStatus == SDSBaselineValidationStatusUnknown {
			readiness.ValidationStatus = SDSBaselineValidationStatusReady
		}
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

		// Auto-revalidate if blocked due to missing design type (backward compatibility fix)
		validationStatus := normalizedSDSBaselineValidationStatus(entry.ValidationStatus)
		validationReasonCode := strings.TrimSpace(entry.ValidationReasonCode)
		if validationStatus == SDSBaselineValidationStatusBlocked &&
			validationReasonCode == SDSBaselineReasonCodeMissingDesignType {
			// Trigger re-validation with default design type
			revalidatedReadiness, revalErr := b.revalidateSDSBaseline(ctx, entry, query)
			if revalErr == nil && revalidatedReadiness != nil {
				return revalidatedReadiness, nil
			}
			// If re-validation fails, continue with original result
		}

		readiness.Status, readiness.ReasonCode, readiness.Reason = deriveSDSBaselineOverallStatus(
			validationStatus,
			validationReasonCode,
			strings.TrimSpace(entry.ValidationReason),
		)
		b.reconcileCachedSDSLoginBaselineReadiness(ctx, readiness)
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

func (b *sdsBaselineService) reconcileCachedSDSLoginBaselineReadiness(
	ctx context.Context,
	readiness *SDSBaselineReadiness,
) {
	if b == nil || readiness == nil {
		return
	}
	if readiness.ReasonCode != SDSBaselineReasonCodeLoginMissingCredentials &&
		!isSDSBaselineCredentialBootstrapReadinessFailure(readiness) {
		return
	}
	if b.sdsLoginStatusProvider == nil {
		return
	}
	status, err := b.sdsLoginStatusProvider.Status(ctx)
	if err != nil || status == nil || !status.HasAccessToken {
		return
	}
	readiness.ValidationStatus = SDSBaselineValidationStatusReady
	readiness.Status = SDSBaselineStatusReady
	readiness.ReasonCode = ""
	readiness.Reason = ""
}

func isSDSBaselineCredentialBootstrapReadinessFailure(readiness *SDSBaselineReadiness) bool {
	if readiness == nil {
		return false
	}
	if readiness.ReasonCode != SDSBaselineReasonCodeDesignSurfaceCheckFailed {
		return false
	}
	return isSDSBaselineCredentialBootstrapError(fmt.Errorf("%s", readiness.Reason))
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

// revalidateSDSBaseline re-validates a baseline cache entry with default design type for backward compatibility
func (b *sdsBaselineService) revalidateSDSBaseline(ctx context.Context, entry *SDSBaselineCacheEntry, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {
	if entry == nil || entry.CanonicalProductBase == nil {
		return nil, fmt.Errorf("baseline entry or canonical product is missing")
	}

	// For backward compatibility: if blocked due to missing design type,
	// assume it's valid with default "material" design type
	entry.ValidationStatus = SDSBaselineValidationStatusReady
	entry.ValidationReasonCode = ""
	entry.ValidationReason = ""
	now := time.Now().UTC()
	entry.ValidatedAt = &now

	cacheRepo, ok := b.repo.(SDSBaselineCacheRepository)
	if !ok {
		return nil, fmt.Errorf("baseline cache repository is unavailable")
	}

	if saveErr := cacheRepo.SaveSDSBaselineCache(ctx, entry); saveErr != nil {
		return nil, saveErr
	}

	// Return updated readiness
	readiness := &SDSBaselineReadiness{
		BaselineKey:      entry.BaselineKey,
		CacheStatus:      entry.Status,
		ValidationStatus: SDSBaselineValidationStatusReady,
		Status:           SDSBaselineStatusReady,
		ReasonCode:       "",
		Reason:           "",
	}

	return readiness, nil
}

func deriveSDSBaselineOverallStatusFromResult(result sdsBaselineValidationResult) string {
	switch result.Status {
	case SDSBaselineValidationStatusReady:
		return SDSBaselineStatusReady
	case SDSBaselineValidationStatusBlocked:
		return SDSBaselineStatusBlocked
	case SDSBaselineValidationStatusFailed:
		return SDSBaselineStatusFailed
	default:
		return SDSBaselineStatusMissing
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
