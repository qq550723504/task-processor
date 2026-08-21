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

const studioProductImageLedgerMetric = "product_image_jobs_succeeded"

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
	result, err := reserve(requestTenant)
	if errors.Is(err, listingsubscription.ErrSubscriptionRequired) {
		if legacyTenant, ok := resolveLegacySubscriptionTenantID(c, requestTenant); ok {
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

func releaseStudioProductImageUsage(ctx context.Context, service *listingsubscription.Service, eventID, reason string) error {
	if strings.TrimSpace(eventID) == "" || service == nil {
		return nil
	}
	_, err := service.ReleaseUsage(ctx, strings.TrimSpace(eventID), strings.TrimSpace(reason))
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
