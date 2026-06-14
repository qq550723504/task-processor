package listingkit

import (
	"context"
	"fmt"
	"testing"

	"task-processor/internal/catalog/canonical"
	sdsdesign "task-processor/internal/sds/design"
	sdstemplate "task-processor/internal/sds/template"
	"task-processor/internal/sdslogin"
)

type stubSDSLoginStatusProvider struct {
	status *sdslogin.Status
	err    error
}

func (s stubSDSLoginStatusProvider) Status(context.Context) (*sdslogin.Status, error) {
	return s.status, s.err
}

type stubSDSBaselineRemoteProvider struct {
	productDetail    *sdstemplate.ProductDetail
	productDetailErr error
	designProduct    *sdsdesign.DesignProductPage
	designProductErr error
	prototypeGroups  []sdsdesign.PrototypeGroup
	prototypeErr     error
}

func (s stubSDSBaselineRemoteProvider) GetProductDetail(context.Context, int64) (*sdstemplate.ProductDetail, error) {
	return s.productDetail, s.productDetailErr
}

func (s stubSDSBaselineRemoteProvider) GetDesignProduct(context.Context, int64) (*sdsdesign.DesignProductPage, error) {
	return s.designProduct, s.designProductErr
}

func (s stubSDSBaselineRemoteProvider) GetPrototypeGroups(context.Context, int64) ([]sdsdesign.PrototypeGroup, error) {
	return s.prototypeGroups, s.prototypeErr
}

func TestWarmSDSBaselineReturnsReadyWhenCacheAndValidationPass(t *testing.T) {
	t.Parallel()

	svc := seedWorkflowDepsFromMirrors(&service{
		repo: NewInMemoryRepositoryForTest(),
		mirrors: serviceDependencyMirrors{
			sdsLoginStatusProvider: stubSDSLoginStatusProvider{
				status: &sdslogin.Status{
					HasAccessToken: true,
				},
			},
			sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{
				productDetail: &sdstemplate.ProductDetail{},
				designProduct: &sdsdesign.DesignProductPage{
					Product:        sdsdesign.DesignProduct{ID: 101},
					PrototypeGroup: sdsdesign.PrototypeGroup{ID: 7001},
					Layers:         []sdsdesign.DesignLayer{{ID: "layer-1"}},
				},
				prototypeGroups: []sdsdesign.PrototypeGroup{{ID: 7001}},
			},
		},
	})

	readiness, err := svc.WarmSDSBaseline(context.Background(), &WarmSDSBaselineRequest{
		SDS: &SDSSyncOptions{
			ParentProductID:  9001,
			PrototypeGroupID: 7001,
			VariantID:        101,
			DesignType:       "material",
			LayerID:          "layer-1",
			PrintableWidth:   1000,
			PrintableHeight:  1000,
			ProductName:      "Baseline product",
		},
	})
	if err != nil {
		t.Fatalf("WarmSDSBaseline() error = %v", err)
	}
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.CacheStatus != SDSBaselineStatusBaselineCached {
		t.Fatalf("cache status = %q, want baseline_cached", readiness.CacheStatus)
	}
	if readiness.ValidationStatus != SDSBaselineValidationStatusReady {
		t.Fatalf("validation status = %q, want ready", readiness.ValidationStatus)
	}
	if readiness.Status != SDSBaselineStatusReady {
		t.Fatalf("status = %q, want ready", readiness.Status)
	}
}

func TestWarmSDSBaselineReturnsBlockedWhenRemoteSurfaceMismatches(t *testing.T) {
	t.Parallel()

	svc := seedWorkflowDepsFromMirrors(&service{
		repo: NewInMemoryRepositoryForTest(),
		mirrors: serviceDependencyMirrors{
			sdsLoginStatusProvider: stubSDSLoginStatusProvider{
				status: &sdslogin.Status{
					HasAccessToken: true,
				},
			},
			sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{
				productDetail: &sdstemplate.ProductDetail{},
				designProduct: &sdsdesign.DesignProductPage{
					Product:        sdsdesign.DesignProduct{ID: 101},
					PrototypeGroup: sdsdesign.PrototypeGroup{ID: 9999},
					Layers:         []sdsdesign.DesignLayer{{ID: "other-layer"}},
				},
				prototypeGroups: []sdsdesign.PrototypeGroup{{ID: 9999}},
			},
		},
	})

	readiness, err := svc.WarmSDSBaseline(context.Background(), &WarmSDSBaselineRequest{
		SDS: &SDSSyncOptions{
			ParentProductID:  9001,
			PrototypeGroupID: 7001,
			VariantID:        101,
			DesignType:       "material",
			LayerID:          "layer-1",
			PrintableWidth:   1000,
			PrintableHeight:  1000,
			ProductName:      "Baseline product",
		},
	})
	if err != nil {
		t.Fatalf("WarmSDSBaseline() error = %v", err)
	}
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.ValidationStatus != SDSBaselineValidationStatusBlocked {
		t.Fatalf("validation status = %q, want blocked", readiness.ValidationStatus)
	}
	if readiness.Status != SDSBaselineStatusBlocked {
		t.Fatalf("status = %q, want blocked", readiness.Status)
	}
	if readiness.ReasonCode != SDSBaselineReasonCodePrototypeGroupMismatch {
		t.Fatalf("reason code = %q, want %q", readiness.ReasonCode, SDSBaselineReasonCodePrototypeGroupMismatch)
	}
}

func TestWarmSDSBaselineReturnsBlockedWhenRequiredFieldsMissing(t *testing.T) {
	t.Parallel()

	svc := &service{repo: NewInMemoryRepositoryForTest()}

	readiness, err := svc.WarmSDSBaseline(context.Background(), &WarmSDSBaselineRequest{
		SDS: &SDSSyncOptions{
			ParentProductID:  9001,
			PrototypeGroupID: 7001,
			VariantID:        101,
			DesignType:       "material",
			ProductName:      "Baseline product",
			PrintableWidth:   1000,
			PrintableHeight:  1000,
			// Missing LayerID to trigger blocked status
		},
	})
	if err != nil {
		t.Fatalf("WarmSDSBaseline() error = %v", err)
	}
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.CacheStatus != SDSBaselineStatusBaselineCached {
		t.Fatalf("cache status = %q, want baseline_cached", readiness.CacheStatus)
	}
	if readiness.ValidationStatus != SDSBaselineValidationStatusBlocked {
		t.Fatalf("validation status = %q, want blocked", readiness.ValidationStatus)
	}
	if readiness.Status != SDSBaselineStatusBlocked {
		t.Fatalf("status = %q, want blocked", readiness.Status)
	}
	if readiness.Reason == "" {
		t.Fatal("expected blocked reason")
	}
}

func TestGetSDSBaselineReadinessReturnsCachedWhenValidationUnknown(t *testing.T) {
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
	payload, err := newCanonicalProductCachePayload(&canonical.Product{Title: "Baseline Product"})
	if err != nil {
		t.Fatalf("newCanonicalProductCachePayload: %v", err)
	}
	if err := cacheRepo.SaveSDSBaselineCache(context.Background(), &SDSBaselineCacheEntry{
		BaselineKey:          SDSBaselineKeyFromOptions(DefaultTenantID, query.BaselineOptions()),
		Status:               SDSBaselineStatusBaselineCached,
		Version:              1,
		CanonicalProductBase: payload,
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	svc := &service{repo: repo}
	readiness, err := svc.GetSDSBaselineReadiness(context.Background(), query)
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.CacheStatus != SDSBaselineStatusBaselineCached {
		t.Fatalf("cache status = %q, want baseline_cached", readiness.CacheStatus)
	}
	if readiness.ValidationStatus != SDSBaselineValidationStatusUnknown {
		t.Fatalf("validation status = %q, want unknown", readiness.ValidationStatus)
	}
	if readiness.Status != SDSBaselineStatusBaselineCached {
		t.Fatalf("status = %q, want baseline_cached", readiness.Status)
	}
}

func TestWarmSDSBaselineTreatsRemoteDesignSurfaceCredentialBootstrapFailureAsReady(t *testing.T) {
	t.Parallel()

	svc := seedWorkflowDepsFromMirrors(&service{
		repo: NewInMemoryRepositoryForTest(),
		mirrors: serviceDependencyMirrors{
			sdsLoginStatusProvider: stubSDSLoginStatusProvider{
				status: &sdslogin.Status{
					HasAccessToken: true,
				},
			},
			sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{
				productDetail:    &sdstemplate.ProductDetail{},
				designProductErr: fmt.Errorf("merchant_name, username and password are required"),
			},
		},
	})

	readiness, err := svc.WarmSDSBaseline(context.Background(), &WarmSDSBaselineRequest{
		SDS: &SDSSyncOptions{
			ParentProductID:  9001,
			PrototypeGroupID: 7001,
			VariantID:        101,
			DesignType:       "material",
			LayerID:          "layer-1",
			PrintableWidth:   1000,
			PrintableHeight:  1000,
			ProductName:      "Baseline product",
		},
	})
	if err != nil {
		t.Fatalf("WarmSDSBaseline() error = %v", err)
	}
	if readiness == nil {
		t.Fatal("expected readiness payload")
	}
	if readiness.ValidationStatus != SDSBaselineValidationStatusReady {
		t.Fatalf("validation status = %q, want ready", readiness.ValidationStatus)
	}
	if readiness.Status != SDSBaselineStatusReady {
		t.Fatalf("status = %q, want ready", readiness.Status)
	}
	if readiness.ReasonCode != "" || readiness.Reason != "" {
		t.Fatalf("readiness = %+v, want empty downgraded reason", readiness)
	}
}
