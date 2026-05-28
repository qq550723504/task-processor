package listingkit

import (
	"context"
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestGetSDSBaselineReadinessReturnsBaselineCachedForUsableBaseline(t *testing.T) {
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
		Status:               SDSBaselineStatusBaselineCached,
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
	if readiness == nil || readiness.Status != SDSBaselineStatusBaselineCached {
		t.Fatalf("readiness = %+v, want baseline_cached", readiness)
	}
	if readiness.CacheStatus != SDSBaselineStatusBaselineCached || readiness.ValidationStatus != SDSBaselineValidationStatusUnknown {
		t.Fatalf("readiness = %+v, want cache_status=baseline_cached validation_status=unknown", readiness)
	}
	if readiness.ReasonCode != "" {
		t.Fatalf("readiness = %+v, want empty reason code for unknown validation", readiness)
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
	if readiness == nil || readiness.Status != SDSBaselineStatusMissing {
		t.Fatalf("readiness = %+v, want missing", readiness)
	}
	if readiness.CacheStatus != SDSBaselineStatusMissing || readiness.ValidationStatus != SDSBaselineValidationStatusUnknown {
		t.Fatalf("readiness = %+v, want cache_status=missing validation_status=unknown", readiness)
	}
}

func TestGetSDSBaselineReadinessReturnsFailedForMalformedCachedBaseline(t *testing.T) {
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
		Status:      SDSBaselineStatusBaselineCached,
		Version:     1,
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	readiness, err := svc.GetSDSBaselineReadiness(ctx, query)
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil || readiness.Status != SDSBaselineStatusFailed {
		t.Fatalf("readiness = %+v, want failed", readiness)
	}
	if readiness.CacheStatus != SDSBaselineStatusFailed {
		t.Fatalf("readiness = %+v, want cache_status=failed for malformed payload", readiness)
	}
}
