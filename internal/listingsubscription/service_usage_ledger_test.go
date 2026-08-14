package listingsubscription

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewServiceWithLedgerRejectsNilDependencies(t *testing.T) {
	repo := NewMemRepository()
	ledger := NewMemUsageLedger(repo)

	if _, err := NewServiceWithLedger(nil, ledger); err == nil {
		t.Fatal("NewServiceWithLedger(nil, ledger) error = nil, want repository validation error")
	}
	if _, err := NewServiceWithLedger(repo, nil); err == nil {
		t.Fatal("NewServiceWithLedger(repo, nil) error = nil, want ledger validation error")
	}
	var typedNilLedger UsageLedger = (*memUsageLedger)(nil)
	if _, err := NewServiceWithLedger(repo, typedNilLedger); err == nil {
		t.Fatal("NewServiceWithLedger(repo, typed nil ledger) error = nil, want ledger validation error")
	}
}

func TestNewServiceWithLedgerRejectsTypedNilRepositoryBeforeInitialization(t *testing.T) {
	var typedNilRepository Repository = (*MemRepository)(nil)
	ledger := NewMemUsageLedger(NewMemRepository())

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("NewServiceWithLedger() panicked for typed nil repository: %v", recovered)
		}
	}()
	if _, err := NewServiceWithLedger(typedNilRepository, ledger); err == nil {
		t.Fatal("NewServiceWithLedger(typed nil repository, ledger) error = nil, want repository validation error")
	}
}

func TestServiceWithLedgerDelegatesOnlyExplicitUsageLedgerMethods(t *testing.T) {
	ctx := context.Background()
	repo := NewMemRepository()
	ledger := NewMemUsageLedger(repo)
	svc, err := NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	seedServiceUsageLedgerEntitlement(t, svc)

	reservation, err := svc.ReserveUsage(ctx, serviceUsageLedgerInput())
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	committed, err := svc.CommitUsage(ctx, reservation.Event.EventID)
	if err != nil {
		t.Fatalf("CommitUsage() error = %v", err)
	}
	if committed.Status != UsageEventCommitted {
		t.Fatalf("CommitUsage() status = %q, want committed", committed.Status)
	}

	reversal, err := svc.ReverseUsage(ctx, committed.EventID, "service-reversal", "operator correction")
	if err != nil {
		t.Fatalf("ReverseUsage() error = %v", err)
	}
	if reversal.Status != UsageEventReversed || reversal.ReversalOf != committed.EventID {
		t.Fatalf("ReverseUsage() event = %#v, want reversal of %q", reversal, committed.EventID)
	}

	second, err := svc.ReserveUsage(ctx, serviceUsageLedgerInputWith("release-request", "job-43"))
	if err != nil {
		t.Fatalf("second ReserveUsage() error = %v", err)
	}
	released, err := svc.ReleaseUsage(ctx, second.Event.EventID, "business failure")
	if err != nil {
		t.Fatalf("ReleaseUsage() error = %v", err)
	}
	if released.Status != UsageEventReleased {
		t.Fatalf("ReleaseUsage() status = %q, want released", released.Status)
	}
}

func TestNewServiceKeepsLegacyCounterPathWhenLedgerIsNotConfigured(t *testing.T) {
	ctx := context.Background()
	svc, err := NewService(NewMemRepository())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := svc.ReserveUsage(ctx, serviceUsageLedgerInput()); !errors.Is(err, ErrUsageLedgerNotConfigured) {
		t.Fatalf("ReserveUsage() error = %v, want ErrUsageLedgerNotConfigured", err)
	}
	if _, err := svc.UpsertEntitlement(ctx, "tenant-17", ModuleStudio, EntitlementInput{
		Status: StatusActive,
		Limits: map[string]int{"design_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}

	result, err := svc.CheckUsage(ctx, "tenant-17", ModuleStudio, "design_jobs", 1)
	if err != nil {
		t.Fatalf("CheckUsage() error = %v", err)
	}
	if !result.Allowed || result.Used != 1 {
		t.Fatalf("CheckUsage() result = %#v, want legacy counter usage 1", result)
	}
}

func TestCommittedUsageHasOneRetryableOpenMeterOutboxIdentity(t *testing.T) {
	ctx := context.Background()
	repo := NewMemRepository()
	ledger := NewMemUsageLedger(repo)
	svc, err := NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	seedServiceUsageLedgerEntitlement(t, svc)

	reservation, err := svc.ReserveUsage(ctx, serviceUsageLedgerInput())
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	committed, err := svc.CommitUsage(ctx, reservation.Event.EventID)
	if err != nil {
		t.Fatalf("CommitUsage() error = %v", err)
	}

	first, err := svc.ListPendingUsageOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("ListPendingUsageOutbox() error = %v", err)
	}
	if len(first) != 1 || first[0].Destination != "openmeter" || first[0].EventID != committed.EventID {
		t.Fatalf("pending outbox = %#v, want one OpenMeter item for %q", first, committed.EventID)
	}
	second, err := svc.ListPendingUsageOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("retry ListPendingUsageOutbox() error = %v", err)
	}
	if len(second) != 1 || second[0].ID != first[0].ID || second[0].EventID != first[0].EventID {
		t.Fatalf("retry pending outbox = %#v, want same outbox identity %#v", second, first[0])
	}
}

func TestOpenMeterOutboxPayloadRejectsUnsafeMetadataWithoutChangingCommittedEvent(t *testing.T) {
	ctx := context.Background()
	repo := NewMemRepository()
	ledger := NewMemUsageLedger(repo)
	svc, err := NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	seedServiceUsageLedgerEntitlement(t, svc)
	input := serviceUsageLedgerInput()
	input.Metadata = map[string]string{"Authorization": "Bearer secret"}

	reservation, err := svc.ReserveUsage(ctx, input)
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	committed, err := svc.CommitUsage(ctx, reservation.Event.EventID)
	if err != nil {
		t.Fatalf("CommitUsage() error = %v", err)
	}
	if _, err := BuildOpenMeterUsageOutboxPayload(committed); !errors.Is(err, ErrUsageOutboxUnsafeMetadata) {
		t.Fatalf("BuildOpenMeterUsageOutboxPayload() error = %v, want ErrUsageOutboxUnsafeMetadata", err)
	}

	stored, err := ledger.Get(ctx, committed.TenantID, committed.IdempotencyKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Status != UsageEventCommitted || stored.EventID != committed.EventID {
		t.Fatalf("committed event after failed payload build = %#v, want committed %q", stored, committed.EventID)
	}
	pending, err := svc.ListPendingUsageOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("ListPendingUsageOutbox() error = %v", err)
	}
	if len(pending) != 1 || pending[0].EventID != committed.EventID {
		t.Fatalf("pending outbox after failed payload build = %#v, want %q", pending, committed.EventID)
	}
}

func TestBuildOpenMeterUsageOutboxPayloadContainsOnlyAdapterSafeFields(t *testing.T) {
	event := UsageEvent{
		EventID:    "usage-event-42",
		TenantID:   "tenant-17",
		ModuleCode: ModuleStudio,
		Metric:     "studio_design_jobs_succeeded",
		Quantity:   1,
		SourceType: "design_job",
		SourceID:   "job-42",
		Status:     UsageEventCommitted,
		OccurredAt: time.Date(2026, 8, 14, 2, 0, 0, 0, time.UTC),
	}

	payload, err := BuildOpenMeterUsageOutboxPayload(event)
	if err != nil {
		t.Fatalf("BuildOpenMeterUsageOutboxPayload() error = %v", err)
	}
	if payload.EventID != event.EventID || payload.TenantID != event.TenantID || payload.Metric != event.Metric || payload.Quantity != event.Quantity || !payload.OccurredAt.Equal(event.OccurredAt) {
		t.Fatalf("payload identity = %#v, want event identity and metering fields from %#v", payload, event)
	}
	wantMetadata := map[string]string{
		"module_code": event.ModuleCode,
		"source_id":   event.SourceID,
		"source_type": event.SourceType,
	}
	if len(payload.Metadata) != len(wantMetadata) {
		t.Fatalf("payload metadata = %#v, want only %#v", payload.Metadata, wantMetadata)
	}
	for key, want := range wantMetadata {
		if got := payload.Metadata[key]; got != want {
			t.Fatalf("payload metadata[%q] = %q, want %q", key, got, want)
		}
	}
}

func seedServiceUsageLedgerEntitlement(t *testing.T, svc *Service) {
	t.Helper()
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-17", ModuleStudio, EntitlementInput{
		Status: StatusActive,
		Limits: map[string]int{"studio_design_jobs_succeeded": 3},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
}

func serviceUsageLedgerInput() ReserveUsageInput {
	return serviceUsageLedgerInputWith("request-42", "job-42")
}

func serviceUsageLedgerInputWith(idempotencyKey, sourceID string) ReserveUsageInput {
	return ReserveUsageInput{
		TenantID:       "tenant-17",
		ModuleCode:     ModuleStudio,
		Metric:         "studio_design_jobs_succeeded",
		Quantity:       1,
		PeriodKey:      "2026-08",
		SourceType:     "design_job",
		SourceID:       sourceID,
		IdempotencyKey: idempotencyKey,
		OccurredAt:     time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC),
	}
}
