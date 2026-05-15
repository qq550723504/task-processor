package listingkit

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/listingkit/tenantctx"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UploadedImageRepository interface {
	SaveUploadedImage(ctx context.Context, record *UploadedImageRecord) error
	GetUploadedImage(ctx context.Context, key string) (*UploadedImageRecord, error)
	MarkUploadedImageDeleted(ctx context.Context, key string) (*UploadedImageRecord, error)
}

type MemUploadedImageRepository struct {
	mu      sync.Mutex
	nextID  int64
	records map[string]UploadedImageRecord
}

func NewMemUploadedImageRepository() *MemUploadedImageRepository {
	return &MemUploadedImageRepository{nextID: 1, records: map[string]UploadedImageRecord{}}
}

func (r *MemUploadedImageRepository) SaveUploadedImage(ctx context.Context, record *UploadedImageRecord) error {
	if record == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	tenantID := tenantctx.TenantIDFromContext(ctx)
	key := uploadedImageKey(tenantID, record.Key)
	existing := r.records[key]
	if existing.ID == 0 {
		existing.ID = r.nextID
		r.nextID++
		existing.CreatedAt = now
	}
	existing.TenantID = tenantID
	existing.Key = record.Key
	existing.Filename = record.Filename
	existing.PublicURL = record.PublicURL
	existing.ContentType = record.ContentType
	existing.Size = record.Size
	existing.OriginalName = record.OriginalName
	existing.DeletedAt = nil
	existing.UpdatedAt = now
	r.records[key] = existing
	return nil
}

func (r *MemUploadedImageRepository) GetUploadedImage(ctx context.Context, key string) (*UploadedImageRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.records[uploadedImageKey(tenantctx.TenantIDFromContext(ctx), key)]
	if !ok || record.DeletedAt != nil {
		return nil, ErrUploadedImageNotFound
	}
	out := record
	return &out, nil
}

func (r *MemUploadedImageRepository) MarkUploadedImageDeleted(ctx context.Context, key string) (*UploadedImageRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	mapKey := uploadedImageKey(tenantctx.TenantIDFromContext(ctx), key)
	record, ok := r.records[mapKey]
	if !ok || record.DeletedAt != nil {
		return nil, ErrUploadedImageNotFound
	}
	now := time.Now().UTC()
	record.DeletedAt = &now
	record.UpdatedAt = now
	r.records[mapKey] = record
	out := record
	return &out, nil
}

type GormUploadedImageRepository struct {
	db *gorm.DB
}

func NewGormUploadedImageRepository(db *gorm.DB) *GormUploadedImageRepository {
	return &GormUploadedImageRepository{db: db}
}

func AutoMigrateUploadedImageRepository(db *gorm.DB) error {
	return db.AutoMigrate(&UploadedImageRecord{})
}

func (r *GormUploadedImageRepository) SaveUploadedImage(ctx context.Context, record *UploadedImageRecord) error {
	if record == nil {
		return nil
	}
	now := time.Now().UTC()
	row := *record
	row.TenantID = tenantctx.TenantIDFromContext(ctx)
	row.DeletedAt = nil
	row.UpdatedAt = now
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"filename":      row.Filename,
			"public_url":    row.PublicURL,
			"content_type":  row.ContentType,
			"size":          row.Size,
			"original_name": row.OriginalName,
			"deleted_at":    nil,
			"updated_at":    now,
		}),
	}).Create(&row).Error
}

func (r *GormUploadedImageRepository) GetUploadedImage(ctx context.Context, key string) (*UploadedImageRecord, error) {
	var record UploadedImageRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND key = ? AND deleted_at IS NULL", tenantctx.TenantIDFromContext(ctx), key).
		Take(&record).Error
	if err != nil {
		return nil, ErrUploadedImageNotFound
	}
	return &record, nil
}

func (r *GormUploadedImageRepository) MarkUploadedImageDeleted(ctx context.Context, key string) (*UploadedImageRecord, error) {
	record, err := r.GetUploadedImage(ctx, key)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).Model(&UploadedImageRecord{}).
		Where("tenant_id = ? AND key = ? AND deleted_at IS NULL", tenantctx.TenantIDFromContext(ctx), key).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
		return nil, err
	}
	record.DeletedAt = &now
	record.UpdatedAt = now
	return record, nil
}

func uploadedImageKey(tenantID, key string) string {
	return tenantctx.NormalizeTenantID(tenantID) + "\x00" + key
}
