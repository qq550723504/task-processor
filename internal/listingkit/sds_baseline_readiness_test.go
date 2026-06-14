package listingkit

import (
	"context"
	"testing"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/sdslogin"
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

func TestGetSDSBaselineReadinessTreatsReadyCacheStatusAsUsable(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	cacheRepo, ok := repo.(SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}
	query := &SDSBaselineReadinessQuery{
		TenantID:           "tenant-a",
		ParentProductID:    9002,
		PrototypeGroupID:   7002,
		VariantID:          212095,
		SelectedVariantIDs: []int64{212095},
	}
	payload, err := newCanonicalProductCachePayload(&canonical.Product{
		Title: "Ready Baseline Product",
	})
	if err != nil {
		t.Fatalf("newCanonicalProductCachePayload: %v", err)
	}
	if err := cacheRepo.SaveSDSBaselineCache(WithTenantID(context.Background(), "tenant-a"), &SDSBaselineCacheEntry{
		TenantID:             "tenant-a",
		BaselineKey:          SDSBaselineKeyFromOptions("tenant-a", query.BaselineOptions()),
		Status:               SDSBaselineStatusReady,
		Version:              1,
		CanonicalProductBase: payload,
		ValidationStatus:     SDSBaselineValidationStatusReady,
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	svc := &service{repo: repo}
	readiness, err := svc.GetSDSBaselineReadiness(WithTenantID(context.Background(), "tenant-a"), query)
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.CacheStatus != SDSBaselineStatusReady {
		t.Fatalf("readiness = %+v, want cache_status=ready", readiness)
	}
	if readiness.ValidationStatus != SDSBaselineValidationStatusReady {
		t.Fatalf("readiness = %+v, want validation_status=ready", readiness)
	}
	if readiness.Status != SDSBaselineStatusReady {
		t.Fatalf("readiness = %+v, want status=ready", readiness)
	}
	if readiness.ReasonCode != "" || readiness.Reason != "" {
		t.Fatalf("readiness = %+v, want empty reason for ready cache", readiness)
	}
}

func TestGetSDSBaselineReadinessTreatsReadyCacheStatusWithUnknownValidationAsReady(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	cacheRepo, ok := repo.(SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}
	query := &SDSBaselineReadinessQuery{
		TenantID:           "tenant-a",
		ParentProductID:    9003,
		PrototypeGroupID:   7003,
		VariantID:          212095,
		SelectedVariantIDs: []int64{212095},
	}
	payload, err := newCanonicalProductCachePayload(&canonical.Product{
		Title: "Legacy Ready Baseline Product",
	})
	if err != nil {
		t.Fatalf("newCanonicalProductCachePayload: %v", err)
	}
	if err := cacheRepo.SaveSDSBaselineCache(WithTenantID(context.Background(), "tenant-a"), &SDSBaselineCacheEntry{
		TenantID:             "tenant-a",
		BaselineKey:          SDSBaselineKeyFromOptions("tenant-a", query.BaselineOptions()),
		Status:               SDSBaselineStatusReady,
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
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.CacheStatus != SDSBaselineStatusReady {
		t.Fatalf("readiness = %+v, want cache_status=ready", readiness)
	}
	if readiness.Status != SDSBaselineStatusReady {
		t.Fatalf("readiness = %+v, want status=ready for legacy ready cache", readiness)
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

func TestGetSDSBaselineReadinessClearsCachedLoginCredentialBlockWhenAccessTokenExists(t *testing.T) {
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
	payload, err := newCanonicalProductCachePayload(&canonical.Product{
		Title: "Baseline Product",
	})
	if err != nil {
		t.Fatalf("newCanonicalProductCachePayload: %v", err)
	}
	ctx := WithTenantID(context.Background(), DefaultTenantID)
	if err := cacheRepo.SaveSDSBaselineCache(ctx, &SDSBaselineCacheEntry{
		BaselineKey:          SDSBaselineKeyFromOptions(DefaultTenantID, query.BaselineOptions()),
		Status:               SDSBaselineStatusBaselineCached,
		Version:              1,
		CanonicalProductBase: payload,
		ValidationStatus:     SDSBaselineValidationStatusBlocked,
		ValidationReasonCode: SDSBaselineReasonCodeLoginMissingCredentials,
		ValidationReason:     "SDS login state is missing cookie or access token.",
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	svc := seedWorkflowDepsFromMirrors(&service{
		repo: repo,
		mirrors: serviceDependencyMirrors{
			sdsLoginStatusProvider: stubSDSLoginStatusProvider{
				status: &sdslogin.Status{HasAccessToken: true},
			},
		},
	})
	readiness, err := svc.GetSDSBaselineReadiness(ctx, query)
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.Status != SDSBaselineStatusReady || readiness.ValidationStatus != SDSBaselineValidationStatusReady {
		t.Fatalf("readiness = %+v, want ready/ready after clearing stale login block", readiness)
	}
	if readiness.ReasonCode != "" || readiness.Reason != "" {
		t.Fatalf("readiness = %+v, want cleared reason fields", readiness)
	}
}

func TestGetSDSBaselineReadinessClearsCachedLoginInProgressBlockWhenLoginHasCompleted(t *testing.T) {
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
	payload, err := newCanonicalProductCachePayload(&canonical.Product{
		Title: "Baseline Product",
	})
	if err != nil {
		t.Fatalf("newCanonicalProductCachePayload: %v", err)
	}
	ctx := WithTenantID(context.Background(), DefaultTenantID)
	if err := cacheRepo.SaveSDSBaselineCache(ctx, &SDSBaselineCacheEntry{
		BaselineKey:          SDSBaselineKeyFromOptions(DefaultTenantID, query.BaselineOptions()),
		Status:               SDSBaselineStatusBaselineCached,
		Version:              1,
		CanonicalProductBase: payload,
		ValidationStatus:     SDSBaselineValidationStatusBlocked,
		ValidationReasonCode: SDSBaselineReasonCodeLoginInProgress,
		ValidationReason:     "SDS login is still in progress.",
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	svc := seedWorkflowDepsFromMirrors(&service{
		repo: repo,
		mirrors: serviceDependencyMirrors{
			sdsLoginStatusProvider: stubSDSLoginStatusProvider{
				status: &sdslogin.Status{
					HasAccessToken: true,
				},
			},
		},
	})
	readiness, err := svc.GetSDSBaselineReadiness(ctx, query)
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.Status != SDSBaselineStatusReady || readiness.ValidationStatus != SDSBaselineValidationStatusReady {
		t.Fatalf("readiness = %+v, want ready/ready after clearing stale login-in-progress block", readiness)
	}
	if readiness.ReasonCode != "" || readiness.Reason != "" {
		t.Fatalf("readiness = %+v, want cleared reason fields", readiness)
	}
}

func TestGetSDSBaselineReadinessClearsCachedDesignSurfaceCredentialFailureWhenAccessTokenExists(t *testing.T) {
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
	payload, err := newCanonicalProductCachePayload(&canonical.Product{
		Title: "Baseline Product",
	})
	if err != nil {
		t.Fatalf("newCanonicalProductCachePayload: %v", err)
	}
	ctx := WithTenantID(context.Background(), DefaultTenantID)
	if err := cacheRepo.SaveSDSBaselineCache(ctx, &SDSBaselineCacheEntry{
		BaselineKey:          SDSBaselineKeyFromOptions(DefaultTenantID, query.BaselineOptions()),
		Status:               SDSBaselineStatusBaselineCached,
		Version:              1,
		CanonicalProductBase: payload,
		ValidationStatus:     SDSBaselineValidationStatusFailed,
		ValidationReasonCode: SDSBaselineReasonCodeDesignSurfaceCheckFailed,
		ValidationReason:     "SDS design surface check failed: merchant_name, username and password are required",
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	svc := seedWorkflowDepsFromMirrors(&service{
		repo: repo,
		mirrors: serviceDependencyMirrors{
			sdsLoginStatusProvider: stubSDSLoginStatusProvider{
				status: &sdslogin.Status{HasAccessToken: true},
			},
		},
	})
	readiness, err := svc.GetSDSBaselineReadiness(ctx, query)
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.Status != SDSBaselineStatusReady || readiness.ValidationStatus != SDSBaselineValidationStatusReady {
		t.Fatalf("readiness = %+v, want ready/ready after clearing stale design-surface credential failure", readiness)
	}
	if readiness.ReasonCode != "" || readiness.Reason != "" {
		t.Fatalf("readiness = %+v, want cleared reason fields", readiness)
	}
}
