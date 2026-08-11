package schema

import "gorm.io/gorm"

const systemOwnedExceptionRegistryTable = "listingkit_owner_scope_system_owned_exceptions"

func autoMigrateSystemOwnedExceptionRegistry(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS listingkit_owner_scope_system_owned_exceptions (
        id BIGSERIAL PRIMARY KEY,
        table_name VARCHAR(128) NOT NULL,
        tenant_fingerprint VARCHAR(64) NOT NULL,
        candidate_fingerprint VARCHAR(64) NOT NULL,
        report_fingerprint VARCHAR(32) NOT NULL,
        reason TEXT NOT NULL,
        active BOOLEAN NOT NULL DEFAULT TRUE,
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        CONSTRAINT listingkit_owner_scope_system_owned_exceptions_key
            UNIQUE (table_name, tenant_fingerprint, candidate_fingerprint)
    )`).Error; err != nil {
		return err
	}
	return db.Exec(`CREATE INDEX IF NOT EXISTS listingkit_owner_scope_system_owned_exceptions_active_idx
        ON listingkit_owner_scope_system_owned_exceptions (active, table_name, tenant_fingerprint)`).Error
}
