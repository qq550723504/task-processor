package listingadmin

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type GormRawJSONDataRepository struct{ db *gorm.DB }

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
	if existing, err := r.GetLatestRawJSONData(ctx, record.Platform, record.ProductID, record.Region); err != nil {
		return nil, err
	} else if existing != nil {
		updates := map[string]any{
			"raw_json_data": record.RawJSONData,
			"update_time":   time.Now(),
		}
		if record.StoreID > 0 {
			updates["store_id"] = record.StoreID
		}
		if record.ImportTaskID > 0 {
			updates["import_task_id"] = record.ImportTaskID
		}
		if record.CategoryID > 0 {
			updates["category_id"] = record.CategoryID
		}
		if record.Creator != "" {
			updates["creator"] = record.Creator
		}
		if record.Updater != "" {
			updates["updater"] = record.Updater
		} else if record.Creator != "" {
			updates["updater"] = record.Creator
		}
		if err := r.db.WithContext(ctx).Table((listingRawJSONData{}).TableName()).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		return r.GetLatestRawJSONData(ctx, record.Platform, record.ProductID, record.Region)
	}

	row := listingRawJSONDataFromRawJSONData(record)
	if row.Status == 0 {
		row.Status = 0
	}
	if row.Creator != "" && row.Updater == "" {
		row.Updater = row.Creator
	}
	now := time.Now()
	if row.CreateTime == nil {
		row.CreateTime = &now
	}
	if row.UpdateTime == nil {
		row.UpdateTime = &now
	}
	if err := r.db.WithContext(ctx).Table(row.TableName()).Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toRawJSONData()
	return &created, nil
}
