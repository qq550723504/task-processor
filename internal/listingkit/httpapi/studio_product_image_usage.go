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
	legacyMirrorMetadataKey        = "listingkit_legacy_counter_mirror"
	legacyMirrorPending            = "pending"
	legacyMirrorSettled            = "settled"
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
		TenantID:          billingTenant,
		ModuleCode:        studioProductImageModule,
		Metric:            studioProductImageLedgerMetric,
		LegacyUsageMetric: studioProductImageMetric,
		Quantity:          int64(quantity),
		PeriodKey:         now.Format("2006-01"),
		SourceType:        "listingkit_product_image",
		SourceID:          reservationID,
		IdempotencyKey:    reservationKey,
		OccurredAt:        now,
	})
	if err != nil {
		return err
	}
	if result.Event.Status == listingsubscription.UsageEventReleased || result.Event.Status == listingsubscription.UsageEventReversed {
		return fmt.Errorf("product image usage reservation is no longer active")
	}
	if result.Existing {
		return a.reconcileLegacyMirror(ctx, result.Event, billingTenant)
	}
	if _, metadataErr := a.service.UpdateUsageMetadata(ctx, result.Event.EventID, map[string]string{legacyMirrorMetadataKey: legacyMirrorPending}); metadataErr != nil {
		return metadataErr
	}
	if _, _, err := a.service.RecordUsageForPeriodOnce(ctx, billingTenant, studioProductImageModule, studioProductImageMetric, result.Event.PeriodKey, quantity, legacyMirrorOperationKey(result.Event.EventID, "reserve")); err != nil {
		if _, releaseErr := a.service.ReleaseUsage(ctx, result.Event.EventID, "legacy_counter_mirror_failed"); releaseErr != nil {
			return errors.Join(err, fmt.Errorf("release product image usage after legacy mirror failure: %w", releaseErr))
		}
		return err
	}
	_, err = a.service.UpdateUsageMetadata(ctx, result.Event.EventID, map[string]string{legacyMirrorMetadataKey: legacyMirrorSettled})
	return err
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
	if event == nil {
		return nil
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
	if event == nil {
		return nil
	}
	billingTenant := strings.TrimSpace(event.TenantID)
	if event.Quantity > 0 && event.Metadata[legacyMirrorMetadataKey] != legacyMirrorSettled {
		if err := a.reconcileLegacyMirror(ctx, *event, billingTenant); err != nil {
			return err
		}
		event.Metadata = map[string]string{legacyMirrorMetadataKey: legacyMirrorSettled}
	}
	legacyMirrorSettledForEvent := event.Quantity > 0 && event.Metadata[legacyMirrorMetadataKey] == legacyMirrorSettled
	if event.Status == listingsubscription.UsageEventReleased {
		if !legacyMirrorSettledForEvent {
			return nil
		}
		_, _, err = a.service.RecordUsageForPeriodOnce(ctx, strings.TrimSpace(event.TenantID), studioProductImageModule, studioProductImageMetric, event.PeriodKey, -int(event.Quantity), legacyMirrorOperationKey(event.EventID, "release"))
		return err
	}
	if event.Status != listingsubscription.UsageEventReserved {
		return fmt.Errorf("product image usage release requires a reserved event")
	}
	if _, err = a.service.ReleaseUsage(ctx, event.EventID, strings.TrimSpace(reason)); err != nil {
		return err
	}
	if legacyMirrorSettledForEvent {
		_, _, err = a.service.RecordUsageForPeriodOnce(ctx, strings.TrimSpace(event.TenantID), studioProductImageModule, studioProductImageMetric, event.PeriodKey, -int(event.Quantity), legacyMirrorOperationKey(event.EventID, "release"))
		return err
	}
	return nil
}

func legacyMirrorOperationKey(eventID, operation string) string {
	return "listingkit:legacy_product_image_mirror:" + strings.TrimSpace(eventID) + ":" + strings.TrimSpace(operation)
}

func (a *subscriptionStudioProductImageUsage) reconcileLegacyMirror(ctx context.Context, event listingsubscription.UsageEvent, billingTenant string) error {
	if event.Quantity <= 0 || event.Metadata[legacyMirrorMetadataKey] == legacyMirrorSettled {
		return nil
	}
	if _, _, err := a.service.RecordUsageForPeriodOnce(ctx, strings.TrimSpace(billingTenant), studioProductImageModule, studioProductImageMetric, event.PeriodKey, int(event.Quantity), legacyMirrorOperationKey(event.EventID, "reserve")); err != nil {
		return err
	}
	_, err := a.service.UpdateUsageMetadata(ctx, event.EventID, map[string]string{legacyMirrorMetadataKey: legacyMirrorSettled})
	return err
}

func (a *subscriptionStudioProductImageUsage) RecordProductImageUsageOnce(ctx context.Context, tenantID string, quantity int, operationKey string) error {
	if a == nil || a.service == nil {
		return listingsubscription.ErrSubscriptionRequired
	}
	if strings.TrimSpace(tenantID) == "" || quantity <= 0 || strings.TrimSpace(operationKey) == "" {
		return fmt.Errorf("idempotent product image usage recording requires tenant, positive quantity, and operation key")
	}
	operationKey = strings.TrimSpace(operationKey)
	if exists, lookupErr := a.service.UsageOperationExists(ctx, operationKey); lookupErr != nil {
		return lookupErr
	} else if exists {
		_, _, err := a.service.RecordUsageForPeriodOnce(ctx, strings.TrimSpace(tenantID), studioProductImageModule, studioProductImageMetric, time.Now().UTC().Format("2006-01"), quantity, operationKey)
		return err
	}
	billingTenant, err := a.authorizeUsageTenant(ctx, tenantID, quantity)
	if err != nil {
		return err
	}
	_, _, err = a.service.RecordUsageForPeriodOnce(ctx, billingTenant, studioProductImageModule, studioProductImageMetric, time.Now().UTC().Format("2006-01"), quantity, operationKey)
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
	if resolveErr != nil {
		if errors.Is(resolveErr, tenantbridge.ErrLegacyTenantNotFound) {
			return "", listingsubscription.ErrSubscriptionRequired
		}
		return "", resolveErr
	}
	if legacyTenantID <= 0 || strconv.FormatInt(legacyTenantID, 10) == canonical {
		return "", listingsubscription.ErrSubscriptionRequired
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
	if resolveErr != nil {
		if errors.Is(resolveErr, tenantbridge.ErrLegacyTenantNotFound) {
			return canonical, nil, nil
		}
		return "", nil, resolveErr
	}
	if legacyTenantID <= 0 || strconv.FormatInt(legacyTenantID, 10) == canonical {
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
