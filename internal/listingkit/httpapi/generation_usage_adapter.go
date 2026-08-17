package httpapi

import (
	"context"
	"errors"
	"strings"
	"time"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

type subscriptionGenerationUsage struct {
	service *listingsubscription.Service
}

func newSubscriptionGenerationUsage(service *listingsubscription.Service) *subscriptionGenerationUsage {
	if service == nil {
		return nil
	}
	return &subscriptionGenerationUsage{service: service}
}

func (a *subscriptionGenerationUsage) LookupGeneration(ctx context.Context, tenantID, taskID string) (listingkit.GenerationUsageEventState, bool, error) {
	event, err := a.lookup(ctx, tenantID, taskID)
	if errors.Is(err, listingsubscription.ErrUsageEventNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if event == nil {
		return "", false, errors.New("generation usage lookup returned nil event")
	}
	switch event.Status {
	case listingsubscription.UsageEventReserved:
		return listingkit.GenerationUsageEventReserved, true, nil
	case listingsubscription.UsageEventCommitted:
		return listingkit.GenerationUsageEventCommitted, true, nil
	case listingsubscription.UsageEventReleased:
		return listingkit.GenerationUsageEventReleased, true, nil
	case listingsubscription.UsageEventReversed:
		return listingkit.GenerationUsageEventReversed, true, nil
	default:
		return "", false, errors.New("generation usage lookup returned an unknown event status")
	}
}

func (a *subscriptionGenerationUsage) ReserveGeneration(ctx context.Context, tenantID, taskID string, occurredAt time.Time) (listingkit.GenerationUsageReservation, error) {
	if a == nil || a.service == nil {
		return listingkit.GenerationUsageReservation{}, listingsubscription.ErrUsageLedgerNotConfigured
	}
	fact := generationUsageFactForAdapter(tenantID, taskID, occurredAt)
	if fact.occurredAt.IsZero() {
		// A zero occurrence is the service's replay sentinel. It is valid only
		// when the deterministic event already exists; otherwise a previous
		// reserve failed before inserting and this fresh reservation must claim
		// the current period rather than send the invalid 0001-01 period.
		event, lookupErr := a.lookup(ctx, fact.tenantID, fact.sourceID)
		if errors.Is(lookupErr, listingsubscription.ErrUsageEventNotFound) {
			fact = generationUsageFactForAdapter(tenantID, taskID, time.Now().UTC())
		} else if lookupErr != nil {
			return listingkit.GenerationUsageReservation{}, lookupErr
		} else if event == nil {
			return listingkit.GenerationUsageReservation{}, errors.New("generation usage lookup returned nil event")
		}
	}
	result, err := a.service.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID:       fact.tenantID,
		ModuleCode:     fact.moduleCode,
		Metric:         fact.metric,
		Quantity:       fact.quantity,
		PeriodKey:      fact.occurredAt.UTC().Format("2006-01"),
		SourceType:     fact.sourceType,
		SourceID:       fact.sourceID,
		IdempotencyKey: fact.idempotencyKey,
		OccurredAt:     fact.occurredAt,
	})
	if err != nil {
		return listingkit.GenerationUsageReservation{}, err
	}
	return listingkit.GenerationUsageReservation{
		EventID:          result.Event.EventID,
		AlreadyCommitted: result.Event.Status == listingsubscription.UsageEventCommitted,
	}, nil
}

func (a *subscriptionGenerationUsage) CommitGeneration(ctx context.Context, tenantID, taskID string) error {
	event, err := a.lookup(ctx, tenantID, taskID)
	if err != nil {
		return err
	}
	if event.Status == listingsubscription.UsageEventCommitted {
		return nil
	}
	_, err = a.service.CommitUsage(ctx, event.EventID)
	return err
}

func (a *subscriptionGenerationUsage) ReleaseGeneration(ctx context.Context, tenantID, taskID, reason string) error {
	event, err := a.lookup(ctx, tenantID, taskID)
	if err != nil {
		return err
	}
	if event.Status == listingsubscription.UsageEventReleased {
		return nil
	}
	if event.Status != listingsubscription.UsageEventReserved {
		return errors.New("generation usage release requires a reserved event")
	}
	_, err = a.service.ReleaseUsage(ctx, event.EventID, strings.TrimSpace(reason))
	return err
}

type adapterGenerationUsageFact struct {
	tenantID       string
	moduleCode     string
	metric         string
	quantity       int64
	sourceType     string
	sourceID       string
	idempotencyKey string
	occurredAt     time.Time
}

func generationUsageFactForAdapter(tenantID, taskID string, occurredAt time.Time) adapterGenerationUsageFact {
	tenantID = strings.TrimSpace(tenantID)
	taskID = strings.TrimSpace(taskID)
	return adapterGenerationUsageFact{
		tenantID:       tenantID,
		moduleCode:     "studio",
		metric:         "studio_design_jobs_succeeded",
		quantity:       1,
		sourceType:     "listingkit_generation",
		sourceID:       taskID,
		idempotencyKey: "listingkit:generation:" + taskID,
		occurredAt:     occurredAt,
	}
}

func (a *subscriptionGenerationUsage) lookup(ctx context.Context, tenantID, taskID string) (*listingsubscription.UsageEvent, error) {
	if a == nil || a.service == nil {
		return nil, listingsubscription.ErrUsageLedgerNotConfigured
	}
	fact := generationUsageFactForAdapter(tenantID, taskID, time.Time{})
	return a.service.GetUsage(ctx, fact.tenantID, fact.idempotencyKey)
}
