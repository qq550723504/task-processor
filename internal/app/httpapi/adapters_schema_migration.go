package httpapi

import (
	"gorm.io/gorm"

	productlisting_schema "task-processor/internal/app/schema/productlisting"
	"task-processor/internal/core/config"
)

func shouldAutoMigrateProductListingAPIRuntime() bool {
	return config.ProductListingAPIRuntimeAutoMigrateEnabled()
}

func AutoMigrateProductListingAPIRuntimeSchema(db *gorm.DB) error {
	return productlisting_schema.AutoMigrateRuntime(db)
}
