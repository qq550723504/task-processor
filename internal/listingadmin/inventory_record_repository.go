package listingadmin

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type GormInventoryRecordRepository struct{ db *gorm.DB }

func NewGormInventoryRecordRepository(db *gorm.DB) *GormInventoryRecordRepository {
	return &GormInventoryRecordRepository{db: db}
}

func (r *GormInventoryRecordRepository) CreateInventoryRecord(ctx context.Context, record *InventoryRecord) (*InventoryRecord, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("inventory record repository database is not configured")
	}
	if record == nil {
		return nil, nil
	}
	row := listingInventoryRecordFromInventoryRecord(record)
	if row.CreateTime == nil {
		now := time.Now()
		row.CreateTime = &now
	}
	if err := r.db.WithContext(ctx).Table(row.TableName()).Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toInventoryRecord()
	return &created, nil
}

func (r *GormInventoryRecordRepository) GetLatestInventoryRecord(ctx context.Context, platform, productID, region string) (*InventoryRecord, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("inventory record repository database is not configured")
	}
	var row listingInventoryRecord
	err := r.db.WithContext(ctx).Table(row.TableName()).
		Where("platform = ? AND product_id = ? AND region = ?", platform, productID, region).
		Order("create_time desc, id desc").
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record := row.toInventoryRecord()
	return &record, nil
}
