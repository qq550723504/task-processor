package listingkit_test

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/store"
)

func TestSDSBaselineKeyFromOptionsStableAcrossImageAndCopyChanges(t *testing.T) {
	t.Parallel()

	options := &listingkit.SDSSyncOptions{
		VariantID:        101,
		ParentProductID:  9001,
		PrototypeGroupID: 7001,
		Variants: []listingkit.SDSSyncVariantOption{
			{VariantID: 101},
			{VariantID: 102},
		},
	}

	first := listingkit.SDSBaselineKeyFromOptions("tenant-a", options)
	second := listingkit.SDSBaselineKeyFromOptions("tenant-a", &listingkit.SDSSyncOptions{
		VariantID:        101,
		ParentProductID:  9001,
		PrototypeGroupID: 7001,
		ProductName:      "Different copy",
		MockupImageURLs:  []string{"https://example.com/changed.png"},
		Variants: []listingkit.SDSSyncVariantOption{
			{VariantID: 102},
			{VariantID: 101},
		},
	})

	if first == "" || second == "" {
		t.Fatal("expected non-empty baseline keys")
	}
	if first != second {
		t.Fatalf("baseline key drifted: %q != %q", first, second)
	}
}

func TestMemTaskRepositorySaveAndGetSDSBaselineCache(t *testing.T) {
	t.Parallel()

	memRepo := store.NewMemTaskRepository()
	repo, ok := interface{}(memRepo).(listingkit.SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}

	payload := listingkit.CanonicalProductCachePayload(canonical.Product{Title: "Baseline Product"})
	entry := &listingkit.SDSBaselineCacheEntry{
		TenantID:    "tenant-a",
		BaselineKey: "baseline-key",
		Status:      listingkit.SDSBaselineStatusBaselineCached,
		Version:     1,
		Identity: listingkit.SDSBaselineIdentity{
			ParentProductID:    9001,
			PrototypeGroupID:   7001,
			VariantID:          101,
			SelectedVariantIDs: []int64{101, 102},
		},
		CanonicalProductBase: &payload,
	}

	if err := repo.SaveSDSBaselineCache(context.Background(), entry); err != nil {
		t.Fatalf("save baseline: %v", err)
	}
	got, err := repo.GetSDSBaselineCache(context.Background(), "tenant-a", "baseline-key")
	if err != nil {
		t.Fatalf("get baseline: %v", err)
	}
	if got == nil || got.CanonicalProductBase == nil || got.CanonicalProductBase.Title != "Baseline Product" {
		t.Fatalf("unexpected baseline entry: %+v", got)
	}
}

func TestTaskRepositorySaveAndGetSDSBaselineCache(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSBaselineCacheEntry{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo, ok := interface{}(store.NewTaskRepository(db)).(listingkit.SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("task repository does not expose SDS baseline cache repository")
	}

	payload := listingkit.CanonicalProductCachePayload(canonical.Product{Title: "Persisted Baseline"})
	entry := &listingkit.SDSBaselineCacheEntry{
		TenantID:    "tenant-a",
		BaselineKey: "baseline-key",
		Status:      listingkit.SDSBaselineStatusBaselineCached,
		Version:     1,
		Identity: listingkit.SDSBaselineIdentity{
			ParentProductID:    9001,
			PrototypeGroupID:   7001,
			VariantID:          101,
			SelectedVariantIDs: []int64{101, 102},
		},
		CanonicalProductBase: &payload,
	}

	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	if err := repo.SaveSDSBaselineCache(ctx, entry); err != nil {
		t.Fatalf("save baseline: %v", err)
	}
	got, err := repo.GetSDSBaselineCache(ctx, "tenant-a", "baseline-key")
	if err != nil {
		t.Fatalf("get baseline: %v", err)
	}
	if got == nil {
		t.Fatal("expected persisted baseline entry")
	}
	if got.TenantID != "tenant-a" || got.CanonicalProductBase == nil || got.CanonicalProductBase.Title != "Persisted Baseline" {
		t.Fatalf("unexpected baseline entry: %+v", got)
	}
}

func TestTaskRepositorySaveSDSBaselineCacheUpdatesValidationFieldsOnConflict(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSBaselineCacheEntry{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo, ok := interface{}(store.NewTaskRepository(db)).(listingkit.SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("task repository does not expose SDS baseline cache repository")
	}

	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	entry := newSDSBaselineCacheEntry("baseline-key", "tenant-a")
	if err := repo.SaveSDSBaselineCache(ctx, entry); err != nil {
		t.Fatalf("save baseline: %v", err)
	}

	entry.ValidationStatus = listingkit.SDSBaselineValidationStatusBlocked
	entry.ValidationReasonCode = listingkit.SDSBaselineReasonCodeLayerMissing
	entry.ValidationReason = "SDS design surface does not include the selected layer."
	if err := repo.SaveSDSBaselineCache(ctx, entry); err != nil {
		t.Fatalf("save baseline second pass: %v", err)
	}

	got, err := repo.GetSDSBaselineCache(ctx, "tenant-a", "baseline-key")
	if err != nil {
		t.Fatalf("get baseline: %v", err)
	}
	if got == nil {
		t.Fatal("expected persisted baseline entry")
	}
	if got.ValidationStatus != listingkit.SDSBaselineValidationStatusBlocked {
		t.Fatalf("validation status = %q, want blocked", got.ValidationStatus)
	}
	if got.ValidationReasonCode != listingkit.SDSBaselineReasonCodeLayerMissing {
		t.Fatalf("validation reason code = %q, want layer_missing", got.ValidationReasonCode)
	}
	if got.ValidationReason != "SDS design surface does not include the selected layer." {
		t.Fatalf("validation reason = %q, want selected-layer message", got.ValidationReason)
	}
}

func TestSDSBaselineCacheRepositorySaveUsesTenantFromContextWhenEntryTenantEmpty(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		repo func(t *testing.T) listingkit.SDSBaselineCacheRepository
	}{
		{name: "mem", repo: newMemSDSBaselineCacheRepository},
		{name: "db", repo: newDBSDSBaselineCacheRepository},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := tc.repo(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			entry := newSDSBaselineCacheEntry("baseline-key", "")

			if err := repo.SaveSDSBaselineCache(ctx, entry); err != nil {
				t.Fatalf("save baseline: %v", err)
			}
			got, err := repo.GetSDSBaselineCache(ctx, "", "baseline-key")
			if err != nil {
				t.Fatalf("get baseline: %v", err)
			}
			if got == nil {
				t.Fatal("expected baseline entry")
			}
			if got.TenantID != "tenant-a" {
				t.Fatalf("tenant id = %q, want tenant-a", got.TenantID)
			}
			if got.BaselineKey != "baseline-key" {
				t.Fatalf("baseline key = %q, want logical baseline-key", got.BaselineKey)
			}
		})
	}
}

func TestSDSBaselineCacheRepositoryRejectsTenantArgumentContextMismatch(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		repo func(t *testing.T) listingkit.SDSBaselineCacheRepository
	}{
		{name: "mem", repo: newMemSDSBaselineCacheRepository},
		{name: "db", repo: newDBSDSBaselineCacheRepository},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := tc.repo(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")

			if err := repo.SaveSDSBaselineCache(ctx, newSDSBaselineCacheEntry("baseline-key", "tenant-b")); err == nil {
				t.Fatal("expected tenant mismatch error on save")
			}
			if _, err := repo.GetSDSBaselineCache(ctx, "tenant-b", "baseline-key"); err == nil {
				t.Fatal("expected tenant mismatch error on get")
			}
		})
	}
}

func TestMemTaskRepositorySDSBaselineCacheIsMutationSafe(t *testing.T) {
	t.Parallel()

	repo, ok := interface{}(store.NewMemTaskRepository()).(listingkit.SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}

	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	entry := newSDSBaselineCacheEntry("baseline-key", "")

	if err := repo.SaveSDSBaselineCache(ctx, entry); err != nil {
		t.Fatalf("save baseline: %v", err)
	}

	entry.Status = "mutated-after-save"
	entry.Identity.SelectedVariantIDs[0] = 999
	entry.CanonicalProductBase.Title = "Mutated Title"

	got, err := repo.GetSDSBaselineCache(ctx, "", "baseline-key")
	if err != nil {
		t.Fatalf("get baseline: %v", err)
	}
	if got == nil {
		t.Fatal("expected baseline entry")
	}
	if got.Status != listingkit.SDSBaselineStatusBaselineCached {
		t.Fatalf("saved status mutated unexpectedly: %q", got.Status)
	}
	if got.Identity.SelectedVariantIDs[0] != 101 {
		t.Fatalf("saved selected variants mutated unexpectedly: %+v", got.Identity.SelectedVariantIDs)
	}
	if got.CanonicalProductBase == nil || got.CanonicalProductBase.Title != "Baseline Product" {
		t.Fatalf("saved canonical product mutated unexpectedly: %+v", got.CanonicalProductBase)
	}

	got.Status = "mutated-after-get"
	got.Identity.SelectedVariantIDs[0] = 555
	got.CanonicalProductBase.Title = "Changed After Get"

	again, err := repo.GetSDSBaselineCache(ctx, "", "baseline-key")
	if err != nil {
		t.Fatalf("get baseline again: %v", err)
	}
	if again == nil {
		t.Fatal("expected baseline entry on second read")
	}
	if again.Status != listingkit.SDSBaselineStatusBaselineCached {
		t.Fatalf("stored status mutated by returned clone: %q", again.Status)
	}
	if again.Identity.SelectedVariantIDs[0] != 101 {
		t.Fatalf("stored selected variants mutated by returned clone: %+v", again.Identity.SelectedVariantIDs)
	}
	if again.CanonicalProductBase == nil || again.CanonicalProductBase.Title != "Baseline Product" {
		t.Fatalf("stored canonical product mutated by returned clone: %+v", again.CanonicalProductBase)
	}
}

func newMemSDSBaselineCacheRepository(t *testing.T) listingkit.SDSBaselineCacheRepository {
	t.Helper()
	repo, ok := interface{}(store.NewMemTaskRepository()).(listingkit.SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}
	return repo
}

func newDBSDSBaselineCacheRepository(t *testing.T) listingkit.SDSBaselineCacheRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSBaselineCacheEntry{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo, ok := interface{}(store.NewTaskRepository(db)).(listingkit.SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("task repository does not expose SDS baseline cache repository")
	}
	return repo
}

func newSDSBaselineCacheEntry(baselineKey string, tenantID string) *listingkit.SDSBaselineCacheEntry {
	payload := listingkit.CanonicalProductCachePayload(canonical.Product{Title: "Baseline Product"})
	return &listingkit.SDSBaselineCacheEntry{
		TenantID:    tenantID,
		BaselineKey: baselineKey,
		Status:      listingkit.SDSBaselineStatusBaselineCached,
		Version:     1,
		Identity: listingkit.SDSBaselineIdentity{
			ParentProductID:    9001,
			PrototypeGroupID:   7001,
			VariantID:          101,
			SelectedVariantIDs: []int64{101, 102},
		},
		CanonicalProductBase: &payload,
	}
}
