package openmeter

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

const (
	pocStudioLimit       = "5"
	pocSheinLimit        = "3"
	pocStorageLimitBytes = "10485760"
)

type pocDerivedEntitlementEvidence struct {
	Usage      string
	Balance    string
	Overage    string
	HasAccess  bool
	FeatureKey string
}

func TestPoCEntitlementAccessTracksUsageThresholds(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantID := mustPoCTenantForSubject(t, fixture.Names.SubjectA)

	steps := []struct {
		name        string
		events      int
		wantUsage   string
		wantBalance string
		wantOverage string
		wantAccess  bool
	}{
		{name: "zero", wantUsage: "0", wantBalance: "5", wantOverage: "0", wantAccess: true},
		{name: "partial", events: 2, wantUsage: "2", wantBalance: "3", wantOverage: "0", wantAccess: true},
		{name: "exact limit", events: 3, wantUsage: "5", wantBalance: "0", wantOverage: "0", wantAccess: false},
		{name: "above limit", events: 1, wantUsage: "6", wantBalance: "-1", wantOverage: "1", wantAccess: false},
	}

	eventOffset := 0
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			ingestPoCCountEvents(t, fixture, client, tenantID, MetricStudioDesignJobsSucceeded, window, "entitlement-threshold", eventOffset, step.events)
			eventOffset += step.events
			waitForPoCEntitlementEvidence(t, client, fixture.Customers[0].ID, fixture.Names.StudioFeatureKey, fixture.Meters[0].ID, fixture.Names.SubjectA, window, pocStudioLimit, pocDerivedEntitlementEvidence{
				Usage:      step.wantUsage,
				Balance:    step.wantBalance,
				Overage:    step.wantOverage,
				HasAccess:  step.wantAccess,
				FeatureKey: fixture.Names.StudioFeatureKey,
			})
		})
	}
}

func TestPoCEntitlementsRemainTenantIsolated(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantA := mustPoCTenantForSubject(t, fixture.Names.SubjectA)
	tenantB := mustPoCTenantForSubject(t, fixture.Names.SubjectB)

	// A's exhausted entitlement must not consume B's independently attributed usage.
	ingestPoCCountEvents(t, fixture, client, tenantA, MetricSheinDraftsSucceeded, window, "entitlement-isolation-a", 0, 4)
	waitForPoCEntitlementEvidence(t, client, fixture.Customers[0].ID, fixture.Names.SheinFeatureKey, fixture.Meters[1].ID, fixture.Names.SubjectA, window, pocSheinLimit, pocDerivedEntitlementEvidence{
		Usage: "4", Balance: "-1", Overage: "1", HasAccess: false, FeatureKey: fixture.Names.SheinFeatureKey,
	})
	waitForPoCEntitlementEvidence(t, client, fixture.Customers[1].ID, fixture.Names.SheinFeatureKey, fixture.Meters[1].ID, fixture.Names.SubjectB, window, pocSheinLimit, pocDerivedEntitlementEvidence{
		Usage: "0", Balance: "3", Overage: "0", HasAccess: true, FeatureKey: fixture.Names.SheinFeatureKey,
	})

	// B can consume its own allowance without changing A's usage or access result.
	ingestPoCCountEvents(t, fixture, client, tenantB, MetricSheinDraftsSucceeded, window, "entitlement-isolation-b", 0, 2)
	waitForPoCEntitlementEvidence(t, client, fixture.Customers[1].ID, fixture.Names.SheinFeatureKey, fixture.Meters[1].ID, fixture.Names.SubjectB, window, pocSheinLimit, pocDerivedEntitlementEvidence{
		Usage: "2", Balance: "1", Overage: "0", HasAccess: true, FeatureKey: fixture.Names.SheinFeatureKey,
	})
	waitForPoCEntitlementEvidence(t, client, fixture.Customers[0].ID, fixture.Names.SheinFeatureKey, fixture.Meters[1].ID, fixture.Names.SubjectA, window, pocSheinLimit, pocDerivedEntitlementEvidence{
		Usage: "4", Balance: "-1", Overage: "1", HasAccess: false, FeatureKey: fixture.Names.SheinFeatureKey,
	})
}

func TestPoCStorageAccessRecoversAfterUsageDrops(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantID := mustPoCTenantForSubject(t, fixture.Names.SubjectA)

	snapshots := []struct {
		name        string
		quantity    string
		wantBalance string
		wantOverage string
		wantAccess  bool
	}{
		{name: "12 MiB exhausts the 10 MiB hard limit", quantity: "12582912", wantBalance: "-2097152", wantOverage: "2097152", wantAccess: false},
		{name: "3 MiB restores access", quantity: "3145728", wantBalance: "7340032", wantOverage: "0", wantAccess: true},
	}

	for index, snapshot := range snapshots {
		t.Run(snapshot.name, func(t *testing.T) {
			event := mustPoCUsageEvent(t, UsageFact{
				TenantID:   tenantID,
				Metric:     MetricStorageBytesCurrent,
				Quantity:   snapshot.quantity,
				SourceType: "storage_snapshot",
				SourceID:   fmt.Sprintf("poc-%s-entitlement-storage-%d", fixture.Environment.RunID, index),
				Revision:   fmt.Sprintf("snapshot-%d", index),
				OccurredAt: window.OccurredAt.Add(time.Duration(index) * time.Second),
			})
			mustPoCIngest(t, client, event)
			waitForPoCEntitlementEvidence(t, client, fixture.Customers[0].ID, fixture.Names.StorageFeatureKey, fixture.Meters[2].ID, fixture.Names.SubjectA, window, pocStorageLimitBytes, pocDerivedEntitlementEvidence{
				Usage: snapshot.quantity, Balance: snapshot.wantBalance, Overage: snapshot.wantOverage, HasAccess: snapshot.wantAccess, FeatureKey: fixture.Names.StorageFeatureKey,
			})
		})
	}
}

func TestPoCConcurrentAccessCheckDoesNotPromiseAtomicReservation(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantID := mustPoCTenantForSubject(t, fixture.Names.SubjectB)

	// Four committed units leave one unit. The experiment intentionally adds no
	// local lock or reservation around the subsequent check-then-ingest workers.
	ingestPoCCountEvents(t, fixture, client, tenantID, MetricStudioDesignJobsSucceeded, window, "concurrency-primer", 0, 4)
	waitForPoCEntitlementEvidence(t, client, fixture.Customers[1].ID, fixture.Names.StudioFeatureKey, fixture.Meters[0].ID, fixture.Names.SubjectB, window, pocStudioLimit, pocDerivedEntitlementEvidence{
		Usage: "4", Balance: "1", Overage: "0", HasAccess: true, FeatureKey: fixture.Names.StudioFeatureKey,
	})

	const workers = 20
	events := make([]openmeterapi.EventInput, workers)
	for index := range events {
		events[index] = mustPoCUsageEvent(t, UsageFact{
			TenantID:   tenantID,
			Metric:     MetricStudioDesignJobsSucceeded,
			Quantity:   "1",
			SourceType: "concurrent_committed_success",
			SourceID:   fmt.Sprintf("poc-%s-concurrent-access-%02d", fixture.Environment.RunID, index),
			Revision:   "committed",
			OccurredAt: window.OccurredAt.Add(time.Duration(index+10) * time.Millisecond),
		})
	}

	type workerResult struct {
		index     int
		sawAccess bool
		err       error
	}
	start := make(chan struct{})
	results := make(chan workerResult, workers)
	parentContext := t.Context()
	var ready sync.WaitGroup
	ready.Add(workers)
	for index, event := range events {
		go func(index int, event openmeterapi.EventInput) {
			ready.Done()
			<-start

			ctx, cancel := context.WithTimeout(parentContext, pocMeterPollTimeout)
			defer cancel()
			hasAccess, err := customerHasPoCFeatureAccess(ctx, client, fixture.Customers[1].ID, fixture.Names.StudioFeatureKey)
			if err == nil {
				err = client.Ingest(ctx, event)
			}
			results <- workerResult{index: index, sawAccess: hasAccess, err: err}
		}(index, event)
	}
	ready.Wait()
	close(start)

	sawAccess := 0
	workerErrors := make([]error, 0)
	for range workers {
		result := <-results
		if result.err != nil {
			workerErrors = append(workerErrors, fmt.Errorf("worker %d: %w", result.index, result.err))
			continue
		}
		if result.sawAccess {
			sawAccess++
		}
	}
	if err := errors.Join(workerErrors...); err != nil {
		t.Fatalf("concurrent access experiment errors: %v", err)
	}

	waitForPoCUsage(t, client, fixture.Meters[0].ID, fixture.Names.SubjectB, window.From, window.To, "24", 1)
	queryContext, cancel := context.WithTimeout(t.Context(), pocMeterPollTimeout)
	defer cancel()
	finalUsage, err := client.QueryUsage(queryContext, fixture.Meters[0].ID, fixture.Names.SubjectB, window.From, window.To)
	if err != nil {
		t.Fatalf("query exact final concurrent usage: %v", err)
	}
	if finalUsage != "24" {
		t.Fatalf("final concurrent usage = %q, want exactly 24", finalUsage)
	}

	// sawAccess is an observation, not an atomic-reservation assertion. OpenMeter
	// can answer several checks from the same pre-ingest state; this test records
	// that behavior but does not turn it into a business reservation guarantee.
	t.Logf("OpenMeter concurrency observation: %d/%d workers saw access before ingest; exact final usage=%s (no atomic reservation asserted)", sawAccess, workers, finalUsage)
}

func ingestPoCCountEvents(t *testing.T, fixture *pocFixture, client *Client, tenantID string, metric Metric, window pocContractWindow, scenario string, offset, count int) {
	t.Helper()
	for index := 0; index < count; index++ {
		eventIndex := offset + index
		mustPoCIngest(t, client, mustPoCUsageEvent(t, UsageFact{
			TenantID:   tenantID,
			Metric:     metric,
			Quantity:   "1",
			SourceType: "committed_success",
			SourceID:   fmt.Sprintf("poc-%s-%s-%d", fixture.Environment.RunID, scenario, eventIndex),
			Revision:   "committed",
			OccurredAt: window.OccurredAt.Add(time.Duration(eventIndex) * time.Second),
		}))
	}
}

func mustPoCTenantForSubject(t *testing.T, subject string) string {
	t.Helper()
	tenantID, err := tenantIDFromSubject(subject)
	if err != nil {
		t.Fatalf("derive tenant from fixture subject %q: %v", subject, err)
	}
	return tenantID
}

func waitForPoCEntitlementEvidence(t *testing.T, client *Client, customerID, featureKey, meterID, subject string, window pocContractWindow, limit string, want pocDerivedEntitlementEvidence) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), pocMeterPollTimeout)
	defer cancel()
	ticker := time.NewTicker(pocMeterPollInterval)
	defer ticker.Stop()

	lastResult := "no entitlement query completed"
	for {
		got, err := queryPoCEntitlementEvidence(ctx, client, customerID, featureKey, meterID, subject, window, limit)
		if err == nil {
			lastResult = fmt.Sprintf("usage=%s locally-derived-balance=%s locally-derived-overage=%s hasAccess=%t", got.Usage, got.Balance, got.Overage, got.HasAccess)
			if got == want {
				t.Logf("OpenMeter entitlement evidence for %s: usage=%s hasAccess=%t; locally-derived balance=%s overage=%s (not native v3 access fields)", featureKey, got.Usage, got.HasAccess, got.Balance, got.Overage)
				return
			}
		} else {
			lastResult = "error: " + err.Error()
		}

		select {
		case <-ctx.Done():
			t.Fatalf("OpenMeter entitlement evidence for customer %q feature %q did not become %+v within %s; last result: %s", customerID, featureKey, want, pocMeterPollTimeout, lastResult)
		case <-ticker.C:
		}
	}
}

func queryPoCEntitlementEvidence(ctx context.Context, client *Client, customerID, featureKey, meterID, subject string, window pocContractWindow, limit string) (pocDerivedEntitlementEvidence, error) {
	usage, err := client.QueryUsage(ctx, meterID, subject, window.From, window.To)
	if err != nil {
		return pocDerivedEntitlementEvidence{}, fmt.Errorf("query usage: %w", err)
	}
	hasAccess, err := customerHasPoCFeatureAccess(ctx, client, customerID, featureKey)
	if err != nil {
		return pocDerivedEntitlementEvidence{}, err
	}

	usageInteger, ok := new(big.Int).SetString(usage, 10)
	if !ok {
		return pocDerivedEntitlementEvidence{}, fmt.Errorf("usage %q is not a base-10 integer", usage)
	}
	limitInteger, ok := new(big.Int).SetString(limit, 10)
	if !ok {
		return pocDerivedEntitlementEvidence{}, fmt.Errorf("limit %q is not a base-10 integer", limit)
	}
	// The pinned v3 entitlement access endpoint does not return balance or
	// overage. These evidence-only values are derived locally with integer
	// decimal arithmetic and must never be described as native OpenMeter fields.
	balance := new(big.Int).Sub(new(big.Int).Set(limitInteger), usageInteger)
	overage := new(big.Int)
	if usageInteger.Cmp(limitInteger) > 0 {
		overage.Sub(new(big.Int).Set(usageInteger), limitInteger)
	}

	return pocDerivedEntitlementEvidence{
		Usage:      usageInteger.String(),
		Balance:    balance.String(),
		Overage:    overage.String(),
		HasAccess:  hasAccess,
		FeatureKey: featureKey,
	}, nil
}

func customerHasPoCFeatureAccess(ctx context.Context, client *Client, customerID, featureKey string) (bool, error) {
	access, err := client.ListCustomerAccess(ctx, customerID)
	if err != nil {
		return false, fmt.Errorf("list customer access: %w", err)
	}
	matched := 0
	hasAccess := false
	for _, result := range access {
		if result.FeatureKey != featureKey {
			continue
		}
		matched++
		if result.Type != openmeterapi.EntitlementTypeMetered {
			return false, fmt.Errorf("feature %q entitlement type = %q, want metered", featureKey, result.Type)
		}
		if result.Config != nil {
			return false, fmt.Errorf("metered feature %q unexpectedly exposed static config", featureKey)
		}
		hasAccess = result.HasAccess
	}
	if matched != 1 {
		return false, fmt.Errorf("customer %q access contained %d results for feature %q, want exactly one", customerID, matched, featureKey)
	}
	return hasAccess, nil
}
