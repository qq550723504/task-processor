package listingadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type GormGenerationTopicOverrideRepository struct{ db *gorm.DB }

func NewGormGenerationTopicOverrideRepository(db *gorm.DB) *GormGenerationTopicOverrideRepository {
	return &GormGenerationTopicOverrideRepository{db: db}
}

func AutoMigrateGenerationTopicOverrideRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	if err := db.AutoMigrate(&listingGenerationTopicOverride{}); err != nil {
		return err
	}
	if err := ensureOwnerAuditColumns(db, (listingGenerationTopicOverride{}).TableName()); err != nil {
		return err
	}
	if err := ensureNoDuplicateGenerationTopicOverrides(db); err != nil {
		return err
	}
	return ensureUniqueIndex(
		db,
		(listingGenerationTopicOverride{}).TableName(),
		"idx_listing_generation_topic_override_unique",
		"tenant_id",
		"platform",
		"topic_key",
	)
}

func (r *GormGenerationTopicOverrideRepository) ListGenerationTopicOverrides(ctx context.Context, query GenerationTopicOverrideQuery) (*GenerationTopicOverridePage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic override repository database is not configured")
	}
	var rows []listingGenerationTopicOverride
	total, page, pageSize, err := findPagedRows(
		applyOwnerScope(
			applyGenerationTopicOverrideQuery(r.db.WithContext(ctx).Table((listingGenerationTopicOverride{}).TableName()), query),
			ctx,
			"owner_user_id",
		),
		query.Page,
		query.PageSize,
		&rows,
	)
	if err != nil {
		return nil, err
	}
	items := make([]GenerationTopicOverride, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toGenerationTopicOverride())
	}
	return &GenerationTopicOverridePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormGenerationTopicOverrideRepository) GetGenerationTopicOverride(ctx context.Context, tenantID, id int64) (*GenerationTopicOverride, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic override repository database is not configured")
	}
	var row listingGenerationTopicOverride
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table((listingGenerationTopicOverride{}).TableName()), tenantID, id, "owner_user_id", &row, ErrGenerationTopicOverrideNotFound)
	if err != nil {
		return nil, err
	}
	item := row.toGenerationTopicOverride()
	return &item, nil
}

func (r *GormGenerationTopicOverrideRepository) GetGenerationTopicOverrideByTopicKey(ctx context.Context, tenantID int64, platform string, topicKey string) (*GenerationTopicOverride, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic override repository database is not configured")
	}
	var row listingGenerationTopicOverride
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table((listingGenerationTopicOverride{}).TableName()).Where(
			"tenant_id = ? AND platform = ? AND topic_key = ? AND deleted = 0",
			tenantID,
			strings.TrimSpace(platform),
			strings.TrimSpace(topicKey),
		),
		ctx,
		"owner_user_id",
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGenerationTopicOverrideNotFound
	}
	if err != nil {
		return nil, err
	}
	item := row.toGenerationTopicOverride()
	return &item, nil
}

func (r *GormGenerationTopicOverrideRepository) CreateGenerationTopicOverride(ctx context.Context, item *GenerationTopicOverride) (*GenerationTopicOverride, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic override repository database is not configured")
	}
	row := listingGenerationTopicOverrideFromGenerationTopicOverride(item)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyGenerationTopicOverrideAuditFields(&row, ownerUserID, true)
	}
	if err := r.db.WithContext(ctx).Table((listingGenerationTopicOverride{}).TableName()).Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toGenerationTopicOverride()
	return &created, nil
}

func (r *GormGenerationTopicOverrideRepository) UpdateGenerationTopicOverride(ctx context.Context, item *GenerationTopicOverride) (*GenerationTopicOverride, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic override repository database is not configured")
	}
	row := listingGenerationTopicOverrideFromGenerationTopicOverride(item)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyGenerationTopicOverrideAuditFields(&row, ownerUserID, false)
	}
	updates := map[string]any{
		"owner_user_id":                     row.OwnerUserID,
		"platform":                          row.Platform,
		"topic_key":                         row.TopicKey,
		"additional_prompt_directives_json": row.AdditionalPromptDirectivesJSON,
		"additional_lexicon_json":           row.AdditionalLexiconJSON,
		"remark":                            row.Remark,
		"status":                            row.Status,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table((listingGenerationTopicOverride{}).TableName()), row.TenantID, row.ID, "owner_user_id", updates, ErrGenerationTopicOverrideNotFound); err != nil {
		return nil, err
	}
	return r.GetGenerationTopicOverride(ctx, row.TenantID, row.ID)
}

func (r *GormGenerationTopicOverrideRepository) UpdateGenerationTopicOverrideStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*GenerationTopicOverride, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic override repository database is not configured")
	}
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table((listingGenerationTopicOverride{}).TableName()), tenantID, id, "owner_user_id", updates, ErrGenerationTopicOverrideNotFound); err != nil {
		return nil, err
	}
	return r.GetGenerationTopicOverride(ctx, tenantID, id)
}

func (r *GormGenerationTopicOverrideRepository) DeleteGenerationTopicOverride(ctx context.Context, tenantID, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("generation topic override repository database is not configured")
	}
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table((listingGenerationTopicOverride{}).TableName()), tenantID, id, "owner_user_id", updates, ErrGenerationTopicOverrideNotFound)
}

type duplicateGenerationTopicOverrideKey struct {
	TenantID int64  `gorm:"column:tenant_id"`
	Platform string `gorm:"column:platform"`
	TopicKey string `gorm:"column:topic_key"`
	Count    int64  `gorm:"column:duplicate_count"`
}

func ensureNoDuplicateGenerationTopicOverrides(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	var duplicates []duplicateGenerationTopicOverrideKey
	err := db.Table((listingGenerationTopicOverride{}).TableName()).
		Select("tenant_id, platform, topic_key, COUNT(*) AS duplicate_count").
		Where("deleted = 0").
		Group("tenant_id, platform, topic_key").
		Having("COUNT(*) > 1").
		Order("tenant_id ASC, platform ASC, topic_key ASC").
		Limit(5).
		Scan(&duplicates).Error
	if err != nil {
		return err
	}
	if len(duplicates) == 0 {
		return nil
	}
	first := duplicates[0]
	return fmt.Errorf(
		"cannot create unique index on listing_generation_topic_override (tenant_id, platform, topic_key): duplicate active rows exist, first duplicate tenant_id=%d platform=%q topic_key=%q count=%d",
		first.TenantID,
		first.Platform,
		first.TopicKey,
		first.Count,
	)
}
