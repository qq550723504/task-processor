package store

import (
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

type GormSheinSyncRepository struct {
	db *gorm.DB
}

func NewSheinSyncRepository(db *gorm.DB) listingkit.SheinSyncRepository {
	return &GormSheinSyncRepository{db: db}
}

func AutoMigrateSheinSyncRepository(db *gorm.DB) error {
	return db.AutoMigrate(
		&listingkit.SheinSyncedProductRecord{},
		&listingkit.SheinSDSCostGroupRecord{},
		&listingkit.SheinSyncJobRecord{},
		&listingkit.SheinActivityCandidateRecord{},
		&listingkit.SheinActivityEnrollmentRunRecord{},
		&listingkit.SheinActivityEnrollmentItemRecord{},
	)
}
