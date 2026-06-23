package listingadmin

import (
	"context"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"
)

type GormRawJSONDataRepository struct {
	db *gorm.DB

	columnsOnce sync.Once
	columns     map[string]bool
	columnsErr  error
}

func NewGormRawJSONDataRepository(db *gorm.DB) *GormRawJSONDataRepository {
	return &GormRawJSONDataRepository{db: db}
}

func (r *GormRawJSONDataRepository) GetLatestRawJSONData(ctx context.Context, platform, productID, region string) (*RawJSONData, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("raw json data repository database is not configured")
	}
	var row listingRawJSONData
	err := r.db.WithContext(ctx).Table(row.TableName()).
		Where("deleted = ? AND platform = ? AND product_id = ? AND region = ?", 0, platform, productID, region).
		Order("id desc").
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record := row.toRawJSONData()
	return &record, nil
}

func (r *GormRawJSONDataRepository) UpsertRawJSONData(ctx context.Context, record *RawJSONData) (*RawJSONData, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("raw json data repository database is not configured")
	}
	if record == nil {
		return nil, nil
	}
	columns, err := r.tableColumns()
	if err != nil {
		return nil, err
	}
	if existing, err := r.GetLatestRawJSONData(ctx, record.Platform, record.ProductID, record.Region); err != nil {
		return nil, err
	} else if existing != nil {
		updates := map[string]any{
			"raw_json_data": record.RawJSONData,
			"update_time":   time.Now(),
		}
		if columns["store_id"] && record.StoreID > 0 {
			updates["store_id"] = record.StoreID
		}
		if columns["import_task_id"] && record.ImportTaskID > 0 {
			updates["import_task_id"] = record.ImportTaskID
		}
		if columns["category_id"] && record.CategoryID > 0 {
			updates["category_id"] = record.CategoryID
		}
		if columns["creator"] && record.Creator != "" {
			updates["creator"] = record.Creator
		}
		if columns["updater"] && record.Updater != "" {
			updates["updater"] = record.Updater
		} else if columns["updater"] && record.Creator != "" {
			updates["updater"] = record.Creator
		}
		if err := r.db.WithContext(ctx).Table((listingRawJSONData{}).TableName()).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		return r.GetLatestRawJSONData(ctx, record.Platform, record.ProductID, record.Region)
	}

	now := time.Now()
	createTime := record.CreateTime
	if createTime == nil {
		createTime = &now
	}
	updateTime := record.UpdateTime
	if updateTime == nil {
		updateTime = &now
	}
	values := map[string]any{
		"platform":      record.Platform,
		"product_id":    record.ProductID,
		"region":        record.Region,
		"raw_json_data": record.RawJSONData,
		"status":        record.Status,
		"create_time":   createTime,
		"update_time":   updateTime,
		"deleted":       0,
	}
	if columns["store_id"] && record.StoreID > 0 {
		values["store_id"] = record.StoreID
	}
	if columns["import_task_id"] && record.ImportTaskID > 0 {
		values["import_task_id"] = record.ImportTaskID
	}
	if columns["category_id"] && record.CategoryID > 0 {
		values["category_id"] = record.CategoryID
	}
	if columns["creator"] && record.Creator != "" {
		values["creator"] = record.Creator
		if columns["updater"] && record.Updater == "" {
			values["updater"] = record.Creator
		}
	}
	if columns["updater"] && record.Updater != "" {
		values["updater"] = record.Updater
	}
	if err := r.db.WithContext(ctx).Table((listingRawJSONData{}).TableName()).Create(values).Error; err != nil {
		return nil, err
	}
	return r.GetLatestRawJSONData(ctx, record.Platform, record.ProductID, record.Region)
}

func (r *GormRawJSONDataRepository) tableColumns() (map[string]bool, error) {
	r.columnsOnce.Do(func() {
		table := (listingRawJSONData{}).TableName()
		columns := map[string]bool{}
		for _, column := range []string{
			"id",
			"store_id",
			"import_task_id",
			"platform",
			"product_id",
			"region",
			"category_id",
			"raw_json_data",
			"status",
			"creator",
			"updater",
			"create_time",
			"update_time",
			"deleted",
		} {
			columns[column] = r.db.Migrator().HasColumn(table, column)
		}
		r.columns = columns
	})
	return r.columns, r.columnsErr
}
