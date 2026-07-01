package listingadmin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormScheduledTaskConfigRepository struct{ db *gorm.DB }

func NewGormScheduledTaskConfigRepository(db *gorm.DB) *GormScheduledTaskConfigRepository {
	return &GormScheduledTaskConfigRepository{db: db}
}

func AutoMigrateScheduledTaskConfigRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	if err := db.AutoMigrate(&listingScheduledTaskConfig{}); err != nil {
		return err
	}
	if err := ensureOwnerAuditColumns(db, (listingScheduledTaskConfig{}).TableName()); err != nil {
		return err
	}
	return ensureUniqueIndex(
		db,
		(listingScheduledTaskConfig{}).TableName(),
		"uk_listing_scheduled_task_config_scope",
		"tenant_id",
		"store_id",
		"platform",
		"task_type",
	)
}

func (r *GormScheduledTaskConfigRepository) ListScheduledTaskConfigs(ctx context.Context, query ScheduledTaskConfigQuery) (*ScheduledTaskConfigPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("scheduled task config repository database is not configured")
	}
	rows, total, page, pageSize, err := findScheduledTaskConfigRows(ctx, r.db.WithContext(ctx).Table("listing_scheduled_task_config"), query)
	if err != nil {
		return nil, err
	}
	items := make([]ScheduledTaskConfig, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toScheduledTaskConfig())
	}
	return &ScheduledTaskConfigPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormScheduledTaskConfigRepository) GetScheduledTaskConfig(ctx context.Context, tenantID, id int64) (*ScheduledTaskConfig, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("scheduled task config repository database is not configured")
	}
	var row listingScheduledTaskConfig
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_scheduled_task_config"), tenantID, id, "owner_user_id", &row, ErrScheduledTaskConfigNotFound)
	if err != nil {
		return nil, err
	}
	item := row.toScheduledTaskConfig()
	return &item, nil
}

func (r *GormScheduledTaskConfigRepository) UpsertScheduledTaskConfig(ctx context.Context, config *ScheduledTaskConfig) (*ScheduledTaskConfig, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("scheduled task config repository database is not configured")
	}
	row := listingScheduledTaskConfigFromScheduledTaskConfig(config)
	if err := validateScheduledTaskConfigRow(row); err != nil {
		return nil, err
	}
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Creator = ownerUserID
		row.CreatedBy = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "store_id"},
				{Name: "platform"},
				{Name: "task_type"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"owner_user_id",
				"enabled",
				"interval_seconds",
				"remark",
				"updater",
				"updated_by",
				"update_time",
				"deleted",
			}),
		}).
		Create(&row).Error
	if err != nil {
		return nil, err
	}
	return r.findByScope(ctx, row.TenantID, row.StoreID, row.Platform, row.TaskType)
}

func (r *GormScheduledTaskConfigRepository) UpdateScheduledTaskConfigStatus(ctx context.Context, tenantID, id int64, enabled bool, remark string) (*ScheduledTaskConfig, error) {
	updates := map[string]any{
		"enabled": boolToInt16(enabled),
	}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_scheduled_task_config"), tenantID, id, "owner_user_id", updates, ErrScheduledTaskConfigNotFound); err != nil {
		return nil, err
	}
	return r.GetScheduledTaskConfig(ctx, tenantID, id)
}

func (r *GormScheduledTaskConfigRepository) DeleteScheduledTaskConfig(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_scheduled_task_config"), tenantID, id, "owner_user_id", updates, ErrScheduledTaskConfigNotFound)
}

func (r *GormScheduledTaskConfigRepository) ListEnabledScheduledTaskConfigs(ctx context.Context, platform, taskType string) ([]ScheduledTaskConfig, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("scheduled task config repository database is not configured")
	}
	var rows []listingScheduledTaskConfig
	err := r.db.WithContext(ctx).
		Table("listing_scheduled_task_config").
		Where("deleted = 0 AND enabled = ? AND platform = ? AND task_type = ?", int16(1), normalizeScheduledTaskPlatform(platform), normalizeScheduledTaskType(taskType)).
		Order("store_id asc, id desc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]ScheduledTaskConfig, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toScheduledTaskConfig())
	}
	return items, nil
}

func (r *GormScheduledTaskConfigRepository) findByScope(ctx context.Context, tenantID, storeID int64, platform, taskType string) (*ScheduledTaskConfig, error) {
	var row listingScheduledTaskConfig
	err := r.db.WithContext(ctx).
		Table("listing_scheduled_task_config").
		Where("tenant_id = ? AND store_id = ? AND platform = ? AND task_type = ? AND deleted = 0", tenantID, storeID, platform, taskType).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrScheduledTaskConfigNotFound
	}
	if err != nil {
		return nil, err
	}
	item := row.toScheduledTaskConfig()
	return &item, nil
}

func validateScheduledTaskConfigRow(row listingScheduledTaskConfig) error {
	switch {
	case row.TenantID <= 0:
		return errors.New("tenant id is required")
	case row.StoreID <= 0:
		return errors.New("store id is required")
	case strings.TrimSpace(row.Platform) == "":
		return errors.New("platform is required")
	case strings.TrimSpace(row.TaskType) == "":
		return errors.New("task type is required")
	case row.IntervalSeconds <= 0:
		return errors.New("interval seconds must be positive")
	default:
		return nil
	}
}
