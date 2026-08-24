package schema

import "gorm.io/gorm"

const systemOwnedExceptionRegistryTable = "listingkit_owner_scope_system_owned_exceptions"
const systemOwnedExceptionRegistryIndex = "listingkit_owner_scope_system_owned_exceptions_active_idx"

func autoMigrateSystemOwnedExceptionRegistry(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS listingkit_owner_scope_system_owned_exceptions (
        id BIGSERIAL PRIMARY KEY,
        table_name VARCHAR(128) NOT NULL,
        tenant_fingerprint VARCHAR(64) NOT NULL,
        candidate_fingerprint VARCHAR(64) NOT NULL,
        report_fingerprint VARCHAR(32) NOT NULL,
        reason TEXT NOT NULL,
        row_count BIGINT NOT NULL DEFAULT 0,
        active BOOLEAN NOT NULL DEFAULT TRUE,
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        CONSTRAINT listingkit_owner_scope_system_owned_exceptions_key
            UNIQUE (table_name, tenant_fingerprint, candidate_fingerprint)
    )`).Error; err != nil {
		return err
	}
	columns, err := db.Migrator().ColumnTypes(systemOwnedExceptionRegistryTable)
	if err != nil {
		return err
	}
	hasRowCount := false
	for _, column := range columns {
		if column.Name() == "row_count" {
			hasRowCount = true
			break
		}
	}
	if !hasRowCount {
		if err := db.Exec(`ALTER TABLE listingkit_owner_scope_system_owned_exceptions
        ADD COLUMN row_count BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasIndex(systemOwnedExceptionRegistryTable, systemOwnedExceptionRegistryIndex) {
		return nil
	}
	return db.Exec(`CREATE INDEX listingkit_owner_scope_system_owned_exceptions_active_idx
		ON listingkit_owner_scope_system_owned_exceptions (active, table_name, tenant_fingerprint)`).Error
}
