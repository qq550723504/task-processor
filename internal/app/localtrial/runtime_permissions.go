package localtrial

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var trialWritableTables = []string{
	"product_snapshot_versions", "product_snapshot_heads",
	"product_source_publications", "product_source_publication_receipts",
	"product_title_proposals", "product_title_operations",
	"product_approved_assets", "product_approval_receipts",
	"product_approved_inventory_heads", "product_approved_inventory_version_heads",
	"listing_shein_records", "listing_shein_record_operations",
}

type trialTableRights struct {
	CanSelect bool `gorm:"column:can_select"`
	CanInsert bool `gorm:"column:can_insert"`
	CanUpdate bool `gorm:"column:can_update"`
	CanDelete bool `gorm:"column:can_delete"`
}

func tableRights(ctx context.Context, db *gorm.DB, name string) (trialTableRights, error) {
	var rights trialTableRights
	result := db.WithContext(ctx).Raw(`SELECT
    has_table_privilege(current_user,c.oid,'SELECT') AS can_select,
    has_table_privilege(current_user,c.oid,'INSERT') AS can_insert,
    has_table_privilege(current_user,c.oid,'UPDATE') AS can_update,
    has_table_privilege(current_user,c.oid,'DELETE') AS can_delete
  FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='public' AND c.relname=? AND c.relkind IN ('r','p')`, name).Scan(&rights)
	if result.Error != nil || result.RowsAffected != 1 {
		return trialTableRights{}, fmt.Errorf("local trial table %s unavailable", name)
	}
	return rights, nil
}

// VerifyRuntimePermissions fails closed before HTTP registration. The
// separately installed trial role can mutate only current Product/Asset/Review
// and Listing facts; Store Center remains the sole Store writer.
func VerifyRuntimePermissions(ctx context.Context, db *gorm.DB, expectedRole, expectedDatabase string) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" || expectedRole == "" || expectedDatabase == "" {
		return errors.New("local trial runtime database unavailable")
	}
	var session struct {
		Role           string `gorm:"column:role"`
		Database       string `gorm:"column:database"`
		SchemaCreate   bool   `gorm:"column:schema_create"`
		DatabaseCreate bool   `gorm:"column:database_create"`
	}
	if err := db.WithContext(ctx).Raw(`SELECT current_user AS role,current_database() AS database,
    has_schema_privilege(current_user,'public','CREATE') AS schema_create,
    has_database_privilege(current_user,current_database(),'CREATE') AS database_create`).Scan(&session).Error; err != nil {
		return errors.New("local trial runtime identity check failed")
	}
	if session.Role != expectedRole || session.Database != expectedDatabase || session.SchemaCreate || session.DatabaseCreate {
		return errors.New("local trial runtime identity or DDL boundary mismatch")
	}
	for _, name := range trialWritableTables {
		rights, err := tableRights(ctx, db, name)
		if err != nil {
			return err
		}
		if !rights.CanSelect || !rights.CanInsert || !rights.CanUpdate || rights.CanDelete {
			return fmt.Errorf("local trial table %s permissions mismatch", name)
		}
	}
	store, err := tableRights(ctx, db, "workbench_stores")
	if err != nil {
		return err
	}
	if !store.CanSelect || store.CanInsert || store.CanUpdate || store.CanDelete {
		return errors.New("local trial Store read-only boundary mismatch")
	}
	return nil
}
