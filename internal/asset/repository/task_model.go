package repository

import (
	"time"

	assetgeneration "task-processor/internal/asset/generation"
)

type GenerationTaskSnapshot struct {
	ID        string                `gorm:"primaryKey;type:varchar(128)"`
	TaskID    string                `gorm:"index;type:varchar(64)"`
	Platform  string                `gorm:"index;type:varchar(32)"`
	RecipeID  string                `gorm:"index;type:varchar(128)"`
	Payload   *assetgeneration.Task `gorm:"type:text"`
	CreatedAt time.Time             `gorm:"autoCreateTime"`
	UpdatedAt time.Time             `gorm:"autoUpdateTime"`
}
