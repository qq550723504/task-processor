package httpapi

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/listingsubscription"
)

const (
	studioProductImageModule       = listingsubscription.ModuleStudio
	studioProductImageMetric       = "product_image_jobs"
	studioProductImageLedgerMetric = "product_image_jobs_succeeded"
)

type subscriptionStudioProductImageUsage struct {
	service *listingsubscription.Service
}

func studioProductImageUsageDependency(service *listingsubscription.Service) *subscriptionStudioProductImageUsage {
	if service == nil {
		return nil
	}
	return &subscriptionStudioProductImageUsage{service: service}
}

func (a *subscriptionStudioProductImageUsage) AuthorizeProductImageUsage(ctx context.Context, tenantID string, quantity int) error {
	if a == nil || a.service == nil {
		return listingsubscription.ErrSubscriptionRequired
	}
	if strings.TrimSpace(tenantID) == "" || quantity <= 0 {
		return fmt.Errorf("product image usage authorization requires tenant and positive quantity")
	}
	result, err := a.service.AuthorizeUsage(ctx, strings.TrimSpace(tenantID), studioProductImageModule, studioProductImageMetric, quantity)
	if err != nil {
		return err
	}
	if !result.Allowed {
		return listingsubscription.ErrSubscriptionRequired
	}
	return nil
}

func (a *subscriptionStudioProductImageUsage) RecordProductImageUsage(ctx context.Context, tenantID string, quantity int) error {
	if a == nil || a.service == nil {
		return listingsubscription.ErrSubscriptionRequired
	}
	if strings.TrimSpace(tenantID) == "" || quantity <= 0 {
		return fmt.Errorf("product image usage recording requires tenant and positive quantity")
	}
	_, err := a.service.RecordUsage(ctx, strings.TrimSpace(tenantID), studioProductImageModule, studioProductImageMetric, quantity)
	return err
}

func (a *subscriptionStudioProductImageUsage) StudioProductImageUsageReservationEnabled() bool {
	return a != nil && a.service != nil && a.service.HasUsageLedger()
}

func (a *subscriptionStudioProductImageUsage) ReserveProductImageUsage(ctx context.Context, tenantID, reservationID string, quantity int) error {
	if a == nil || a.service == nil || !a.service.HasUsageLedger() {
		return listingsubscription.ErrUsageLedgerNotConfigured
	}
	tenantID = strings.TrimSpace(tenantID)
	reservationID = strings.TrimSpace(reservationID)
	if tenantID == "" || reservationID == "" || quantity <= 0 {
		return fmt.Errorf("product image usage reservation requires tenant, reservation, and positive quantity")
	}
	reservationKey := studioProductImageUsageIdempotencyKey(reservationID)
	if _, lookupErr := a.service.GetUsage(ctx, tenantID, reservationKey); lookupErr != nil && !errors.Is(lookupErr, listingsubscription.ErrUsageEventNotFound) {
		return lookupErr
	} else if errors.Is(lookupErr, listingsubscription.ErrUsageEventNotFound) {
		legacyGuard, err := a.service.AuthorizeUsage(ctx, tenantID, studioProductImageModule, studioProductImageMetric, quantity)
		if err != nil {
			return err
		}
		if !legacyGuard.Allowed {
			return listingsubscription.ErrSubscriptionQuotaExceed
		}
	}
	now := time.Now().UTC()
	result, err := a.service.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID:       tenantID,
		ModuleCode:     studioProductImageModule,
		Metric:         studioProductImageLedgerMetric,
		Quantity:       int64(quantity),
		PeriodKey:      now.Format("2006-01"),
		SourceType:     "listingkit_product_image",
		SourceID:       reservationID,
		IdempotencyKey: reservationKey,
		OccurredAt:     now,
	})
	if err != nil {
		return err
	}
	if result.Event.Status == listingsubscription.UsageEventReleased || result.Event.Status == listingsubscription.UsageEventReversed {
		return fmt.Errorf("product image usage reservation is no longer active")
	}
	if !result.Existing {
		if _, err := a.service.RecordUsage(ctx, tenantID, studioProductImageModule, studioProductImageMetric, quantity); err != nil {
			_, _ = a.service.ReleaseUsage(ctx, result.Event.EventID, "legacy_counter_mirror_failed")
			return err
		}
	}
	return nil
}

func (a *subscriptionStudioProductImageUsage) CommitProductImageUsage(ctx context.Context, tenantID, reservationID string) error {
	if a == nil || a.service == nil || !a.service.HasUsageLedger() {
		return listingsubscription.ErrUsageLedgerNotConfigured
	}
	event, err := a.service.GetUsage(ctx, strings.TrimSpace(tenantID), studioProductImageUsageIdempotencyKey(strings.TrimSpace(reservationID)))
	if errors.Is(err, listingsubscription.ErrUsageEventNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if event.Status == listingsubscription.UsageEventCommitted {
		return nil
	}
	if event.Status != listingsubscription.UsageEventReserved {
		return fmt.Errorf("product image usage commit requires a reserved event")
	}
	_, err = a.service.CommitUsage(ctx, event.EventID)
	return err
}

func (a *subscriptionStudioProductImageUsage) ReleaseProductImageUsage(ctx context.Context, tenantID, reservationID, reason string) error {
	if a == nil || a.service == nil || !a.service.HasUsageLedger() {
		return listingsubscription.ErrUsageLedgerNotConfigured
	}
	event, err := a.service.GetUsage(ctx, strings.TrimSpace(tenantID), studioProductImageUsageIdempotencyKey(strings.TrimSpace(reservationID)))
	if errors.Is(err, listingsubscription.ErrUsageEventNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if event.Status == listingsubscription.UsageEventReleased {
		return nil
	}
	if event.Status != listingsubscription.UsageEventReserved {
		return fmt.Errorf("product image usage release requires a reserved event")
	}
	if _, err = a.service.ReleaseUsage(ctx, event.EventID, strings.TrimSpace(reason)); err != nil {
		return err
	}
	if event.Quantity > 0 {
		_, err = a.service.RecordUsage(ctx, strings.TrimSpace(tenantID), studioProductImageModule, studioProductImageMetric, -int(event.Quantity))
	}
	return err
}

func studioProductImageUsageIdempotencyKey(reservationID string) string {
	reservationID = strings.TrimSpace(reservationID)
	key := "listingkit:studio_product_image:" + reservationID
	if len(key) <= 128 {
		return key
	}
	sum := sha256.Sum256([]byte(key))
	return "listingkit:studio_product_image:" + fmt.Sprintf("%x", sum[:])
}
