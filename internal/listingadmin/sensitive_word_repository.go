package listingadmin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type GormSensitiveWordRepository struct{ db *gorm.DB }

func NewGormSensitiveWordRepository(db *gorm.DB) *GormSensitiveWordRepository {
	return &GormSensitiveWordRepository{db: db}
}

func AutoMigrateSensitiveWordRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	if err := ensurePostgresColumnTypeMigrations(db, (listingSensitiveWord{}).TableName(), sensitiveWordLegacyColumnMigrations()); err != nil {
		return err
	}
	return ensureOwnerAuditColumns(db, (listingSensitiveWord{}).TableName())
}

func (r *GormSensitiveWordRepository) ListSensitiveWords(ctx context.Context, query SensitiveWordQuery) (*SensitiveWordPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sensitive word repository database is not configured")
	}
	rows, total, page, pageSize, err := findSensitiveWordRows(ctx, r.db.WithContext(ctx).Table("listing_sensitive_word"), query)
	if err != nil {
		return nil, err
	}
	items := make([]SensitiveWord, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toSensitiveWord())
	}
	return &SensitiveWordPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormSensitiveWordRepository) GetSensitiveWord(ctx context.Context, tenantID, id int64) (*SensitiveWord, error) {
	var row listingSensitiveWord
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_sensitive_word"), tenantID, id, "owner_user_id", &row, ErrSensitiveWordNotFound)
	if err != nil {
		return nil, err
	}
	word := row.toSensitiveWord()
	return &word, nil
}

func (r *GormSensitiveWordRepository) CreateSensitiveWord(ctx context.Context, word *SensitiveWord) (*SensitiveWord, error) {
	row := listingSensitiveWordFromSensitiveWord(word)
	applySensitiveWordDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applySensitiveWordAuditFields(&row, ownerUserID, true)
	}
	if err := r.db.WithContext(ctx).Table("listing_sensitive_word").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toSensitiveWord()
	return &created, nil
}

func (r *GormSensitiveWordRepository) UpdateSensitiveWord(ctx context.Context, word *SensitiveWord) (*SensitiveWord, error) {
	row := listingSensitiveWordFromSensitiveWord(word)
	applySensitiveWordDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applySensitiveWordAuditFields(&row, ownerUserID, false)
	}
	updates := map[string]any{
		"owner_user_id": row.OwnerUserID,
		"word":          row.Word,
		"language":      row.Language,
		"tags":          row.Tags,
		"level":         row.Level,
		"replace_word":  row.ReplaceWord,
		"remark":        row.Remark,
		"status":        row.Status,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_sensitive_word"), row.TenantID, row.ID, "owner_user_id", updates, ErrSensitiveWordNotFound); err != nil {
		return nil, err
	}
	return r.GetSensitiveWord(ctx, row.TenantID, row.ID)
}

func (r *GormSensitiveWordRepository) UpdateSensitiveWordStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*SensitiveWord, error) {
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_sensitive_word"), tenantID, id, "owner_user_id", updates, ErrSensitiveWordNotFound); err != nil {
		return nil, err
	}
	return r.GetSensitiveWord(ctx, tenantID, id)
}

func (r *GormSensitiveWordRepository) DeleteSensitiveWord(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_sensitive_word"), tenantID, id, "owner_user_id", updates, ErrSensitiveWordNotFound)
}
