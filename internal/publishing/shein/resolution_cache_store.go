package shein

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ResolutionCacheKindCategory                  = "category"
	ResolutionCacheKindAttribute                 = "attribute"
	ResolutionCacheKindSaleAttribute             = "sale_attribute"
	ResolutionCacheKindPricing                   = "pricing"
	ResolutionCacheKindSaleAttributeCustomDenied = "sale_attribute_custom_denied"

	ResolutionCacheHitSourceMemoryCache            = "memory_cache"
	ResolutionCacheHitSourcePersistentManualCache  = "persistent_manual_cache"
	ResolutionCacheHitSourcePersistentHistoryCache = "persistent_history_cache"
	ResolutionCacheHitSourcePublishRemembered      = "publish_remembered"
)

type ResolutionCacheInfo struct {
	Status    string     `json:"status,omitempty"`
	Source    string     `json:"source,omitempty"`
	HitSource string     `json:"hit_source,omitempty"`
	CacheKey  string     `json:"cache_key,omitempty"`
	ShortKey  string     `json:"short_key,omitempty"`
	HitCount  int        `json:"hit_count,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Manual    bool       `json:"manual,omitempty"`
	Clearable bool       `json:"clearable,omitempty"`
}

type SheinResolutionCacheEntry struct {
	ID             uint      `json:"id,omitempty" gorm:"primaryKey"`
	StoreID        string    `json:"store_id,omitempty" gorm:"type:varchar(64);not null;uniqueIndex:idx_shein_resolution_cache_key"`
	CacheKind      string    `json:"cache_kind,omitempty" gorm:"type:varchar(32);not null;uniqueIndex:idx_shein_resolution_cache_key"`
	CacheKey       string    `json:"cache_key,omitempty" gorm:"type:varchar(128);not null;uniqueIndex:idx_shein_resolution_cache_key"`
	ShortKey       string    `json:"short_key,omitempty" gorm:"type:varchar(16);index"`
	Source         string    `json:"source,omitempty" gorm:"type:varchar(32)"`
	Manual         bool      `json:"manual" gorm:"index"`
	SourceIdentity string    `json:"source_identity,omitempty" gorm:"type:text"`
	ResolutionJSON string    `json:"resolution_json,omitempty" gorm:"type:text"`
	HitCount       int       `json:"hit_count"`
	LastHitAt      time.Time `json:"last_hit_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ResolutionCacheStore interface {
	GetResolutionCache(ctx context.Context, kind string, storeID string, cacheKey string) (*SheinResolutionCacheEntry, error)
	SaveResolutionCache(ctx context.Context, entry *SheinResolutionCacheEntry) error
	DeleteResolutionCache(ctx context.Context, kind string, storeID string, cacheKey string) error
}

type ResolutionCacheShortKeyDeleter interface {
	DeleteResolutionCacheByShortKey(ctx context.Context, kind string, storeID string, shortKey string) error
}

type GormResolutionCacheStore struct {
	db *gorm.DB
}

func NewGormResolutionCacheStore(db *gorm.DB) ResolutionCacheStore {
	if db == nil {
		return nil
	}
	return &GormResolutionCacheStore{db: db}
}

func (s *GormResolutionCacheStore) GetResolutionCache(ctx context.Context, kind string, storeID string, cacheKey string) (*SheinResolutionCacheEntry, error) {
	var entry SheinResolutionCacheEntry
	if err := s.db.WithContext(ctx).
		Where("cache_kind = ? AND store_id = ? AND cache_key = ?", kind, storeID, cacheKey).
		First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&SheinResolutionCacheEntry{}).
		Where("id = ?", entry.ID).
		Updates(map[string]any{"hit_count": gorm.Expr("hit_count + ?", 1), "last_hit_at": now}).Error
	entry.HitCount++
	entry.LastHitAt = now
	return &entry, nil
}

func (s *GormResolutionCacheStore) SaveResolutionCache(ctx context.Context, entry *SheinResolutionCacheEntry) error {
	if entry == nil {
		return nil
	}
	var existing SheinResolutionCacheEntry
	err := s.db.WithContext(ctx).
		Where("cache_kind = ? AND store_id = ? AND cache_key = ?", entry.CacheKind, entry.StoreID, entry.CacheKey).
		First(&existing).Error
	if err == nil && existing.Manual && !entry.Manual {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "store_id"},
			{Name: "cache_kind"},
			{Name: "cache_key"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"short_key",
			"source",
			"manual",
			"source_identity",
			"resolution_json",
			"updated_at",
		}),
	}).Create(entry).Error
}

func (s *GormResolutionCacheStore) DeleteResolutionCache(ctx context.Context, kind string, storeID string, cacheKey string) error {
	query := s.db.WithContext(ctx).
		Where("store_id = ? AND cache_key = ?", storeID, cacheKey)
	if kind != "" && kind != "all" {
		query = query.Where("cache_kind = ?", kind)
	}
	return query.Delete(&SheinResolutionCacheEntry{}).Error
}

func (s *GormResolutionCacheStore) DeleteResolutionCacheByShortKey(ctx context.Context, kind string, storeID string, shortKey string) error {
	query := s.db.WithContext(ctx).
		Where("store_id = ? AND short_key = ?", storeID, shortKey)
	if kind != "" && kind != "all" {
		query = query.Where("cache_kind = ?", kind)
	}
	return query.Delete(&SheinResolutionCacheEntry{}).Error
}
