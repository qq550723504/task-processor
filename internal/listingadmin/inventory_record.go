package listingadmin

import (
	"context"
	"errors"
	"time"
)

var ErrInventoryRecordNotFound = errors.New("inventory record not found")

type InventoryRecord struct {
	ID                 int64      `json:"id"`
	Platform           string     `json:"platform"`
	ProductID          string     `json:"productId"`
	Region             string     `json:"region"`
	Stock              *int       `json:"stock,omitempty"`
	StockStatus        string     `json:"stockStatus,omitempty"`
	IsAvailable        bool       `json:"isAvailable"`
	OriginalPrice      *float64   `json:"originalPrice,omitempty"`
	CurrentPrice       *float64   `json:"currentPrice,omitempty"`
	Currency           string     `json:"currency,omitempty"`
	PriceChangePercent *float64   `json:"priceChangePercent,omitempty"`
	SyncSource         string     `json:"syncSource,omitempty"`
	Remark             string     `json:"remark,omitempty"`
	CreateTime         *time.Time `json:"createTime,omitempty"`
}

type InventoryRecordRepository interface {
	CreateInventoryRecord(ctx context.Context, record *InventoryRecord) (*InventoryRecord, error)
	GetLatestInventoryRecord(ctx context.Context, platform, productID, region string) (*InventoryRecord, error)
}

type listingInventoryRecord struct {
	ID                 int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Platform           string     `gorm:"column:platform;not null;index:idx_inventory_lookup,priority:1"`
	ProductID          string     `gorm:"column:product_id;not null;index:idx_inventory_lookup,priority:2"`
	Region             string     `gorm:"column:region;not null;index:idx_inventory_lookup,priority:3"`
	Stock              *int       `gorm:"column:stock"`
	StockStatus        string     `gorm:"column:stock_status"`
	IsAvailable        bool       `gorm:"column:is_available;not null"`
	OriginalPrice      *float64   `gorm:"column:original_price"`
	CurrentPrice       *float64   `gorm:"column:current_price"`
	Currency           string     `gorm:"column:currency"`
	PriceChangePercent *float64   `gorm:"column:price_change_percent"`
	SyncSource         string     `gorm:"column:sync_source"`
	Remark             string     `gorm:"column:remark"`
	CreateTime         *time.Time `gorm:"column:create_time;autoCreateTime"`
}

func (listingInventoryRecord) TableName() string {
	return "listing_inventory_record"
}

func (r listingInventoryRecord) toInventoryRecord() InventoryRecord {
	return InventoryRecord{
		ID:                 r.ID,
		Platform:           r.Platform,
		ProductID:          r.ProductID,
		Region:             r.Region,
		Stock:              r.Stock,
		StockStatus:        r.StockStatus,
		IsAvailable:        r.IsAvailable,
		OriginalPrice:      r.OriginalPrice,
		CurrentPrice:       r.CurrentPrice,
		Currency:           r.Currency,
		PriceChangePercent: r.PriceChangePercent,
		SyncSource:         r.SyncSource,
		Remark:             r.Remark,
		CreateTime:         r.CreateTime,
	}
}

func listingInventoryRecordFromInventoryRecord(record *InventoryRecord) listingInventoryRecord {
	if record == nil {
		return listingInventoryRecord{}
	}
	return listingInventoryRecord{
		ID:                 record.ID,
		Platform:           record.Platform,
		ProductID:          record.ProductID,
		Region:             record.Region,
		Stock:              record.Stock,
		StockStatus:        record.StockStatus,
		IsAvailable:        record.IsAvailable,
		OriginalPrice:      record.OriginalPrice,
		CurrentPrice:       record.CurrentPrice,
		Currency:           record.Currency,
		PriceChangePercent: record.PriceChangePercent,
		SyncSource:         record.SyncSource,
		Remark:             record.Remark,
		CreateTime:         record.CreateTime,
	}
}
