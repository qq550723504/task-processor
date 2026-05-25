package listingadmin

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

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
	rows, total, page, pageSize, err := findImportTaskRows(ctx, r.db.WithContext(ctx).Table("listing_product_import_task"), query)
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
		applyImportTaskDefaults(&row)
		if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
			applyImportTaskAuditFields(&row, ownerUserID, true)
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
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_import_task"), tenantID, id, "owner_user_id", updates, ErrImportTaskNotFound)
}
