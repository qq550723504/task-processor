package listingkit

import (
	"context"
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestGetSDSBaselineReadinessReturnsReadyForUsableBaseline(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	cacheRepo, ok := repo.(SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}
	query := &SDSBaselineReadinessQuery{
		TenantID:           "tenant-a",
		ParentProductID:    9001,
		PrototypeGroupID:   7001,
		VariantID:          101,
		SelectedVariantIDs: []int64{102, 101},
	}
	payload, err := newCanonicalProductCachePayload(&canonical.Product{
		Title: "Baseline Product",
	})
	if err != nil {
		t.Fatalf("newCanonicalProductCachePayload: %v", err)
	}
	if err := cacheRepo.SaveSDSBaselineCache(WithTenantID(context.Background(), "tenant-a"), &SDSBaselineCacheEntry{
		TenantID:             "tenant-a",
		BaselineKey:          SDSBaselineKeyFromOptions("tenant-a", query.BaselineOptions()),
		Status:               "ready",
		Version:              1,
		CanonicalProductBase: payload,
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	svc := &service{repo: repo}
	readiness, err := svc.GetSDSBaselineReadiness(WithTenantID(context.Background(), "tenant-a"), query)
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil || readiness.Status != "ready" {
		t.Fatalf("readiness = %+v, want ready", readiness)
	}
	if readiness.BaselineKey == "" {
		t.Fatalf("readiness = %+v, want baseline key", readiness)
	}
}

func TestGetSDSBaselineReadinessReturnsMissingWhenBaselineDoesNotExist(t *testing.T) {
	t.Parallel()

	svc := &service{repo: NewInMemoryRepositoryForTest()}
	readiness, err := svc.GetSDSBaselineReadiness(context.Background(), &SDSBaselineReadinessQuery{
		ParentProductID:    9001,
		PrototypeGroupID:   7001,
		VariantID:          101,
		SelectedVariantIDs: []int64{101},
	})
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil || readiness.Status != "missing" {
		t.Fatalf("readiness = %+v, want missing", readiness)
	}
}

func TestGetSDSBaselineReadinessReturnsFailedForMalformedReadyBaseline(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	cacheRepo, ok := repo.(SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}
	query := &SDSBaselineReadinessQuery{
		ParentProductID:    9001,
		PrototypeGroupID:   7001,
		VariantID:          101,
		SelectedVariantIDs: []int64{101},
	}
	svc := &service{repo: repo}
	ctx := WithTenantID(context.Background(), DefaultTenantID)
	if err := cacheRepo.SaveSDSBaselineCache(ctx, &SDSBaselineCacheEntry{
		BaselineKey: SDSBaselineKeyFromOptions(DefaultTenantID, query.BaselineOptions()),
		Status:      "ready",
		Version:     1,
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	readiness, err := svc.GetSDSBaselineReadiness(ctx, query)
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil || readiness.Status != "failed" {
		t.Fatalf("readiness = %+v, want failed", readiness)
	}
}
