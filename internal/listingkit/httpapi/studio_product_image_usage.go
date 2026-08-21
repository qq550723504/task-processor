package httpapi

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/listingsubscription"
	"task-processor/internal/tenantbridge"
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
	_, err := a.authorizeUsageTenant(ctx, tenantID, quantity)
	return err
}

func (a *subscriptionStudioProductImageUsage) RecordProductImageUsage(ctx context.Context, tenantID string, quantity int) error {
	if a == nil || a.service == nil {
		return listingsubscription.ErrSubscriptionRequired
	}
	if strings.TrimSpace(tenantID) == "" || quantity <= 0 {
		return fmt.Errorf("product image usage recording requires tenant and positive quantity")
	}
	billingTenant, err := a.authorizeUsageTenant(ctx, tenantID, quantity)
	if err != nil {
		return err
	}
	_, err = a.service.RecordUsage(ctx, billingTenant, studioProductImageModule, studioProductImageMetric, quantity)
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
	billingTenant, existingEvent, err := a.lookupProductImageUsageEvent(ctx, tenantID, reservationKey)
	if err != nil {
		return err
	}
	if existingEvent == nil {
		billingTenant, err = a.authorizeUsageTenant(ctx, tenantID, quantity)
		if err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	result, err := a.service.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID:       billingTenant,
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
		if _, err := a.service.RecordUsage(ctx, billingTenant, studioProductImageModule, studioProductImageMetric, quantity); err != nil {
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
	_, event, err := a.lookupProductImageUsageEvent(ctx, tenantID, studioProductImageUsageIdempotencyKey(strings.TrimSpace(reservationID)))
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
	_, event, err := a.lookupProductImageUsageEvent(ctx, tenantID, studioProductImageUsageIdempotencyKey(strings.TrimSpace(reservationID)))
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
	if event.Quantity > 0 {
		if _, err = a.service.RecordUsage(ctx, strings.TrimSpace(event.TenantID), studioProductImageModule, studioProductImageMetric, -int(event.Quantity)); err != nil {
			return err
		}
	}
	if _, err = a.service.ReleaseUsage(ctx, event.EventID, strings.TrimSpace(reason)); err != nil {
		if event.Quantity > 0 {
			_, rollbackErr := a.service.RecordUsage(ctx, strings.TrimSpace(event.TenantID), studioProductImageModule, studioProductImageMetric, int(event.Quantity))
			if rollbackErr != nil {
				return errors.Join(err, fmt.Errorf("restore legacy product image usage mirror: %w", rollbackErr))
			}
		}
		return err
	}
	return nil
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

func (a *subscriptionStudioProductImageUsage) authorizeUsageTenant(ctx context.Context, tenantID string, quantity int) (string, error) {
	canonical := strings.TrimSpace(tenantID)
	result, err := a.service.AuthorizeUsage(ctx, canonical, studioProductImageModule, studioProductImageMetric, quantity)
	if err == nil && result.Allowed {
		return canonical, nil
	}
	if err == nil || result.Reason != "not_configured" {
		if err != nil {
			return "", err
		}
		return "", listingsubscription.ErrSubscriptionRequired
	}
	legacyTenantID, resolveErr := tenantbridge.ResolveLegacyTenantID(ctx, canonical)
	if resolveErr != nil || legacyTenantID <= 0 || strconv.FormatInt(legacyTenantID, 10) == canonical {
		return "", err
	}
	legacyTenant := strconv.FormatInt(legacyTenantID, 10)
	fallback, fallbackErr := a.service.AuthorizeUsage(ctx, legacyTenant, studioProductImageModule, studioProductImageMetric, quantity)
	if fallbackErr != nil {
		return "", fallbackErr
	}
	if !fallback.Allowed {
		return "", listingsubscription.ErrSubscriptionRequired
	}
	return legacyTenant, nil
}

func (a *subscriptionStudioProductImageUsage) lookupProductImageUsageEvent(ctx context.Context, tenantID, idempotencyKey string) (string, *listingsubscription.UsageEvent, error) {
	canonical := strings.TrimSpace(tenantID)
	event, err := a.service.GetUsage(ctx, canonical, idempotencyKey)
	if err == nil {
		return canonical, event, nil
	}
	if !errors.Is(err, listingsubscription.ErrUsageEventNotFound) {
		return "", nil, err
	}
	legacyTenantID, resolveErr := tenantbridge.ResolveLegacyTenantID(ctx, canonical)
	if resolveErr != nil || legacyTenantID <= 0 || strconv.FormatInt(legacyTenantID, 10) == canonical {
		return canonical, nil, nil
	}
	legacyTenant := strconv.FormatInt(legacyTenantID, 10)
	event, err = a.service.GetUsage(ctx, legacyTenant, idempotencyKey)
	if errors.Is(err, listingsubscription.ErrUsageEventNotFound) {
		return canonical, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return legacyTenant, event, nil
}
