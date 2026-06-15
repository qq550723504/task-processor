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
