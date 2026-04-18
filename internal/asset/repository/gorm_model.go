package repository

import (
	"time"

	"task-processor/internal/asset"
)

type InventorySnapshot struct {
	TaskID     string           `gorm:"primaryKey;type:varchar(64)"`
	ProductKey string           `gorm:"type:varchar(128);index"`
	Payload    *asset.Inventory `gorm:"type:text"`
	CreatedAt  time.Time        `gorm:"autoCreateTime"`
	UpdatedAt  time.Time        `gorm:"autoUpdateTime"`
}
