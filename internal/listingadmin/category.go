package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrCategoryNotFound    = errors.New("category not found")
	ErrCategoryHasChildren = errors.New("category has children")
)

type Category struct {
	ID          int64      `json:"id"`
	TenantID    int64      `json:"tenantId"`
	Name        string     `json:"name"`
	Code        string     `json:"code"`
	ParentID    int64      `json:"parentId"`
	Level       int        `json:"level"`
	Sort        int        `json:"sort"`
	Icon        string     `json:"icon,omitempty"`
	Image       string     `json:"image,omitempty"`
	Description string     `json:"description,omitempty"`
	Status      int16      `json:"status"`
	CreateTime  *time.Time `json:"createTime,omitempty"`
	UpdateTime  *time.Time `json:"updateTime,omitempty"`
	Children    []Category `json:"children,omitempty"`
}

type CategoryQuery struct {
	TenantID    int64
	OwnerUserID string
	Name        string
	Code        string
	ParentID    *int64
	Level       *int
	Status      *int16
}

type CategoryRepository interface {
	ListCategories(ctx context.Context, query CategoryQuery) ([]Category, error)
	GetCategory(ctx context.Context, tenantID, id int64) (*Category, error)
	CreateCategory(ctx context.Context, category *Category) (*Category, error)
	UpdateCategory(ctx context.Context, category *Category) (*Category, error)
	UpdateCategoryStatus(ctx context.Context, tenantID, id int64, status int16) (*Category, error)
	DeleteCategory(ctx context.Context, tenantID, id int64) error
}

type listingCategory struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID    int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Name        string     `gorm:"column:name;not null;index"`
	Code        string     `gorm:"column:code;not null;index"`
	ParentID    int64      `gorm:"column:parent_id;not null;default:0;index"`
	Level       int        `gorm:"column:level;not null;default:1"`
	Sort        int        `gorm:"column:sort;not null;default:0"`
	Icon        string     `gorm:"column:icon"`
	Image       string     `gorm:"column:image"`
	Description string     `gorm:"column:description"`
	Status      int16      `gorm:"column:status;not null;default:0;index"`
	Creator     string     `gorm:"column:creator"`
	CreatedBy   string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime  *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater     string     `gorm:"column:updater"`
	UpdatedBy   string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime  *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted     int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingCategory) TableName() string {
	return "listing_category"
}

func (r listingCategory) toCategory() Category {
	return Category{
		ID:          r.ID,
		TenantID:    r.TenantID,
		Name:        r.Name,
		Code:        r.Code,
		ParentID:    r.ParentID,
		Level:       r.Level,
		Sort:        r.Sort,
		Icon:        r.Icon,
		Image:       r.Image,
		Description: r.Description,
		Status:      r.Status,
		CreateTime:  r.CreateTime,
		UpdateTime:  r.UpdateTime,
	}
}

func listingCategoryFromCategory(category *Category) listingCategory {
	if category == nil {
		return listingCategory{}
	}
	return listingCategory{
		ID:          category.ID,
		TenantID:    category.TenantID,
		Name:        strings.TrimSpace(category.Name),
		Code:        strings.TrimSpace(category.Code),
		ParentID:    category.ParentID,
		Level:       category.Level,
		Sort:        category.Sort,
		Icon:        strings.TrimSpace(category.Icon),
		Image:       strings.TrimSpace(category.Image),
		Description: strings.TrimSpace(category.Description),
		Status:      category.Status,
	}
}

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
	db := applyCategoryQuery(r.db.WithContext(ctx).Table("listing_category"), query)
	var rows []listingCategory
	if err := db.Order("parent_id asc, sort asc, id asc").Find(&rows).Error; err != nil {
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
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_category").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCategoryNotFound
	}
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
		row.OwnerUserID = ownerUserID
		row.Creator = ownerUserID
		row.CreatedBy = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
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
		row.OwnerUserID = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
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
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_category").Where("tenant_id = ? AND id = ? AND deleted = 0", row.TenantID, row.ID),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrCategoryNotFound
	}
	return r.GetCategory(ctx, row.TenantID, row.ID)
}

func (r *GormCategoryRepository) UpdateCategoryStatus(ctx context.Context, tenantID, id int64, status int16) (*Category, error) {
	updates := map[string]any{"status": status}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_category").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrCategoryNotFound
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
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_category").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

func applyCategoryDefaults(row *listingCategory) {
	if row.Level <= 0 {
		row.Level = 1
	}
}

func applyCategoryQuery(db *gorm.DB, query CategoryQuery) *gorm.DB {
	db = db.Where("tenant_id = ? AND deleted = 0", query.TenantID)
	if ownerScopeEnabled() && strings.TrimSpace(query.OwnerUserID) != "" {
		db = db.Where("owner_user_id = ?", strings.TrimSpace(query.OwnerUserID))
	}
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Code != "" {
		db = db.Where("code = ?", query.Code)
	}
	if query.ParentID != nil {
		db = db.Where("parent_id = ?", *query.ParentID)
	}
	if query.Level != nil {
		db = db.Where("level = ?", *query.Level)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
