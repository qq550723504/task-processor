package listingadmin

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type GormCategoryRepository struct{ db *gorm.DB }

func NewGormCategoryRepository(db *gorm.DB) *GormCategoryRepository {
	return &GormCategoryRepository{db: db}
}

func AutoMigrateCategoryRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingCategory{}).TableName())
}

func (r *GormCategoryRepository) ListCategories(ctx context.Context, query CategoryQuery) ([]Category, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("category repository database is not configured")
	}
	rows, err := findCategoryRows(ctx, r.db.WithContext(ctx).Table("listing_category"), query)
	if err != nil {
		return nil, err
	}
	items := make([]Category, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toCategory())
	}
	return items, nil
}

func (r *GormCategoryRepository) GetCategory(ctx context.Context, tenantID, id int64) (*Category, error) {
	var row listingCategory
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_category"), tenantID, id, "owner_user_id", &row, ErrCategoryNotFound)
	if err != nil {
		return nil, err
	}
	category := row.toCategory()
	return &category, nil
}

func (r *GormCategoryRepository) CreateCategory(ctx context.Context, category *Category) (*Category, error) {
	row := listingCategoryFromCategory(category)
	applyCategoryDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyCategoryAuditFields(&row, ownerUserID, true)
	}
	if row.ParentID > 0 {
		if _, err := r.GetCategory(ctx, row.TenantID, row.ParentID); err != nil {
			return nil, err
		}
	}
	if err := r.db.WithContext(ctx).Table("listing_category").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toCategory()
	return &created, nil
}

func (r *GormCategoryRepository) UpdateCategory(ctx context.Context, category *Category) (*Category, error) {
	row := listingCategoryFromCategory(category)
	applyCategoryDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyCategoryAuditFields(&row, ownerUserID, false)
	}
	updates := map[string]any{
		"owner_user_id": row.OwnerUserID,
		"name":          row.Name,
		"code":          row.Code,
		"parent_id":     row.ParentID,
		"level":         row.Level,
		"sort":          row.Sort,
		"icon":          row.Icon,
		"image":         row.Image,
		"description":   row.Description,
		"status":        row.Status,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_category"), row.TenantID, row.ID, "owner_user_id", updates, ErrCategoryNotFound); err != nil {
		return nil, err
	}
	return r.GetCategory(ctx, row.TenantID, row.ID)
}

func (r *GormCategoryRepository) UpdateCategoryStatus(ctx context.Context, tenantID, id int64, status int16) (*Category, error) {
	updates := map[string]any{"status": status}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_category"), tenantID, id, "owner_user_id", updates, ErrCategoryNotFound); err != nil {
		return nil, err
	}
	return r.GetCategory(ctx, tenantID, id)
}

func (r *GormCategoryRepository) DeleteCategory(ctx context.Context, tenantID, id int64) error {
	var childCount int64
	if err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_category").Where("tenant_id = ? AND parent_id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Count(&childCount).Error; err != nil {
		return err
	}
	if childCount > 0 {
		return ErrCategoryHasChildren
	}
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_category"), tenantID, id, "owner_user_id", updates, ErrCategoryNotFound)
}
