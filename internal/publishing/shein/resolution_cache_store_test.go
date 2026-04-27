package shein

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "modernc.org/sqlite"
)

func newResolutionCacheTestStore(t *testing.T) ResolutionCacheStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&SheinResolutionCacheEntry{}); err != nil {
		t.Fatalf("migrate cache entry: %v", err)
	}
	return NewGormResolutionCacheStore(db)
}

func TestGormResolutionCacheStoreCreateGetDelete(t *testing.T) {
	store := newResolutionCacheTestStore(t)
	ctx := context.Background()
	entry := &SheinResolutionCacheEntry{
		StoreID:        "42",
		CacheKind:      ResolutionCacheKindCategory,
		CacheKey:       "abcdef1234567890",
		ShortKey:       "abcdef123456",
		Source:         "live_resolver",
		ResolutionJSON: `{"status":"resolved","category_id":8218}`,
	}

	if err := store.SaveResolutionCache(ctx, entry); err != nil {
		t.Fatalf("save cache: %v", err)
	}
	got, err := store.GetResolutionCache(ctx, ResolutionCacheKindCategory, "42", "abcdef1234567890")
	if err != nil {
		t.Fatalf("get cache: %v", err)
	}
	if got == nil || got.HitCount != 1 || got.ResolutionJSON == "" {
		t.Fatalf("cache entry = %#v, want hit with payload", got)
	}
	if err := store.DeleteResolutionCache(ctx, ResolutionCacheKindCategory, "42", "abcdef1234567890"); err != nil {
		t.Fatalf("delete cache: %v", err)
	}
	got, err = store.GetResolutionCache(ctx, ResolutionCacheKindCategory, "42", "abcdef1234567890")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("cache entry after delete = %#v, want nil", got)
	}
}

func TestGormResolutionCacheStoreDeleteByShortKey(t *testing.T) {
	store := newResolutionCacheTestStore(t)
	deleter, ok := store.(ResolutionCacheShortKeyDeleter)
	if !ok {
		t.Fatal("store does not support short key deletion")
	}
	ctx := context.Background()
	entry := &SheinResolutionCacheEntry{
		StoreID:        "42",
		CacheKind:      ResolutionCacheKindAttribute,
		CacheKey:       "abcdef1234567890",
		ShortKey:       "abcdef123456",
		Source:         "history_cache",
		ResolutionJSON: `{"status":"resolved","resolved_count":1}`,
	}
	if err := store.SaveResolutionCache(ctx, entry); err != nil {
		t.Fatalf("save cache: %v", err)
	}
	if err := deleter.DeleteResolutionCacheByShortKey(ctx, ResolutionCacheKindAttribute, "42", "abcdef123456"); err != nil {
		t.Fatalf("delete by short key: %v", err)
	}
	got, err := store.GetResolutionCache(ctx, ResolutionCacheKindAttribute, "42", "abcdef1234567890")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("cache entry after short-key delete = %#v, want nil", got)
	}
}

func TestGormResolutionCacheStorePreservesManualEntry(t *testing.T) {
	store := newResolutionCacheTestStore(t)
	ctx := context.Background()
	manual := &SheinResolutionCacheEntry{
		StoreID:        "42",
		CacheKind:      ResolutionCacheKindAttribute,
		CacheKey:       "key",
		ShortKey:       "key",
		Source:         "manual_cache",
		Manual:         true,
		ResolutionJSON: `{"status":"resolved","resolved_count":1}`,
	}
	if err := store.SaveResolutionCache(ctx, manual); err != nil {
		t.Fatalf("save manual cache: %v", err)
	}
	auto := &SheinResolutionCacheEntry{
		StoreID:        "42",
		CacheKind:      ResolutionCacheKindAttribute,
		CacheKey:       "key",
		ShortKey:       "key",
		Source:         "live_resolver",
		ResolutionJSON: `{"status":"partial","resolved_count":0}`,
	}
	if err := store.SaveResolutionCache(ctx, auto); err != nil {
		t.Fatalf("save auto cache: %v", err)
	}
	got, err := store.GetResolutionCache(ctx, ResolutionCacheKindAttribute, "42", "key")
	if err != nil {
		t.Fatalf("get cache: %v", err)
	}
	if got == nil || !got.Manual || got.Source != "manual_cache" {
		t.Fatalf("cache entry = %#v, want preserved manual entry", got)
	}
}
