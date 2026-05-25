package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrImportTaskNotFound = errors.New("import task not found")

type ImportTask struct {
	ID             int64      `json:"id"`
	TenantID       int64      `json:"tenantId"`
	StoreID        *int64     `json:"storeId,omitempty"`
	Platform       string     `json:"platform"`
	TargetPlatform string     `json:"targetPlatform,omitempty"`
	SourcePlatform string     `json:"sourcePlatform,omitempty"`
	Region         string     `json:"region"`
	CategoryID     *int64     `json:"categoryId,omitempty"`
	ProductID      string     `json:"productId"`
	Status         int16      `json:"status"`
	ErrorMessage   string     `json:"errorMessage,omitempty"`
	ReasonCode     string     `json:"reasonCode,omitempty"`
	Stage          string     `json:"stage,omitempty"`
	RetryCount     int        `json:"retryCount"`
	MaxRetryCount  int        `json:"maxRetryCount"`
	Remark         string     `json:"remark,omitempty"`
	Priority       int        `json:"priority"`
	CreateTime     *time.Time `json:"createTime,omitempty"`
	UpdateTime     *time.Time `json:"updateTime,omitempty"`
}

type ImportTaskQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	StoreID     *int64
	Platform    string
	Region      string
	CategoryID  *int64
	ProductID   string
	Status      *int16
}

type ImportTaskPage struct {
	Items    []ImportTask `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

type ImportTaskRepository interface {
	ListImportTasks(ctx context.Context, query ImportTaskQuery) (*ImportTaskPage, error)
	BatchCreateImportTasks(ctx context.Context, tasks []ImportTask) ([]ImportTask, error)
	DeleteImportTask(ctx context.Context, tenantID, id int64) error
}

type listingProductImportTask struct {
	ID             int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID       int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID    string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	StoreID        int64      `gorm:"column:store_id;not null;index"`
	Platform       string     `gorm:"column:platform;not null"`
	TargetPlatform string     `gorm:"column:target_platform"`
	SourcePlatform string     `gorm:"column:source_platform"`
	Region         string     `gorm:"column:region;not null"`
	CategoryID     int64      `gorm:"column:category_id;not null;index"`
	ProductID      string     `gorm:"column:product_id;not null;index"`
	Status         int16      `gorm:"column:status;not null;default:0;index"`
	ErrorMessage   string     `gorm:"column:error_message"`
	ReasonCode     string     `gorm:"column:reason_code"`
	Stage          string     `gorm:"column:stage"`
	RetryCount     int        `gorm:"column:retry_count;not null;default:0"`
	MaxRetryCount  int        `gorm:"column:max_retry_count;not null;default:3"`
	Remark         string     `gorm:"column:remark"`
	Priority       int        `gorm:"column:priority;not null;default:5"`
	Creator        string     `gorm:"column:creator"`
	CreatedBy      string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime     *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater        string     `gorm:"column:updater"`
	UpdatedBy      string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime     *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted        int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingProductImportTask) TableName() string {
	return "listing_product_import_task"
}

func (t listingProductImportTask) toImportTask() ImportTask {
	storeID := t.StoreID
	categoryID := t.CategoryID
	return ImportTask{
		ID:             t.ID,
		TenantID:       t.TenantID,
		StoreID:        &storeID,
		Platform:       t.Platform,
		TargetPlatform: t.TargetPlatform,
		SourcePlatform: t.SourcePlatform,
		Region:         t.Region,
		CategoryID:     &categoryID,
		ProductID:      t.ProductID,
		Status:         t.Status,
		ErrorMessage:   t.ErrorMessage,
		ReasonCode:     t.ReasonCode,
		Stage:          t.Stage,
		RetryCount:     t.RetryCount,
		MaxRetryCount:  t.MaxRetryCount,
		Remark:         t.Remark,
		Priority:       t.Priority,
		CreateTime:     t.CreateTime,
		UpdateTime:     t.UpdateTime,
	}
}

func listingProductImportTaskFromImportTask(task ImportTask) listingProductImportTask {
	var storeID int64
	if task.StoreID != nil {
		storeID = *task.StoreID
	}
	var categoryID int64
	if task.CategoryID != nil {
		categoryID = *task.CategoryID
	}
	sourcePlatform := strings.TrimSpace(task.SourcePlatform)
	if sourcePlatform == "" {
		sourcePlatform = strings.TrimSpace(task.Platform)
	}
	return listingProductImportTask{
		ID:             task.ID,
		TenantID:       task.TenantID,
		StoreID:        storeID,
		Platform:       strings.TrimSpace(task.Platform),
		TargetPlatform: strings.TrimSpace(task.TargetPlatform),
		SourcePlatform: sourcePlatform,
		Region:         strings.TrimSpace(task.Region),
		CategoryID:     categoryID,
		ProductID:      strings.TrimSpace(task.ProductID),
		Status:         task.Status,
		ErrorMessage:   strings.TrimSpace(task.ErrorMessage),
		ReasonCode:     strings.TrimSpace(task.ReasonCode),
		Stage:          strings.TrimSpace(task.Stage),
		RetryCount:     task.RetryCount,
		MaxRetryCount:  task.MaxRetryCount,
		Remark:         strings.TrimSpace(task.Remark),
		Priority:       task.Priority,
	}
}

type GormImportTaskRepository struct {
	db *gorm.DB
}

func NewGormImportTaskRepository(db *gorm.DB) *GormImportTaskRepository {
	return &GormImportTaskRepository{db: db}
}

func AutoMigrateImportTaskRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingProductImportTask{}).TableName())
}

func (r *GormImportTaskRepository) ListImportTasks(ctx context.Context, query ImportTaskQuery) (*ImportTaskPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	db := applyImportTaskQuery(r.db.WithContext(ctx).Table("listing_product_import_task"), query)
	var rows []listingProductImportTask
	total, page, pageSize, err := findPagedRows(db, query.Page, query.PageSize, &rows)
	if err != nil {
		return nil, err
	}
	items := make([]ImportTask, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toImportTask())
	}
	return &ImportTaskPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormImportTaskRepository) BatchCreateImportTasks(ctx context.Context, tasks []ImportTask) ([]ImportTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	rows := make([]listingProductImportTask, 0, len(tasks))
	for _, task := range tasks {
		row := listingProductImportTaskFromImportTask(task)
		if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
			row.OwnerUserID = ownerUserID
			row.Creator = ownerUserID
			row.CreatedBy = ownerUserID
			row.Updater = ownerUserID
			row.UpdatedBy = ownerUserID
		}
		if row.Region == "" {
			row.Region = "US"
		}
		if row.Priority <= 0 {
			row.Priority = 5
		}
		if row.MaxRetryCount <= 0 {
			row.MaxRetryCount = 3
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return []ImportTask{}, nil
	}
	if err := r.db.WithContext(ctx).Table("listing_product_import_task").Create(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ImportTask, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toImportTask())
	}
	return out, nil
}

func (r *GormImportTaskRepository) DeleteImportTask(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_import_task").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrImportTaskNotFound
	}
	return nil
}

func applyImportTaskQuery(db *gorm.DB, query ImportTaskQuery) *gorm.DB {
	db = db.Where("deleted = 0")
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if ownerScopeEnabled() && strings.TrimSpace(query.OwnerUserID) != "" {
		db = db.Where("owner_user_id = ?", strings.TrimSpace(query.OwnerUserID))
	}
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.Region != "" {
		db = db.Where("region = ?", query.Region)
	}
	if query.CategoryID != nil {
		db = db.Where("category_id = ?", *query.CategoryID)
	}
	if query.ProductID != "" {
		db = db.Where("product_id LIKE ?", "%"+query.ProductID+"%")
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
