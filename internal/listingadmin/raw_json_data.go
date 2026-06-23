package listingadmin

import (
	"context"
	"errors"
	"time"
)

var ErrRawJSONDataNotFound = errors.New("raw json data not found")

type RawJSONData struct {
	ID           int64      `json:"id"`
	TenantID     int64      `json:"tenantId"`
	StoreID      int64      `json:"storeId"`
	ImportTaskID int64      `json:"importTaskId"`
	Platform     string     `json:"platform"`
	ProductID    string     `json:"productId"`
	Region       string     `json:"region"`
	CategoryID   int64      `json:"categoryId"`
	RawJSONData  string     `json:"rawJsonData"`
	Status       int16      `json:"status"`
	Creator      string     `json:"creator"`
	Updater      string     `json:"updater"`
	CreateTime   *time.Time `json:"createTime,omitempty"`
	UpdateTime   *time.Time `json:"updateTime,omitempty"`
}

type RawJSONDataRepository interface {
	GetLatestRawJSONData(ctx context.Context, platform, productID, region string) (*RawJSONData, error)
	UpsertRawJSONData(ctx context.Context, record *RawJSONData) (*RawJSONData, error)
}

type listingRawJSONData struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement"`
	StoreID      int64      `gorm:"column:store_id"`
	ImportTaskID int64      `gorm:"column:import_task_id"`
	Platform     string     `gorm:"column:platform;not null;index:idx_raw_json_lookup,priority:1"`
	ProductID    string     `gorm:"column:product_id;not null;index:idx_raw_json_lookup,priority:2"`
	Region       string     `gorm:"column:region;not null;index:idx_raw_json_lookup,priority:3"`
	CategoryID   int64      `gorm:"column:category_id"`
	RawJSONData  string     `gorm:"column:raw_json_data"`
	Status       int16      `gorm:"column:status;not null;default:0"`
	Creator      string     `gorm:"column:creator"`
	Updater      string     `gorm:"column:updater"`
	CreateTime   *time.Time `gorm:"column:create_time;autoCreateTime"`
	UpdateTime   *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted      int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingRawJSONData) TableName() string {
	return "listing_raw_json_data"
}

func (r listingRawJSONData) toRawJSONData() RawJSONData {
	return RawJSONData{
		ID:           r.ID,
		StoreID:      r.StoreID,
		ImportTaskID: r.ImportTaskID,
		Platform:     r.Platform,
		ProductID:    r.ProductID,
		Region:       r.Region,
		CategoryID:   r.CategoryID,
		RawJSONData:  r.RawJSONData,
		Status:       r.Status,
		Creator:      r.Creator,
		Updater:      r.Updater,
		CreateTime:   r.CreateTime,
		UpdateTime:   r.UpdateTime,
	}
}

func listingRawJSONDataFromRawJSONData(record *RawJSONData) listingRawJSONData {
	if record == nil {
		return listingRawJSONData{}
	}
	return listingRawJSONData{
		ID:           record.ID,
		StoreID:      record.StoreID,
		ImportTaskID: record.ImportTaskID,
		Platform:     record.Platform,
		ProductID:    record.ProductID,
		Region:       record.Region,
		CategoryID:   record.CategoryID,
		RawJSONData:  record.RawJSONData,
		Status:       record.Status,
		Creator:      record.Creator,
		Updater:      record.Updater,
		CreateTime:   record.CreateTime,
		UpdateTime:   record.UpdateTime,
	}
}
