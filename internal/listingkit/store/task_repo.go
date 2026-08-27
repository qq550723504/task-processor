package store

import (
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

type taskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) listingkit.Repository {
	return &taskRepository{db: db}
}

// NewTaskResultTransactionRepository exposes only the canonical locked task
// result mutation boundary to runtime adapters such as image-agent approval.
func NewTaskResultTransactionRepository(db *gorm.DB) listingkit.TaskResultTransactionRepository {
	return &taskRepository{db: db}
}
