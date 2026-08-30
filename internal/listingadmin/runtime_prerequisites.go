package listingadmin

import (
	"errors"

	"gorm.io/gorm"
)

// AutoMigrateRuntimePrerequisites creates the listing-admin tables that the
// repository migration helpers evolve in place. Keeping this bootstrap in the
// owning package lets the aggregate ListingKit runtime start from a blank
// database while preserving the legacy migration helpers' behavior for
// upgrades of existing databases.
func AutoMigrateRuntimePrerequisites(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return db.AutoMigrate(
		&listingStore{},
		&listingProductImportTask{},
		&listingFilterRule{},
		&listingProfitRule{},
		&listingPricingRule{},
		&listingOperationStrategy{},
		&listingSensitiveWord{},
		&listingProductImportMapping{},
		&listingCategory{},
		&listingProductData{},
	)
}
