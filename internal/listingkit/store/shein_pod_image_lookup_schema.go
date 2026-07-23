package store

import (
	"fmt"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

var legacySheinPODImageLookupTextIndexes = []string{
	"idx_shein_pod_image_lookup_task_id",
	"idx_shein_pod_image_lookup_product_name",
	"idx_shein_pod_image_lookup_supplier_code",
	"idx_shein_pod_image_lookup_seller_sku",
	"idx_shein_pod_image_lookup_shein_spu_name",
	"idx_shein_pod_image_lookup_shein_version",
	"idx_shein_pod_image_lookup_ai_original_image_url",
	"idx_shein_pod_image_lookup_ai_original_image_key",
	"idx_shein_pod_image_lookup_sds_main_image_url",
}

// AutoMigrateSheinPODImageLookupIndex migrates only the POD image lookup
// projection. Legacy indexes over unbounded normalized text are removed after
// their fixed-length hash-key replacements have been created.
func AutoMigrateSheinPODImageLookupIndex(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := db.AutoMigrate(&listingkit.SheinPODImageLookupIndex{}); err != nil {
		return err
	}
	for _, indexName := range legacySheinPODImageLookupTextIndexes {
		if !db.Migrator().HasIndex(&listingkit.SheinPODImageLookupIndex{}, indexName) {
			continue
		}
		if err := db.Migrator().DropIndex(&listingkit.SheinPODImageLookupIndex{}, indexName); err != nil {
			return fmt.Errorf("drop legacy POD image lookup index %q: %w", indexName, err)
		}
	}
	return nil
}
