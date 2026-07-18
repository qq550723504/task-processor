package listingkit

import (
	"context"
	"strings"
	"sync"
	"time"

	"task-processor/internal/shared/tenantctx"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UploadedImageRepository interface {
	SaveUploadedImage(ctx context.Context, record *UploadedImageRecord) error
	GetUploadedImage(ctx context.Context, uploadID string) (*UploadedImageRecord, error)
	MarkUploadedImageDeleted(ctx context.Context, uploadID string) (*UploadedImageRecord, error)
	ClaimUploadedImageDeletion(ctx context.Context, uploadID string) (*UploadedImageDeletionClaim, error)
	CompleteUploadedImageDeletion(ctx context.Context, uploadID string) error
	ReleaseUploadedImageDeletion(ctx context.Context, uploadID string) error
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
	identifier := uploadedImageIdentifier(record)
	key := uploadedImageKey(tenantID, identifier)
	existing := r.records[key]
	if existing.ID == 0 {
		existing.ID = r.nextID
		r.nextID++
		existing.CreatedAt = now
	}
	existing.TenantID = tenantID
	existing.Key = record.Key
	existing.UploadID = record.UploadID
	existing.StorageKey = record.StorageKey
	existing.Filename = record.Filename
	existing.PublicURL = record.PublicURL
	existing.ContentType = record.ContentType
	existing.Size = record.Size
	existing.OriginalName = record.OriginalName
	existing.DeletedAt = nil
	existing.DeleteState = "active"
	existing.UpdatedAt = now
	r.records[key] = existing
	return nil
}

func (r *MemUploadedImageRepository) GetUploadedImage(ctx context.Context, uploadID string) (*UploadedImageRecord, error) {
	if !validUploadedImageLookup(uploadID) {
		return nil, ErrUploadedImageNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.records[uploadedImageKey(tenantctx.TenantIDFromContext(ctx), uploadID)]
	if !ok || record.DeletedAt != nil {
		return nil, ErrUploadedImageNotFound
	}
	out := record
	return &out, nil
}

func (r *MemUploadedImageRepository) MarkUploadedImageDeleted(ctx context.Context, uploadID string) (*UploadedImageRecord, error) {
	claim, err := r.ClaimUploadedImageDeletion(ctx, uploadID)
	if err != nil || claim.AlreadyDeleted {
		return nil, err
	}
	if err := r.CompleteUploadedImageDeletion(ctx, uploadID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	claim.Record.DeleteState = "deleted"
	claim.Record.DeletedAt = &now
	claim.Record.UpdatedAt = now
	return claim.Record, nil
}

func (r *MemUploadedImageRepository) ClaimUploadedImageDeletion(ctx context.Context, uploadID string) (*UploadedImageDeletionClaim, error) {
	if !validUploadedImageLookup(uploadID) {
		return nil, ErrUploadedImageNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	mapKey := uploadedImageKey(tenantctx.TenantIDFromContext(ctx), uploadID)
	record, ok := r.records[mapKey]
	if !ok {
		return nil, ErrUploadedImageNotFound
	}
	if record.DeleteState == "deleting" || record.DeleteState == "deleted" || record.DeletedAt != nil {
		return &UploadedImageDeletionClaim{Record: &record, AlreadyDeleted: true}, nil
	}
	now := time.Now().UTC()
	record.DeleteState = "deleting"
	record.UpdatedAt = now
	r.records[mapKey] = record
	out := record
	return &UploadedImageDeletionClaim{Record: &out, Claimed: true}, nil
}

func (r *MemUploadedImageRepository) CompleteUploadedImageDeletion(ctx context.Context, uploadID string) error {
	if !validUploadedImageLookup(uploadID) {
		return ErrUploadedImageNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	mapKey := uploadedImageKey(tenantctx.TenantIDFromContext(ctx), uploadID)
	record, ok := r.records[mapKey]
	if !ok || record.DeleteState != "deleting" {
		return ErrUploadedImageNotFound
	}
	now := time.Now().UTC()
	record.DeleteState = "deleted"
	record.DeletedAt = &now
	record.UpdatedAt = now
	r.records[mapKey] = record
	return nil
}

func (r *MemUploadedImageRepository) ReleaseUploadedImageDeletion(ctx context.Context, uploadID string) error {
	if !validUploadedImageLookup(uploadID) {
		return ErrUploadedImageNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	mapKey := uploadedImageKey(tenantctx.TenantIDFromContext(ctx), uploadID)
	record, ok := r.records[mapKey]
	if !ok || record.DeleteState != "deleting" {
		return ErrUploadedImageNotFound
	}
	record.DeleteState = "active"
	record.UpdatedAt = time.Now().UTC()
	r.records[mapKey] = record
	return nil
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
	row.DeleteState = "active"
	if row.StorageKey == "" {
		row.StorageKey = row.Key
	}
	row.UpdatedAt = now
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	if row.UploadID != "" {
		var existing UploadedImageRecord
		err := r.db.WithContext(ctx).Where("tenant_id = ? AND upload_id = ?", row.TenantID, row.UploadID).Take(&existing).Error
		if err == nil {
			return r.db.WithContext(ctx).Model(&UploadedImageRecord{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"key": row.Key, "storage_key": row.StorageKey, "filename": row.Filename, "public_url": row.PublicURL, "content_type": row.ContentType, "size": row.Size, "original_name": row.OriginalName, "delete_state": "active", "deleted_at": nil, "updated_at": now,
			}).Error
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		return r.db.WithContext(ctx).Create(&row).Error
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"storage_key":   row.StorageKey,
			"filename":      row.Filename,
			"public_url":    row.PublicURL,
			"content_type":  row.ContentType,
			"size":          row.Size,
			"original_name": row.OriginalName,
			"deleted_at":    nil,
			"delete_state":  "active",
			"updated_at":    now,
		}),
	}).Create(&row).Error
}

func (r *GormUploadedImageRepository) GetUploadedImage(ctx context.Context, uploadID string) (*UploadedImageRecord, error) {
	column, ok := uploadedImageLookupColumn(uploadID)
	if !ok {
		return nil, ErrUploadedImageNotFound
	}
	var record UploadedImageRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND "+column+" = ? AND deleted_at IS NULL", tenantctx.TenantIDFromContext(ctx), uploadID).
		Take(&record).Error
	if err != nil {
		return nil, ErrUploadedImageNotFound
	}
	return &record, nil
}

func (r *GormUploadedImageRepository) MarkUploadedImageDeleted(ctx context.Context, uploadID string) (*UploadedImageRecord, error) {
	claim, err := r.ClaimUploadedImageDeletion(ctx, uploadID)
	if err != nil || claim.AlreadyDeleted {
		return nil, err
	}
	if err := r.CompleteUploadedImageDeletion(ctx, uploadID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	claim.Record.DeleteState = "deleted"
	claim.Record.DeletedAt = &now
	claim.Record.UpdatedAt = now
	return claim.Record, nil
}

func (r *GormUploadedImageRepository) ClaimUploadedImageDeletion(ctx context.Context, uploadID string) (*UploadedImageDeletionClaim, error) {
	column, ok := uploadedImageLookupColumn(uploadID)
	if !ok {
		return nil, ErrUploadedImageNotFound
	}
	var record UploadedImageRecord
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND "+column+" = ?", tenantctx.TenantIDFromContext(ctx), uploadID).Take(&record).Error; err != nil {
		return nil, ErrUploadedImageNotFound
	}
	if record.DeleteState == "deleting" || record.DeleteState == "deleted" || record.DeletedAt != nil {
		return &UploadedImageDeletionClaim{Record: &record, AlreadyDeleted: true}, nil
	}
	result := r.db.WithContext(ctx).Model(&UploadedImageRecord{}).Where("id = ? AND delete_state = ? AND deleted_at IS NULL", record.ID, "active").Updates(map[string]any{"delete_state": "deleting", "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return &UploadedImageDeletionClaim{Record: &record, AlreadyDeleted: true}, nil
	}
	record.DeleteState = "deleting"
	return &UploadedImageDeletionClaim{Record: &record, Claimed: true}, nil
}

func (r *GormUploadedImageRepository) CompleteUploadedImageDeletion(ctx context.Context, uploadID string) error {
	column, ok := uploadedImageLookupColumn(uploadID)
	if !ok {
		return ErrUploadedImageNotFound
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&UploadedImageRecord{}).Where("tenant_id = ? AND "+column+" = ? AND delete_state = ?", tenantctx.TenantIDFromContext(ctx), uploadID, "deleting").Updates(map[string]any{"delete_state": "deleted", "deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUploadedImageNotFound
	}
	return nil
}

func (r *GormUploadedImageRepository) ReleaseUploadedImageDeletion(ctx context.Context, uploadID string) error {
	column, ok := uploadedImageLookupColumn(uploadID)
	if !ok {
		return ErrUploadedImageNotFound
	}
	result := r.db.WithContext(ctx).Model(&UploadedImageRecord{}).Where("tenant_id = ? AND "+column+" = ? AND delete_state = ?", tenantctx.TenantIDFromContext(ctx), uploadID, "deleting").Updates(map[string]any{"delete_state": "active", "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUploadedImageNotFound
	}
	return nil
}

func uploadedImageKey(tenantID, key string) string {
	return tenantctx.NormalizeTenantID(tenantID) + "\x00" + key
}

func uploadedImageIdentifier(record *UploadedImageRecord) string {
	if record != nil && record.UploadID != "" {
		return record.UploadID
	}
	if record == nil {
		return ""
	}
	return record.Key
}

func validUploadedImageLookup(value string) bool {
	_, ok := uploadedImageLookupColumn(value)
	return ok
}

func uploadedImageLookupColumn(value string) (string, bool) {
	if _, err := uuid.Parse(value); err == nil {
		return "upload_id", true
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.Contains(trimmed, "..") || strings.HasPrefix(trimmed, "/") {
		return "", false
	}
	return "key", true
}
