package listingsubscription

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func openStoreQuotaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedStoreQuotaSubscription(t *testing.T, repo *GormRepository, organizationID, planCode string, override int) {
	t.Helper()
	ctx := context.Background()
	if err := repo.UpsertDefaultPlans(ctx, DefaultPlans()); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertTenantSubscription(ctx, &TenantSubscription{TenantID: organizationID, PlanCode: planCode, Status: StatusActive}); err != nil {
		t.Fatal(err)
	}
	limits := map[string]int(nil)
	if override > 0 {
		limits = map[string]int{"store_count": override}
	}
	if _, err := repo.UpsertEntitlement(ctx, &Entitlement{TenantID: organizationID, ModuleCode: ModuleStoreManagement, Status: StatusActive, Limits: limits}); err != nil {
		t.Fatal(err)
	}
}

func TestStoreQuotaReserveReplayAndTransitions(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-a", PlanBasic, 2)
	ledger := NewGormStoreQuotaLedger(repo)
	fingerprint := strings.Repeat("a", 64)
	input := StoreQuotaReserveInput{OrganizationID: "org-a", RequestKey: uuid.NewString(), ActorSubject: "actor-1", RequestFingerprint: fingerprint}

	first, err := ledger.Reserve(context.Background(), input)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if first.Allocation.RequestFingerprint != fingerprint {
		t.Fatalf("stored request fingerprint = %q, want %q", first.Allocation.RequestFingerprint, fingerprint)
	}
	changed := input
	changed.RequestFingerprint = strings.Repeat("b", 64)
	if _, err := ledger.Reserve(context.Background(), changed); !errors.Is(err, ErrStoreQuotaIdentityMismatch) {
		t.Fatalf("changed request fingerprint error = %v, want identity mismatch", err)
	}
	second, err := ledger.Reserve(context.Background(), input)
	if err != nil {
		t.Fatalf("replay Reserve() error = %v", err)
	}
	if first.Existing || !second.Existing || first.AllocationID != second.AllocationID || first.StoreID != second.StoreID {
		t.Fatalf("reserve results = %#v, %#v; want exact durable replay", first, second)
	}

	transition := StoreQuotaTransitionInput{OrganizationID: "org-a", AllocationID: first.AllocationID, StoreID: first.StoreID, RequestKey: input.RequestKey, ActorSubject: "actor-1"}
	committed, err := ledger.Commit(context.Background(), transition)
	if err != nil || committed.Allocation.Status != StoreQuotaAllocated {
		t.Fatalf("Commit() = %#v, %v; want allocated", committed, err)
	}
	replayed, err := ledger.Commit(context.Background(), transition)
	if err != nil || !replayed.Existing {
		t.Fatalf("replay Commit() = %#v, %v; want idempotent result", replayed, err)
	}
	deallocated, err := ledger.Deallocate(context.Background(), transition)
	if err != nil || deallocated.Allocation.Status != StoreQuotaReleased {
		t.Fatalf("Deallocate() = %#v, %v; want released", deallocated, err)
	}
	summary, err := ledger.Summary(context.Background(), "org-a")
	if err != nil || summary.Committed != 0 || summary.Reserved != 0 || !summary.Allowed || summary.Limit == nil || *summary.Limit != 2 {
		t.Fatalf("Summary() = %#v, %v; want available 0/0 at limit 2", summary, err)
	}
}

func TestStoreQuotaReserveRequiresSubscriptionAndEnforcesLimit(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	ledger := NewGormStoreQuotaLedger(repo)
	_, err := ledger.Reserve(context.Background(), StoreQuotaReserveInput{OrganizationID: "org-none", RequestKey: uuid.NewString(), ActorSubject: "actor-1"})
	if !errors.Is(err, ErrSubscriptionRequired) {
		t.Fatalf("unconfigured Reserve() error = %v, want subscription required", err)
	}
	seedStoreQuotaSubscription(t, repo, "org-limit", PlanBasic, 1)
	for i := 0; i < 2; i++ {
		_, err = ledger.Reserve(context.Background(), StoreQuotaReserveInput{OrganizationID: "org-limit", RequestKey: uuid.NewString(), ActorSubject: "actor-1"})
		if i == 0 && err != nil {
			t.Fatalf("first Reserve() error = %v", err)
		}
		if i == 1 && !errors.Is(err, ErrStoreQuotaExceeded) {
			t.Fatalf("second Reserve() error = %v, want quota exceeded", err)
		}
	}
}

func TestStoreQuotaReservationIdentityAndOrganizationAreDurablyScoped(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-a", PlanBasic, 2)
	seedStoreQuotaSubscription(t, repo, "org-b", PlanBasic, 2)
	ledger := NewGormStoreQuotaLedger(repo)
	requestKey := uuid.NewString()
	first, err := ledger.Reserve(context.Background(), StoreQuotaReserveInput{OrganizationID: "org-a", RequestKey: requestKey, ActorSubject: "actor-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.GetByRequestKey(context.Background(), "org-b", requestKey); !errors.Is(err, ErrStoreQuotaNotFound) {
		t.Fatalf("cross-org lookup error = %v, want not found", err)
	}
	second, err := ledger.Reserve(context.Background(), StoreQuotaReserveInput{OrganizationID: "org-b", RequestKey: requestKey, ActorSubject: "actor-1"})
	if err != nil || first.AllocationID == second.AllocationID || first.StoreID == second.StoreID {
		t.Fatalf("cross-org reserve = %#v, %#v, %v; want independent allocations", first, second, err)
	}
	bad := StoreQuotaTransitionInput{OrganizationID: "org-a", AllocationID: first.AllocationID, StoreID: second.StoreID, RequestKey: requestKey, ActorSubject: "actor-1"}
	if _, err := ledger.Commit(context.Background(), bad); !errors.Is(err, ErrStoreQuotaIdentityMismatch) {
		t.Fatalf("mismatched Commit() error = %v, want identity mismatch", err)
	}
	summary, err := ledger.Summary(context.Background(), "org-a")
	if err != nil || summary.Reserved != 1 || summary.Committed != 0 {
		t.Fatalf("mismatch mutated org-a summary = %#v, %v", summary, err)
	}
}

func TestStoreQuotaSummaryUsesStableSubscriptionRequiredPolicy(t *testing.T) {
	cases := []struct {
		name         string
		entitlement  Entitlement
		subscription *TenantSubscription
	}{
		{"inactive entitlement", Entitlement{Status: StatusDisabled}, &TenantSubscription{PlanCode: PlanBasic, Status: StatusActive}},
		{"not started entitlement", Entitlement{Status: StatusActive, StartsAt: storeQuotaTimePointer(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))}, &TenantSubscription{PlanCode: PlanBasic, Status: StatusActive}},
		{"nonpositive explicit override", Entitlement{Status: StatusActive, Limits: map[string]int{"store_count": 0}}, &TenantSubscription{PlanCode: PlanBasic, Status: StatusActive}},
		{"inactive subscription fallback", Entitlement{Status: StatusActive}, &TenantSubscription{PlanCode: PlanBasic, Status: StatusDisabled}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openStoreQuotaTestDB(t)
			repo := NewGormRepository(db)
			if err := repo.UpsertDefaultPlans(context.Background(), DefaultPlans()); err != nil {
				t.Fatal(err)
			}
			organizationID := "org-policy"
			tc.entitlement.TenantID, tc.entitlement.ModuleCode = organizationID, ModuleStoreManagement
			if _, err := repo.UpsertEntitlement(context.Background(), &tc.entitlement); err != nil {
				t.Fatal(err)
			}
			tc.subscription.TenantID = organizationID
			if _, err := repo.UpsertTenantSubscription(context.Background(), tc.subscription); err != nil {
				t.Fatal(err)
			}
			summary, err := NewGormStoreQuotaLedger(repo).Summary(context.Background(), organizationID)
			if err != nil || summary.Allowed || summary.Limit != nil || summary.Reason != "subscription_required" {
				t.Fatalf("Summary() = %#v, %v", summary, err)
			}
		})
	}
}

func TestStoreQuotaEntitlementOverridePrecedesPlanLimit(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-override", PlanBasic, 3)
	ledger := NewGormStoreQuotaLedger(repo)
	for i := 0; i < 4; i++ {
		_, err := ledger.Reserve(context.Background(), StoreQuotaReserveInput{OrganizationID: "org-override", RequestKey: uuid.NewString(), ActorSubject: "actor-1"})
		if i < 3 && err != nil {
			t.Fatalf("reservation %d error = %v", i, err)
		}
		if i == 3 && !errors.Is(err, ErrStoreQuotaExceeded) {
			t.Fatalf("reservation %d error = %v, want quota exceeded", i, err)
		}
	}
}

func TestStoreQuotaMigrationIsRepeatableAndCreatesScopedIndexes(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	for _, table := range []string{"saas_store_quota_allocations", "saas_store_quota_buckets"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing migrated table %s", table)
		}
	}
	for _, index := range []string{"idx_saas_store_quota_org_request", "idx_saas_store_quota_org_store", "idx_saas_store_quota_org_status"} {
		if !db.Migrator().HasIndex(&storeQuotaAllocationRow{}, index) {
			t.Fatalf("missing allocation index %s", index)
		}
	}
	if !db.Migrator().HasColumn(&storeQuotaAllocationRow{}, "request_fingerprint") {
		t.Fatal("missing allocation request_fingerprint column")
	}
}

func TestStoreQuotaMigrationCreatesOnlyQuotaTablesAndIsRepeatable(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		if err := AutoMigrateStoreQuotaLedger(db); err != nil {
			t.Fatalf("AutoMigrateStoreQuotaLedger() attempt %d error = %v", attempt+1, err)
		}
	}

	for _, table := range []string{"saas_store_quota_allocations", "saas_store_quota_buckets"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing quota table %s", table)
		}
	}
	var createdTables []string
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`).Scan(&createdTables).Error; err != nil {
		t.Fatalf("list created tables: %v", err)
	}
	if !reflect.DeepEqual(createdTables, []string{"saas_store_quota_allocations", "saas_store_quota_buckets"}) {
		t.Fatalf("narrow quota migration tables = %v", createdTables)
	}
	for _, unrelated := range []string{"saas_modules", "saas_plans", "saas_tenant_subscriptions", "saas_usage_counters", "saas_usage_events", "saas_subscription_audit_logs"} {
		if db.Migrator().HasTable(unrelated) {
			t.Fatalf("narrow quota migration created unrelated table %s", unrelated)
		}
	}
}

func TestStoreQuotaMigrationRejectsNilDatabase(t *testing.T) {
	if err := AutoMigrateStoreQuotaLedger(nil); err == nil {
		t.Fatal("AutoMigrateStoreQuotaLedger(nil) accepted a nil database")
	}
}

func TestStoreQuotaTransitionPolicyPreservesAllocationHistory(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-a", PlanBasic, 2)
	ledger := NewGormStoreQuotaLedger(repo)
	reserve := func() (StoreQuotaReserveResult, StoreQuotaTransitionInput) {
		input := StoreQuotaReserveInput{OrganizationID: "org-a", RequestKey: uuid.NewString(), ActorSubject: "actor-1"}
		result, err := ledger.Reserve(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		return result, StoreQuotaTransitionInput{OrganizationID: input.OrganizationID, AllocationID: result.AllocationID, StoreID: result.StoreID, RequestKey: input.RequestKey, ActorSubject: input.ActorSubject}
	}
	reserved, releaseInput := reserve()
	if _, err := ledger.ReleaseReservation(context.Background(), releaseInput); err != nil {
		t.Fatal(err)
	}
	if again, err := ledger.ReleaseReservation(context.Background(), releaseInput); err != nil || !again.Existing {
		t.Fatalf("release replay = %#v, %v", again, err)
	}
	if _, err := ledger.Deallocate(context.Background(), releaseInput); !errors.Is(err, ErrStoreQuotaInvalidTransition) {
		t.Fatalf("deallocate uncommitted release = %v, want invalid transition", err)
	}
	if _, err := ledger.Commit(context.Background(), releaseInput); !errors.Is(err, ErrStoreQuotaInvalidTransition) {
		t.Fatalf("commit released reservation = %v, want invalid transition", err)
	}
	if reserved.AllocationID == "" {
		t.Fatal("reserve did not return allocation id")
	}
	_, allocatedInput := reserve()
	allocated, err := ledger.Commit(context.Background(), allocatedInput)
	if err != nil {
		t.Fatal(err)
	}
	if allocated.Allocation.AllocatedAt == nil {
		t.Fatal("commit did not retain allocated_at")
	}
	if _, err := ledger.ReleaseReservation(context.Background(), allocatedInput); !errors.Is(err, ErrStoreQuotaInvalidTransition) {
		t.Fatalf("release allocated row = %v, want invalid transition", err)
	}
	deallocated, err := ledger.Deallocate(context.Background(), allocatedInput)
	if err != nil {
		t.Fatal(err)
	}
	if deallocated.Allocation.AllocatedAt == nil || deallocated.Allocation.ReleasedAt == nil {
		t.Fatalf("deallocation erased durable allocation history: %#v", deallocated.Allocation)
	}
	if again, err := ledger.Deallocate(context.Background(), allocatedInput); err != nil || !again.Existing {
		t.Fatalf("deallocation replay = %#v, %v", again, err)
	}
}

func TestStoreQuotaSummaryUsesPlanFallbackAndKeepsReplayAfterExpiry(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-plan", PlanProfessional, 0)
	now := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	ledger := newGormStoreQuotaLedger(repo, func() time.Time { return now })
	input := StoreQuotaReserveInput{OrganizationID: "org-plan", RequestKey: uuid.NewString(), ActorSubject: "actor-1"}
	first, err := ledger.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := ledger.Summary(context.Background(), "org-plan")
	if err != nil || summary.Limit == nil || *summary.Limit != 5 || !summary.Allowed {
		t.Fatalf("plan-fallback summary = %#v, %v; want limit 5", summary, err)
	}
	expires := now.Add(-time.Second)
	if _, err := repo.UpsertEntitlement(context.Background(), &Entitlement{TenantID: "org-plan", ModuleCode: ModuleStoreManagement, Status: StatusActive, ExpiresAt: &expires}); err != nil {
		t.Fatal(err)
	}
	replay, err := ledger.Reserve(context.Background(), input)
	if err != nil || !replay.Existing || replay.StoreID != first.StoreID {
		t.Fatalf("expired replay = %#v, %v; want durable replay", replay, err)
	}
	summary, err = ledger.Summary(context.Background(), "org-plan")
	if err != nil || summary.Allowed || summary.Limit != nil || summary.Reason != "subscription_required" {
		t.Fatalf("expired summary = %#v, %v", summary, err)
	}
}

func TestStoreQuotaRejectsCorruptBucketWithoutPartialTransition(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-a", PlanBasic, 2)
	ledger := NewGormStoreQuotaLedger(repo)
	input := StoreQuotaReserveInput{OrganizationID: "org-a", RequestKey: uuid.NewString(), ActorSubject: "actor-1"}
	reservation, err := ledger.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA ignore_check_constraints = ON").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE saas_store_quota_buckets SET reserved = -1 WHERE organization_id = ?", "org-a").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA ignore_check_constraints = OFF").Error; err != nil {
		t.Fatal(err)
	}
	transition := StoreQuotaTransitionInput{OrganizationID: "org-a", AllocationID: reservation.AllocationID, StoreID: reservation.StoreID, RequestKey: input.RequestKey, ActorSubject: input.ActorSubject}
	if _, err := ledger.Commit(context.Background(), transition); !errors.Is(err, ErrStoreQuotaInvalidTransition) {
		t.Fatalf("corrupt Commit() error = %v, want invalid transition", err)
	}
	allocation, err := ledger.GetByRequestKey(context.Background(), "org-a", input.RequestKey)
	if err != nil || allocation.Status != StoreQuotaReserved {
		t.Fatalf("corrupt transition mutated allocation = %#v, %v", allocation, err)
	}
}

func TestStoreQuotaConcurrentLimitOneHasOneAdmission(t *testing.T) {
	db := openConcurrentStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-race", PlanBasic, 1)
	ledger := NewGormStoreQuotaLedger(repo)
	inputs := []StoreQuotaReserveInput{{OrganizationID: "org-race", RequestKey: uuid.NewString(), ActorSubject: "actor-a"}, {OrganizationID: "org-race", RequestKey: uuid.NewString(), ActorSubject: "actor-b"}}
	start := make(chan struct{})
	errs := make(chan error, len(inputs))
	var wg sync.WaitGroup
	for _, input := range inputs {
		wg.Add(1)
		go func(input StoreQuotaReserveInput) {
			defer wg.Done()
			<-start
			_, err := ledger.Reserve(context.Background(), input)
			errs <- err
		}(input)
	}
	close(start)
	wg.Wait()
	close(errs)
	var successes, exceeded int
	for err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrStoreQuotaExceeded) {
			exceeded++
		} else {
			t.Fatalf("concurrent Reserve() error = %v", err)
		}
	}
	if successes != 1 || exceeded != 1 {
		t.Fatalf("concurrent results successes=%d exceeded=%d, want 1/1", successes, exceeded)
	}
}

func TestStoreQuotaConcurrentSameRequestKeyReplaysOneDurableReservation(t *testing.T) {
	db := openConcurrentStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-same-key", PlanBasic, 1)
	ledger := NewGormStoreQuotaLedger(repo)
	input := StoreQuotaReserveInput{OrganizationID: "org-same-key", RequestKey: uuid.NewString(), ActorSubject: "actor-1"}
	start := make(chan struct{})
	results := make(chan StoreQuotaReserveResult, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := ledger.Reserve(context.Background(), input)
			results <- result
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	var observed []StoreQuotaReserveResult
	for err := range errs {
		if err != nil {
			t.Fatalf("same-key concurrent Reserve() error = %v", err)
		}
	}
	for result := range results {
		observed = append(observed, result)
	}
	if len(observed) != 2 || observed[0].AllocationID != observed[1].AllocationID || observed[0].StoreID != observed[1].StoreID || observed[0].Existing == observed[1].Existing {
		t.Fatalf("same-key results = %#v, want one new and one exact replay", observed)
	}
	summary, err := ledger.Summary(context.Background(), "org-same-key")
	if err != nil || summary.Reserved != 1 || summary.Committed != 0 {
		t.Fatalf("same-key summary = %#v, %v", summary, err)
	}
}

func TestStoreQuotaTransitionAllowsAuthorizedActorChangeAndDoesNotBackdate(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-actor", PlanBasic, 1)
	future := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.Exec("PRAGMA ignore_check_constraints = ON").Error; err != nil {
		t.Fatal(err)
	}
	ledger := newGormStoreQuotaLedger(repo, func() time.Time { return time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC) })
	input := StoreQuotaReserveInput{OrganizationID: "org-actor", RequestKey: uuid.NewString(), ActorSubject: "creator"}
	reserved, err := ledger.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE saas_store_quota_allocations SET updated_at = ? WHERE allocation_id = ?", future, reserved.AllocationID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE saas_store_quota_buckets SET updated_at = ? WHERE organization_id = ?", future, input.OrganizationID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA ignore_check_constraints = OFF").Error; err != nil {
		t.Fatal(err)
	}
	transition := StoreQuotaTransitionInput{OrganizationID: input.OrganizationID, AllocationID: reserved.AllocationID, StoreID: reserved.StoreID, RequestKey: input.RequestKey, ActorSubject: "operator"}
	committed, err := ledger.Commit(context.Background(), transition)
	if err != nil || committed.Allocation.UpdatedBy != "operator" || committed.Allocation.CreatedBy != "creator" || committed.Allocation.UpdatedAt.Before(future) || committed.Allocation.AllocatedAt.Before(future) {
		t.Fatalf("cross-actor Commit() = %#v, %v", committed, err)
	}
	transition.ActorSubject = "deleter"
	deallocated, err := ledger.Deallocate(context.Background(), transition)
	if err != nil || deallocated.Allocation.UpdatedBy != "deleter" || deallocated.Allocation.CreatedBy != "creator" || deallocated.Allocation.UpdatedAt.Before(future) || deallocated.Allocation.ReleasedAt.Before(future) {
		t.Fatalf("cross-actor Deallocate() = %#v, %v", deallocated, err)
	}
	if replay, err := ledger.Deallocate(context.Background(), transition); err != nil || !replay.Existing {
		t.Fatalf("cross-actor deallocate replay = %#v, %v", replay, err)
	}
}

func TestStoreQuotaCancelledContextStopsBeforeReservationWork(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-cancel", PlanBasic, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewGormStoreQuotaLedger(repo).Reserve(ctx, StoreQuotaReserveInput{OrganizationID: "org-cancel", RequestKey: uuid.NewString(), ActorSubject: "actor-1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Reserve() error = %v, want context canceled", err)
	}
	var count int64
	if err := db.Model(&storeQuotaAllocationRow{}).Where("organization_id = ?", "org-cancel").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("cancelled Reserve() allocations=%d, %v; want 0", count, err)
	}
}

func TestStoreQuotaTransitionMissingBucketRejectsReplayWithoutMutatingAllocation(t *testing.T) {
	db := openStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-missing-bucket", PlanBasic, 1)
	ledger := NewGormStoreQuotaLedger(repo)
	input := StoreQuotaReserveInput{OrganizationID: "org-missing-bucket", RequestKey: uuid.NewString(), ActorSubject: "creator"}
	reserved, err := ledger.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	transition := StoreQuotaTransitionInput{OrganizationID: input.OrganizationID, AllocationID: reserved.AllocationID, StoreID: reserved.StoreID, RequestKey: input.RequestKey, ActorSubject: "operator"}
	if _, err := ledger.Commit(context.Background(), transition); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("organization_id = ?", input.OrganizationID).Delete(&storeQuotaBucketRow{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Commit(context.Background(), transition); !errors.Is(err, ErrStoreQuotaInvalidTransition) {
		t.Fatalf("missing-bucket commit replay error = %v, want invalid transition", err)
	}
	allocation, err := ledger.GetByRequestKey(context.Background(), input.OrganizationID, input.RequestKey)
	if err != nil || allocation.Status != StoreQuotaAllocated {
		t.Fatalf("missing-bucket replay mutated allocation = %#v, %v", allocation, err)
	}
	foreign := transition
	foreign.OrganizationID = "org-foreign"
	if _, err := ledger.Commit(context.Background(), foreign); !errors.Is(err, ErrStoreQuotaNotFound) {
		t.Fatalf("cross-org Commit() error = %v, want not found", err)
	}
}

func TestStoreQuotaConcurrentReserveAndCommitKeepCountersConsistent(t *testing.T) {
	db := openConcurrentStoreQuotaTestDB(t)
	repo := NewGormRepository(db)
	seedStoreQuotaSubscription(t, repo, "org-lock-order", PlanBasic, 2)
	ledger := NewGormStoreQuotaLedger(repo)
	firstInput := StoreQuotaReserveInput{OrganizationID: "org-lock-order", RequestKey: uuid.NewString(), ActorSubject: "creator"}
	first, err := ledger.Reserve(context.Background(), firstInput)
	if err != nil {
		t.Fatal(err)
	}
	transition := StoreQuotaTransitionInput{OrganizationID: firstInput.OrganizationID, AllocationID: first.AllocationID, StoreID: first.StoreID, RequestKey: firstInput.RequestKey, ActorSubject: "operator"}
	secondInput := StoreQuotaReserveInput{OrganizationID: "org-lock-order", RequestKey: uuid.NewString(), ActorSubject: "creator-2"}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err := ledger.Commit(context.Background(), transition)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		<-start
		_, err := ledger.Reserve(context.Background(), secondInput)
		errs <- err
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent transition/reserve error = %v", err)
		}
	}
	summary, err := ledger.Summary(context.Background(), "org-lock-order")
	if err != nil || summary.Committed != 1 || summary.Reserved != 1 {
		t.Fatalf("lock-order summary = %#v, %v; want 1 committed and 1 reserved", summary, err)
	}
}

func openConcurrentStoreQuotaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "store-quota.db")) + "?mode=rwc&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: dsn}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(20)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func storeQuotaTimePointer(value time.Time) *time.Time { return &value }
