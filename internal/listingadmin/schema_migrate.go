package listingadmin

import (
	"fmt"

	"gorm.io/gorm"
)

func ensureOwnerAuditColumns(db *gorm.DB, table string) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	statements := []string{
		fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN IF NOT EXISTS owner_user_id varchar(128)`, table),
		fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN IF NOT EXISTS created_by varchar(128)`, table),
		fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN IF NOT EXISTS updated_by varchar(128)`, table),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "idx_%s_owner_user_id" ON "%s" (owner_user_id)`, table, table),
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
