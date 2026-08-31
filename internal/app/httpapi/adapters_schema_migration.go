package httpapi

import (
	"gorm.io/gorm"

	productlisting_schema "task-processor/internal/app/schema/productlisting"
)

func AutoMigrateProductListingAPIRuntimeSchema(db *gorm.DB) error {
	return productlisting_schema.AutoMigrateRuntime(db)
}
