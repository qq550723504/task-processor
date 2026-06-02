package listingadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type GormGenerationTopicPolicyRepository struct{ db *gorm.DB }

func NewGormGenerationTopicPolicyRepository(db *gorm.DB) *GormGenerationTopicPolicyRepository {
	return &GormGenerationTopicPolicyRepository{db: db}
}

func AutoMigrateGenerationTopicPolicyRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	if err := db.AutoMigrate(&listingGenerationTopicPolicy{}); err != nil {
		return err
	}
	if err := ensureOwnerAuditColumns(db, (listingGenerationTopicPolicy{}).TableName()); err != nil {
		return err
	}
	if err := ensureNoDuplicateGenerationTopicPolicies(db); err != nil {
		return err
	}
	return ensureUniqueIndex(
		db,
		(listingGenerationTopicPolicy{}).TableName(),
		"idx_listing_generation_topic_policy_unique",
		"tenant_id",
		"platform",
		"topic_key",
	)
}

func (r *GormGenerationTopicPolicyRepository) ListGenerationTopicPolicies(ctx context.Context, query GenerationTopicPolicyQuery) (*GenerationTopicPolicyPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic policy repository database is not configured")
	}
	rows, total, page, pageSize, err := findGenerationTopicPolicyRows(ctx, r.db.WithContext(ctx).Table("listing_generation_topic_policy"), query)
	if err != nil {
		return nil, err
	}
	items := make([]GenerationTopicPolicy, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toGenerationTopicPolicy())
	}
	return &GenerationTopicPolicyPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormGenerationTopicPolicyRepository) ListEnabledTopicKeys(ctx context.Context, tenantID int64, platform string) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic policy repository database is not configured")
	}
	var keys []string
	err := r.db.WithContext(ctx).
		Table("listing_generation_topic_policy").
		Where("tenant_id = ? AND platform = ? AND status = ? AND deleted = 0", tenantID, strings.TrimSpace(platform), int16(1)).
		Order("topic_key asc").
		Distinct("topic_key").
		Pluck("topic_key", &keys).Error
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *GormGenerationTopicPolicyRepository) GetGenerationTopicPolicy(ctx context.Context, tenantID, id int64) (*GenerationTopicPolicy, error) {
	var row listingGenerationTopicPolicy
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_generation_topic_policy"), tenantID, id, "owner_user_id", &row, ErrGenerationTopicPolicyNotFound)
	if err != nil {
		return nil, err
	}
	policy := row.toGenerationTopicPolicy()
	return &policy, nil
}

func (r *GormGenerationTopicPolicyRepository) CreateGenerationTopicPolicy(ctx context.Context, policy *GenerationTopicPolicy) (*GenerationTopicPolicy, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic policy repository database is not configured")
	}
	row := listingGenerationTopicPolicyFromGenerationTopicPolicy(policy)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyGenerationTopicPolicyAuditFields(&row, ownerUserID, true)
	}
	if err := r.db.WithContext(ctx).Table("listing_generation_topic_policy").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toGenerationTopicPolicy()
	return &created, nil
}

func (r *GormGenerationTopicPolicyRepository) UpdateGenerationTopicPolicy(ctx context.Context, policy *GenerationTopicPolicy) (*GenerationTopicPolicy, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("generation topic policy repository database is not configured")
	}
	row := listingGenerationTopicPolicyFromGenerationTopicPolicy(policy)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyGenerationTopicPolicyAuditFields(&row, ownerUserID, false)
	}
	updates := map[string]any{
		"owner_user_id": row.OwnerUserID,
		"platform":      row.Platform,
		"topic_key":     row.TopicKey,
		"remark":        row.Remark,
		"status":        row.Status,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := r.db.WithContext(ctx).
		Table("listing_generation_topic_policy").
		Where("tenant_id = ? AND id = ? AND deleted = 0", row.TenantID, row.ID).
		Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrGenerationTopicPolicyNotFound
	}
	var updatedRow listingGenerationTopicPolicy
	if err := r.db.WithContext(ctx).
		Table("listing_generation_topic_policy").
		Where("tenant_id = ? AND id = ? AND deleted = 0", row.TenantID, row.ID).
		Take(&updatedRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGenerationTopicPolicyNotFound
		}
		return nil, err
	}
	updated := updatedRow.toGenerationTopicPolicy()
	return &updated, nil
}

func (r *GormGenerationTopicPolicyRepository) UpdateGenerationTopicPolicyStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*GenerationTopicPolicy, error) {
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_generation_topic_policy"), tenantID, id, "owner_user_id", updates, ErrGenerationTopicPolicyNotFound); err != nil {
		return nil, err
	}
	return r.GetGenerationTopicPolicy(ctx, tenantID, id)
}

func (r *GormGenerationTopicPolicyRepository) DeleteGenerationTopicPolicy(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_generation_topic_policy"), tenantID, id, "owner_user_id", updates, ErrGenerationTopicPolicyNotFound)
}

type duplicateGenerationTopicPolicyKey struct {
	TenantID int64  `gorm:"column:tenant_id"`
	Platform string `gorm:"column:platform"`
	TopicKey string `gorm:"column:topic_key"`
	Count    int64  `gorm:"column:duplicate_count"`
}

func ensureNoDuplicateGenerationTopicPolicies(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	var duplicates []duplicateGenerationTopicPolicyKey
	err := db.Table((listingGenerationTopicPolicy{}).TableName()).
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
		"cannot create unique index on listing_generation_topic_policy (tenant_id, platform, topic_key): duplicate active rows exist, first duplicate tenant_id=%d platform=%q topic_key=%q count=%d",
		first.TenantID,
		first.Platform,
		first.TopicKey,
		first.Count,
	)
}
