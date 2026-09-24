package listingsubscription

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPurchasedPlanSnapshotFingerprintIsSemanticAndDeterministic(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	repository := NewGormRepository(db)
	seedPurchasedPlanCatalog(t, repository)
	service, err := NewRuntimeService(repository)
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.ResolvePurchasablePlan(context.Background(), "professional")
	if err != nil {
		t.Fatal(err)
	}
	if first.PlanCode != "professional" || first.DisplayName != "Professional" || first.Fingerprint == "" {
		t.Fatalf("snapshot = %+v", first)
	}

	// Display copy and sort order are not entitlement semantics.
	if _, err := repository.UpsertPlan(context.Background(), Plan{Code: "professional", Name: "Professional renamed", Active: true, SortOrder: 99}, []PlanModule{
		{PlanCode: "professional", ModuleCode: "module-b", Limits: map[string]int{"z": 9, "a": 1}, SortOrder: 99},
		{PlanCode: "professional", ModuleCode: "module-a", Limits: map[string]int{"quota": 10}, SortOrder: 1},
	}); err != nil {
		t.Fatal(err)
	}
	second, err := service.ResolvePurchasablePlan(context.Background(), "professional")
	if err != nil {
		t.Fatal(err)
	}
	if second.Fingerprint != first.Fingerprint {
		t.Fatalf("display/sort-only edit changed semantic fingerprint: %q != %q", second.Fingerprint, first.Fingerprint)
	}

	if _, err := repository.UpsertPlanModule(context.Background(), PlanModule{PlanCode: "professional", ModuleCode: "module-a", Limits: map[string]int{"quota": 11}}); err != nil {
		t.Fatal(err)
	}
	changed, err := service.ResolvePurchasablePlan(context.Background(), "professional")
	if err != nil {
		t.Fatal(err)
	}
	if changed.Fingerprint == first.Fingerprint {
		t.Fatal("entitlement limit edit did not change semantic fingerprint")
	}
}

func TestPurchasedSubscriptionStateReportsOnlyBlockingSubscription(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	repository := NewGormRepository(db)
	service, err := NewRuntimeService(repository)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	empty, err := service.ReadPurchasedSubscriptionState(context.Background(), "org-a")
	if err != nil || empty.BlocksPurchase {
		t.Fatalf("empty state = %+v, err = %v", empty, err)
	}
	future := now.Add(time.Hour)
	if _, err := repository.UpsertTenantSubscription(context.Background(), &TenantSubscription{TenantID: "org-a", PlanCode: "professional", Status: StatusActive, StartsAt: &future}); err != nil {
		t.Fatal(err)
	}
	blocking, err := service.ReadPurchasedSubscriptionState(context.Background(), "org-a")
	if err != nil || !blocking.BlocksPurchase || blocking.PlanCode != "professional" {
		t.Fatalf("blocking state = %+v, err = %v", blocking, err)
	}
	expired := now.Add(-time.Minute)
	if _, err := repository.UpsertTenantSubscription(context.Background(), &TenantSubscription{TenantID: "org-a", PlanCode: "professional", Status: StatusActive, ExpiresAt: &expired}); err != nil {
		t.Fatal(err)
	}
	nonblocking, err := service.ReadPurchasedSubscriptionState(context.Background(), "org-a")
	if err != nil || nonblocking.BlocksPurchase {
		t.Fatalf("expired state = %+v, err = %v", nonblocking, err)
	}
}

func TestActivatePurchasedPlanCommitsExactEntitlementsAndReplaysDecision(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	repository := NewGormRepository(db)
	seedPurchasedPlanCatalog(t, repository)
	service, err := NewRuntimeService(repository)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	snapshot, err := service.ResolvePurchasablePlan(context.Background(), "professional")
	if err != nil {
		t.Fatal(err)
	}
	input := PurchasedPlanActivationInput{
		OperationID: "subscription-activate:order-1", OrganizationID: "org-a", ActorID: "actor-a",
		SourceType: PurchasedPlanSourceCommercialOrder, SourceID: "order-1", PlanCode: snapshot.PlanCode,
		PlanFingerprint: snapshot.Fingerprint, TermMonths: 1,
	}

	activated, err := service.ActivatePurchasedPlan(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if activated.Outcome != PurchasedPlanActivationActivated || activated.Existing || activated.SubscriptionID == 0 || activated.ActivationRequestFingerprint == "" || activated.EntitlementSetFingerprint == "" {
		t.Fatalf("activation = %+v", activated)
	}
	if activated.StartsAt == nil || !activated.StartsAt.Equal(now) || activated.ExpiresAt == nil || !activated.ExpiresAt.Equal(now.AddDate(0, 1, 0)) {
		t.Fatalf("activation window = %v..%v", activated.StartsAt, activated.ExpiresAt)
	}

	entitlements, err := repository.ListEntitlements(context.Background(), "org-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(entitlements) != 2 || entitlements[0].ModuleCode != "module-a" || entitlements[1].ModuleCode != "module-b" {
		t.Fatalf("entitlements = %+v", entitlements)
	}
	for _, entitlement := range entitlements {
		if entitlement.Status != StatusActive || entitlement.StartsAt == nil || !entitlement.StartsAt.Equal(now) || entitlement.ExpiresAt == nil || !entitlement.ExpiresAt.Equal(now.AddDate(0, 1, 0)) {
			t.Fatalf("entitlement = %+v", entitlement)
		}
	}

	replayed, err := service.ActivatePurchasedPlan(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Existing || replayed.ActivationRequestFingerprint != activated.ActivationRequestFingerprint || replayed.EntitlementSetFingerprint != activated.EntitlementSetFingerprint || replayed.SubscriptionID != activated.SubscriptionID {
		t.Fatalf("replay = %+v, activation = %+v", replayed, activated)
	}
	readback, err := service.ReadPurchasedPlanActivation(context.Background(), "org-a", "order-1")
	if err != nil || readback.ActivationRequestFingerprint != activated.ActivationRequestFingerprint || !readback.Existing {
		t.Fatalf("readback = %+v, err = %v", readback, err)
	}

	conflicting := input
	conflicting.TermMonths = 2
	if _, err := service.ActivatePurchasedPlan(context.Background(), conflicting); !errors.Is(err, ErrPurchasedPlanActivationConflict) {
		t.Fatalf("conflicting replay error = %v", err)
	}
}

func TestActivatePurchasedPlanPersistsTerminalDecisionsWithoutPartialMutation(t *testing.T) {
	tests := []struct {
		name        string
		prepare     func(*testing.T, *GormRepository, *Service, PurchasedPlanSnapshot)
		mutateInput func(*PurchasedPlanActivationInput)
		failure     PurchasedPlanActivationFailureCode
	}{
		{
			name: "active subscription exists",
			prepare: func(t *testing.T, repository *GormRepository, _ *Service, _ PurchasedPlanSnapshot) {
				t.Helper()
				now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
				if _, err := repository.UpsertTenantSubscription(context.Background(), &TenantSubscription{TenantID: "org-a", PlanCode: "basic", Status: StatusActive, StartsAt: &now}); err != nil {
					t.Fatal(err)
				}
			},
			failure: PurchasedPlanActivationActiveSubscriptionExists,
		},
		{
			name: "quoted plan changed",
			prepare: func(t *testing.T, repository *GormRepository, _ *Service, _ PurchasedPlanSnapshot) {
				t.Helper()
				if _, err := repository.UpsertPlanModule(context.Background(), PlanModule{PlanCode: "professional", ModuleCode: "module-a", Limits: map[string]int{"quota": 12}}); err != nil {
					t.Fatal(err)
				}
			},
			failure: PurchasedPlanActivationPlanChanged,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openUsageLedgerTestDB(t)
			repository := NewGormRepository(db)
			seedPurchasedPlanCatalog(t, repository)
			service, err := NewRuntimeService(repository)
			if err != nil {
				t.Fatal(err)
			}
			service.now = func() time.Time { return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) }
			snapshot, err := service.ResolvePurchasablePlan(context.Background(), "professional")
			if err != nil {
				t.Fatal(err)
			}
			test.prepare(t, repository, service, snapshot)
			input := PurchasedPlanActivationInput{OperationID: "subscription-activate:order-rejected", OrganizationID: "org-a", ActorID: "actor-a", SourceType: PurchasedPlanSourceCommercialOrder, SourceID: "order-rejected", PlanCode: "professional", PlanFingerprint: snapshot.Fingerprint, TermMonths: 1}
			if test.mutateInput != nil {
				test.mutateInput(&input)
			}
			decision, err := service.ActivatePurchasedPlan(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Outcome != PurchasedPlanActivationRejected || decision.FailureCode != test.failure || decision.SubscriptionID != 0 || decision.StartsAt != nil || decision.ExpiresAt != nil || decision.EntitlementSetFingerprint != "" {
				t.Fatalf("decision = %+v", decision)
			}
			readback, err := service.ReadPurchasedPlanActivation(context.Background(), "org-a", input.SourceID)
			if err != nil || readback.Outcome != PurchasedPlanActivationRejected || !readback.Existing {
				t.Fatalf("readback = %+v, err = %v", readback, err)
			}
			if test.failure == PurchasedPlanActivationActiveSubscriptionExists {
				expired := service.now().Add(-time.Minute)
				if _, err := repository.UpsertTenantSubscription(context.Background(), &TenantSubscription{TenantID: "org-a", PlanCode: "basic", Status: StatusActive, ExpiresAt: &expired}); err != nil {
					t.Fatal(err)
				}
				replayed, err := service.ActivatePurchasedPlan(context.Background(), input)
				if err != nil || !replayed.Existing || replayed.Outcome != PurchasedPlanActivationRejected || replayed.FailureCode != PurchasedPlanActivationActiveSubscriptionExists {
					t.Fatalf("replayed rejection after expiry = %+v, err = %v", replayed, err)
				}
			}
			entitlements, err := repository.ListEntitlements(context.Background(), "org-a")
			if err != nil {
				t.Fatal(err)
			}
			if test.failure == PurchasedPlanActivationPlanChanged && len(entitlements) != 0 {
				t.Fatalf("plan-changed rejection mutated entitlements: %+v", entitlements)
			}
		})
	}
}

func seedPurchasedPlanCatalog(t *testing.T, repository *GormRepository) {
	t.Helper()
	ctx := context.Background()
	if err := repository.UpsertDefaultModules(ctx, []Module{{Code: "module-a", Name: "Module A", Active: true}, {Code: "module-b", Name: "Module B", Active: true}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertDefaultPlans(ctx, []PlanBundle{{Plan: Plan{Code: "basic", Name: "Basic", Active: true}, Modules: []PlanModule{{PlanCode: "basic", ModuleCode: "module-a", Limits: map[string]int{"quota": 1}}}}, {Plan: Plan{Code: "professional", Name: "Professional", Active: true}, Modules: []PlanModule{{PlanCode: "professional", ModuleCode: "module-b", Limits: map[string]int{"a": 1, "z": 9}, SortOrder: 2}, {PlanCode: "professional", ModuleCode: "module-a", Limits: map[string]int{"quota": 10}, SortOrder: 1}}}}); err != nil {
		t.Fatal(err)
	}
}
