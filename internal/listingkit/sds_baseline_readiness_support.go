package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	sdspod "task-processor/internal/product/sourcing/sdspod"
)

const sdsBaselineSupportedVersion = 1

type sdsBaselineReusableReadiness struct {
	Reusable         bool
	Product          *canonical.Product
	CacheStatus      string
	ValidationStatus string
	ReasonCode       string
	Reason           string
	Err              error
}

func evaluateSDSBaselineReusableReadiness(entry *SDSBaselineCacheEntry) sdsBaselineReusableReadiness {
	if entry == nil {
		return sdsBaselineReusableReadiness{
			CacheStatus:      SDSBaselineStatusMissing,
			ValidationStatus: SDSBaselineValidationStatusUnknown,
			ReasonCode:       SDSBaselineReasonCodeCacheUnavailable,
			Reason:           "No baseline cache entry exists for this SDS selection.",
		}
	}
	cacheStatus := normalizedSDSBaselineCacheStatus(entry.Status)
	validationStatus := normalizedSDSBaselineValidationStatus(entry.ValidationStatus)
	result := sdsBaselineReusableReadiness{
		CacheStatus:      cacheStatus,
		ValidationStatus: validationStatus,
	}
	if !isUsableSDSBaselineCacheStatus(cacheStatus) {
		result.ReasonCode = SDSBaselineReasonCodeCacheUnavailable
		result.Reason = fmt.Sprintf("Baseline cache is not usable for grouped SDS create (status: %s).", firstNonEmpty(cacheStatus, "unknown"))
		return result
	}
	if entry.Version != sdsBaselineSupportedVersion {
		result.ReasonCode = SDSBaselineReasonCodeCacheVersionUnsupported
		result.Reason = fmt.Sprintf("Baseline cache version %d is not supported.", entry.Version)
		return result
	}
	if entry.CanonicalProductBase == nil {
		result.ReasonCode = SDSBaselineReasonCodeCachePayloadMissing
		result.Reason = "Baseline cache entry is marked baseline_cached but missing canonical payload."
		return result
	}
	product, err := entry.CanonicalProduct()
	if err != nil {
		result.ReasonCode = SDSBaselineReasonCodeCachePayloadInvalid
		result.Reason = fmt.Sprintf("Baseline cache payload is invalid: %v", err)
		result.Err = err
		return result
	}
	if product == nil {
		result.ReasonCode = SDSBaselineReasonCodeCachePayloadEmpty
		result.Reason = "Baseline cache entry resolved to an empty canonical product."
		result.Err = fmt.Errorf("sds baseline %q resolved to empty canonical product", entry.BaselineKey)
		return result
	}
	decision := sdspod.EvaluateBaseline(sdspod.BaselineSnapshot{
		CacheStatus:          cacheStatus,
		Version:              entry.Version,
		PayloadState:         sdspod.BaselinePayloadPresent,
		ValidationStatus:     validationStatus,
		ValidationReasonCode: entry.ValidationReasonCode,
		ValidationReason:     entry.ValidationReason,
	})
	if !decision.Reusable {
		result.ReasonCode = decision.ReasonCode
		result.Reason = decision.Reason
		return result
	}
	result.Reusable = true
	result.Product = product
	return result
}

func normalizedSDSBaselineCacheStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func isUsableSDSBaselineCacheStatus(status string) bool {
	switch normalizedSDSBaselineCacheStatus(status) {
	case SDSBaselineStatusBaselineCached, SDSBaselineStatusReady:
		return true
	default:
		return false
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
		readiness.ReasonCode != SDSBaselineReasonCodeLoginInProgress &&
		readiness.ReasonCode != SDSBaselineReasonCodeLoginUnavailable &&
		!isSDSBaselineCredentialBootstrapReadinessFailure(readiness) {
		return
	}
	if b.sdsLoginStatusProvider == nil {
		return
	}
	status, err := b.sdsLoginStatusProvider.Status(ctx)
	if err != nil || status == nil || status.LoginInProgress || !status.HasAccessToken {
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

func (b *sdsBaselineService) revalidateSDSBaseline(ctx context.Context, entry *SDSBaselineCacheEntry, _ *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {
	if entry == nil || entry.CanonicalProductBase == nil {
		return nil, fmt.Errorf("baseline entry or canonical product is missing")
	}

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

	return &SDSBaselineReadiness{
		BaselineKey:      entry.BaselineKey,
		CacheStatus:      entry.Status,
		ValidationStatus: SDSBaselineValidationStatusReady,
		Status:           SDSBaselineStatusReady,
		ReasonCode:       "",
		Reason:           "",
	}, nil
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
