package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"task-processor/internal/listingsubscription"
)

const (
	studioProductImageLedgerMetric              = "product_image_jobs_succeeded"
	studioProductImageReleasePendingMetadataKey = "listingkit_api_release_pending"
)

func studioProductImageUsageLedgerEnabled(h *handler) bool {
	return h != nil && h.subscriptionService != nil && h.subscriptionService.HasUsageLedger()
}

func (h *handler) reserveStudioProductImageUsage(c *gin.Context, reservationID string) (string, error) {
	if !studioProductImageUsageLedgerEnabled(h) {
		return "", listingsubscription.ErrUsageLedgerNotConfigured
	}
	reservationID = strings.TrimSpace(reservationID)
	if reservationID == "" {
		reservationID = uuid.NewString()
	}
	if err := h.reconcileStudioProductImageUsageReleases(c.Request.Context()); err != nil {
		return "", err
	}
	requestTenant := strings.TrimSpace(requestTenantID(c))
	if requestTenant == "" {
		return "", fmt.Errorf("tenant id is required")
	}
	reserve := func(tenantID string) (listingsubscription.ReserveUsageResult, error) {
		now := time.Now().UTC()
		return h.subscriptionService.ReserveUsage(c.Request.Context(), listingsubscription.ReserveUsageInput{
			TenantID:       strings.TrimSpace(tenantID),
			ModuleCode:     listingsubscription.ModuleStudio,
			Metric:         studioProductImageLedgerMetric,
			Quantity:       1,
			PeriodKey:      now.Format("2006-01"),
			SourceType:     "listingkit_product_image",
			SourceID:       reservationID,
			IdempotencyKey: "listingkit:api:studio_product_image:" + reservationID,
			OccurredAt:     now,
		})
	}
	billingTenant, err := h.authorizeStudioProductImageLedgerTenant(c, requestTenant)
	if err != nil {
		return "", err
	}
	result, err := reserve(billingTenant)
	if errors.Is(err, listingsubscription.ErrSubscriptionRequired) {
		if legacyTenant, ok, resolveErr := resolveLegacySubscriptionTenantIDWithError(c, requestTenant); resolveErr != nil {
			return "", resolveErr
		} else if ok {
			result, err = reserve(legacyTenant)
			if err == nil {
				c.Set(subscriptionTenantContextKey, legacyTenant)
			}
		}
	}
	if err != nil {
		return "", err
	}
	c.Set(subscriptionTenantContextKey, result.Event.TenantID)
	return result.Event.EventID, nil
}

func (h *handler) authorizeStudioProductImageLedgerTenant(c *gin.Context, tenantID string) (string, error) {
	result, err := h.subscriptionService.AuthorizeUsage(c.Request.Context(), tenantID, listingsubscription.ModuleStudio, "product_image_jobs", 1)
	if err == nil && result.Allowed {
		return tenantID, nil
	}
	if err != nil && !errors.Is(err, listingsubscription.ErrSubscriptionRequired) {
		return "", err
	}
	if err == nil || result.Reason != "not_configured" {
		if err != nil {
			return "", err
		}
		return "", listingsubscription.ErrSubscriptionRequired
	}
	legacyTenant, ok, resolveErr := resolveLegacySubscriptionTenantIDWithError(c, tenantID)
	if resolveErr != nil {
		return "", resolveErr
	}
	if !ok {
		return "", listingsubscription.ErrSubscriptionRequired
	}
	fallback, fallbackErr := h.subscriptionService.AuthorizeUsage(c.Request.Context(), legacyTenant, listingsubscription.ModuleStudio, "product_image_jobs", 1)
	if fallbackErr != nil {
		return "", fallbackErr
	}
	if !fallback.Allowed {
		return "", listingsubscription.ErrSubscriptionRequired
	}
	return legacyTenant, nil
}

func (h *handler) reconcileStudioProductImageUsageReleases(ctx context.Context) error {
	events, err := h.subscriptionService.ListUsageEvents(ctx, 100)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.Status != listingsubscription.UsageEventReserved || event.SourceType != "listingkit_product_image" || event.Metric != studioProductImageLedgerMetric || event.Metadata[studioProductImageReleasePendingMetadataKey] != "1" {
			continue
		}
		if _, releaseErr := h.subscriptionService.ReleaseUsage(ctx, event.EventID, "retry_pending_api_release"); releaseErr != nil {
			return releaseErr
		}
	}
	return nil
}

func releaseStudioProductImageUsage(ctx context.Context, service *listingsubscription.Service, eventID, reason string) error {
	if strings.TrimSpace(eventID) == "" || service == nil {
		return nil
	}
	event, err := service.GetUsageEventByID(ctx, strings.TrimSpace(eventID))
	if err != nil {
		return err
	}
	if event == nil || event.Status != listingsubscription.UsageEventReserved {
		return nil
	}
	metadata := make(map[string]string, len(event.Metadata)+1)
	for key, value := range event.Metadata {
		metadata[key] = value
	}
	metadata[studioProductImageReleasePendingMetadataKey] = "1"
	if _, err := service.UpdateUsageMetadata(ctx, event.EventID, metadata); err != nil {
		return err
	}
	_, err = service.ReleaseUsage(ctx, event.EventID, strings.TrimSpace(reason))
	return err
}

func commitStudioProductImageUsage(ctx context.Context, service *listingsubscription.Service, eventID string) error {
	if strings.TrimSpace(eventID) == "" || service == nil {
		return nil
	}
	_, err := service.CommitUsage(ctx, strings.TrimSpace(eventID))
	return err
}

func writeStudioProductImageUsageAdmissionError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	var quotaErr *listingsubscription.UsageQuotaError
	if errors.As(err, &quotaErr) {
		writeQuotaExceeded(c, listingsubscription.GuardResult{
			ModuleCode: quotaErr.ModuleCode,
			Metric:     quotaErr.Metric,
			Limit:      usageLimitInt(quotaErr.Limit),
			Used:       int(quotaErr.CommittedUsage + quotaErr.ReservedUsage + quotaErr.Quantity),
			Reason:     "quota_exceeded",
		})
		return
	}
	if errors.Is(err, listingsubscription.ErrUsageQuotaExceeded) {
		writeQuotaExceeded(c, listingsubscription.GuardResult{ModuleCode: listingsubscription.ModuleStudio, Metric: studioProductImageLedgerMetric, Reason: "quota_exceeded"})
		return
	}
	if errors.Is(err, listingsubscription.ErrSubscriptionRequired) {
		writeSubscriptionRequired(c, listingsubscription.GuardResult{ModuleCode: listingsubscription.ModuleStudio, Metric: studioProductImageLedgerMetric, Reason: "not_configured"})
		return
	}
	c.JSON(500, gin.H{"error": "studio_product_images_admission_failed", "message": err.Error()})
}

func usageLimitInt(limit *int64) int {
	if limit == nil {
		return 0
	}
	return int(*limit)
}
