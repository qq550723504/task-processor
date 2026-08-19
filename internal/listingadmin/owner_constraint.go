package listingadmin

import (
	"fmt"

	"gorm.io/gorm"
)

func ownerUserIDConstraintSQL(table string) (string, string) {
	constraintName := "ck_" + table + "_owner_user_id_nonblank"
	statement := fmt.Sprintf(`DO $owner_constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint AS constraint_row
        JOIN pg_class AS table_row ON table_row.oid = constraint_row.conrelid
        WHERE constraint_row.conname = '%s'
          AND table_row.relname = '%s'
    ) THEN
        ALTER TABLE "%s"
            ADD CONSTRAINT "%s"
            CHECK (NULLIF(BTRIM(owner_user_id::text), '') IS NOT NULL) NOT VALID;
    END IF;
END
$owner_constraint$;`, constraintName, table, table, constraintName)
	return constraintName, statement
}

func ensureOwnerUserIDConstraint(db *gorm.DB, table string) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	_, statement := ownerUserIDConstraintSQL(table)
	return db.Exec(statement).Error
}
