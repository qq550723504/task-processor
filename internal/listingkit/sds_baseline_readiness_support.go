package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (b *sdsBaselineService) reconcileCachedSDSLoginBaselineReadiness(
	ctx context.Context,
	readiness *SDSBaselineReadiness,
) {
	if b == nil || readiness == nil {
		return
	}
	if readiness.ReasonCode != SDSBaselineReasonCodeLoginMissingCredentials &&
		readiness.ReasonCode != SDSBaselineReasonCodeLoginInProgress &&
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
