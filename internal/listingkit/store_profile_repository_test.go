package listingkit

import (
	"context"
	"encoding/json"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestGormStoreProfileRepositoryPersistsByTenant(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrateStoreProfileRepository(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := NewGormStoreProfileRepository(db)

	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "101"})

	saved, err := repo.Upsert(ctx, &ListingKitStoreProfile{
		TenantID:          101,
		StoreID:           869,
		Enabled:           true,
		Priority:          10,
		Site:              "US",
		WarehouseCode:     "WH-US-1",
		DefaultStock:      88,
		DefaultSubmitMode: "publish",
		Pricing: sheinpub.PricingRule{
			SourceCurrency:   "CNY",
			TargetCurrency:   "USD",
			MarkupMultiplier: 1.3,
		},
		MatchRules: []ListingKitStoreMatchRule{{Kind: "country", Values: []string{"US"}}},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if saved.ID <= 0 {
		t.Fatalf("saved id = %d, want > 0", saved.ID)
	}
	if saved.Pricing.TargetCurrency != "USD" {
		t.Fatalf("pricing = %+v, want USD target", saved.Pricing)
	}

	updated, err := repo.Upsert(ctx, &ListingKitStoreProfile{
		TenantID:          101,
		StoreID:           869,
		Enabled:           true,
		Priority:          5,
		Site:              "CA",
		WarehouseCode:     "WH-CA-1",
		DefaultStock:      66,
		DefaultSubmitMode: "save_draft",
	})
	if err != nil {
		t.Fatalf("upsert existing store: %v", err)
	}
	if updated.ID != saved.ID {
		t.Fatalf("updated id = %d, want %d", updated.ID, saved.ID)
	}

	items, err := repo.ListByTenant(ctx, 101)
	if err != nil {
		t.Fatalf("list by tenant: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].Priority != 5 || items[0].WarehouseCode != "WH-CA-1" {
		t.Fatalf("items[0] = %+v, want updated values", items[0])
	}

	otherTenantSaved, err := repo.Upsert(ctx, &ListingKitStoreProfile{
		TenantID: 202,
		StoreID:  869,
		Enabled:  true,
		Priority: 1,
	})
	if err != nil {
		t.Fatalf("upsert other tenant: %v", err)
	}
	if otherTenantSaved.ID == saved.ID {
		t.Fatalf("other tenant id = %d, should not reuse same row id", otherTenantSaved.ID)
	}

	items202, err := repo.ListByTenant(ctx, 202)
	if err != nil {
		t.Fatalf("list other tenant: %v", err)
	}
	if len(items202) != 1 || items202[0].TenantID != 202 {
		t.Fatalf("tenant 202 items = %+v, want isolated row", items202)
	}

	if err := repo.Delete(ctx, 101, saved.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	items, err = repo.ListByTenant(ctx, 101)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("tenant 101 items after delete = %d, want 0", len(items))
	}
}

func TestGormStoreProfileRepositoryPreservesLegacyFallbackColumnWithoutExposingIt(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrateStoreProfileRepository(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	legacy := listingKitStoreProfileRecord{
		TenantID:          101,
		StoreID:           869,
		Enabled:           true,
		Priority:          10,
		IsFallback:        true,
		Site:              "US",
		DefaultSubmitMode: "publish",
		PricingJSON:       "{}",
		MatchRulesJSON:    "[]",
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("seed legacy profile: %v", err)
	}

	repo := NewGormStoreProfileRepository(db)
	updated, err := repo.Upsert(context.Background(), &ListingKitStoreProfile{
		ID:                legacy.ID,
		TenantID:          legacy.TenantID,
		StoreID:           legacy.StoreID,
		Enabled:           true,
		Priority:          5,
		Site:              "GB",
		DefaultSubmitMode: "save_draft",
	})
	if err != nil {
		t.Fatalf("upsert legacy profile: %v", err)
	}
	encoded, err := json.Marshal(updated)
	if err != nil {
		t.Fatalf("json.Marshal updated profile: %v", err)
	}
	var current map[string]any
	if err := json.Unmarshal(encoded, &current); err != nil {
		t.Fatalf("json.Unmarshal updated profile: %v", err)
	}
	legacyKey := "is" + "_fallback"
	if _, exists := current[legacyKey]; exists {
		t.Fatalf("updated profile = %s, must not expose legacy field %q", encoded, legacyKey)
	}

	var persisted listingKitStoreProfileRecord
	if err := db.Where("id = ?", legacy.ID).Take(&persisted).Error; err != nil {
		t.Fatalf("read persisted profile: %v", err)
	}
	if !persisted.IsFallback {
		t.Fatal("legacy fallback column was overwritten by current Upsert")
	}
}

func TestGormStoreProfileRepositoryCreateOmitsLegacyFallbackColumn(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE listingkit_store_profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			store_id INTEGER NOT NULL,
			enabled NUMERIC,
			priority INTEGER,
			is_fallback NUMERIC NOT NULL DEFAULT 1,
			site TEXT,
			warehouse_code TEXT,
			default_stock INTEGER,
			default_submit_mode TEXT,
			pricing TEXT,
			match_rules TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE (tenant_id, store_id)
		)
	`).Error; err != nil {
		t.Fatalf("create legacy-compatible schema: %v", err)
	}

	repo := NewGormStoreProfileRepository(db)
	if _, err := repo.Upsert(context.Background(), &ListingKitStoreProfile{
		TenantID:          202,
		StoreID:           870,
		Enabled:           true,
		Priority:          1,
		Site:              "US",
		DefaultSubmitMode: "publish",
	}); err != nil {
		t.Fatalf("create current profile: %v", err)
	}

	var persisted listingKitStoreProfileRecord
	if err := db.Where("tenant_id = ? AND store_id = ?", 202, 870).Take(&persisted).Error; err != nil {
		t.Fatalf("read created profile: %v", err)
	}
	if !persisted.IsFallback {
		t.Fatal("current create path wrote the historical fallback column instead of leaving its database default")
	}
}
