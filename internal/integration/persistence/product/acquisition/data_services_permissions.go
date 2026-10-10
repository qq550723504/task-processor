package acquisition

import (
	"context"
	"gorm.io/gorm"
	"strings"
	"task-processor/internal/dataservice"
	keystore "task-processor/internal/integration/persistence/dataservice"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	jobstore "task-processor/internal/integration/persistence/product/dataacquisition"
)

const DataServicesRuntimeRole = "data_services_runtime"

// The Data Services pool shares Product's physical transaction owner, with no
// access to 1688 execution, Supply, Store, or other installed module tables.
const dataServicesPrivileges = `(VALUES
 ('product_source_publications','SELECT'),('product_source_publications','INSERT'),
 ('product_source_publication_receipts','SELECT'),('product_source_publication_receipts','INSERT'),
 ('product_snapshot_versions','SELECT'),('product_snapshot_versions','INSERT'),
 ('product_snapshot_heads','SELECT'),('product_snapshot_heads','INSERT'),('product_snapshot_heads','UPDATE')` + collectionPrivileges + `,
 ('data_service_credentials','SELECT'),('data_service_credentials','INSERT'),('data_service_credentials','UPDATE'),
 ('data_service_commands','SELECT'),('data_service_commands','INSERT'),
 ('data_service_quota','SELECT'),('data_service_quota','INSERT'),('data_service_quota','UPDATE'),
 ('data_acquisition_jobs','SELECT'),('data_acquisition_jobs','INSERT'),('data_acquisition_jobs','UPDATE'),
 ('data_acquisition_items','SELECT'),('data_acquisition_items','INSERT'),('data_acquisition_items','UPDATE'),
 ('data_service_custom_requests','SELECT'),('data_service_custom_requests','INSERT'),('data_service_custom_requests','UPDATE'),
 ('data_service_custom_events','SELECT'),('data_service_custom_events','INSERT'),
 ('data_service_custom_commands','SELECT'),('data_service_custom_commands','INSERT'))`

func verifyDataServicesSchema(ctx context.Context, db *gorm.DB) error {
	for _, verify := range []func(context.Context, *gorm.DB) error{collectionstore.VerifySchema, keystore.VerifySchema, jobstore.VerifySchema, keystore.VerifyCustomSchema} {
		if err := verify(ctx, db); err != nil {
			return err
		}
	}
	var ready bool
	if err := db.WithContext(ctx).Raw(publicationSchemaQuery).Scan(&ready).Error; err != nil || !ready {
		return dataservice.ErrUnavailable
	}
	return nil
}

// Explicit fresh-install grants; never called by serving construction.
func GrantDataServicesRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return dataservice.ErrUnavailable
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyDataServicesSchema(ctx, tx); err != nil {
			return err
		}
		var safe bool
		if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname=? AND r.rolcanlogin
   AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication AND NOT r.rolbypassrls
   AND NOT EXISTS(SELECT 1 FROM pg_auth_members m WHERE m.member=r.oid))`, DataServicesRuntimeRole).Scan(&safe).Error; err != nil || !safe {
			return dataservice.ErrUnavailable
		}
		var database string
		if err := tx.Raw("SELECT current_database()").Scan(&database).Error; err != nil || database == "" || database == "postgres" || database == "template0" || database == "template1" {
			return dataservice.ErrUnavailable
		}
		quoted := `"` + strings.ReplaceAll(database, `"`, `""`) + `"`
		for _, statement := range []string{
			"REVOKE ALL PRIVILEGES ON DATABASE " + quoted + " FROM PUBLIC,data_services_runtime",
			"GRANT CONNECT ON DATABASE " + quoted + " TO data_services_runtime",
			"REVOKE ALL PRIVILEGES ON SCHEMA public FROM PUBLIC,data_services_runtime",
			"GRANT USAGE ON SCHEMA public TO data_services_runtime",
			"REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM data_services_runtime",
			"GRANT SELECT,INSERT ON public.product_source_publications,public.product_source_publication_receipts,public.product_snapshot_versions,public.product_collection_operations,public.data_service_commands,public.data_service_custom_commands,public.data_service_custom_events TO data_services_runtime",
			"GRANT SELECT,INSERT,UPDATE ON public.product_snapshot_heads,public.product_collection_batches,public.product_collection_items,public.data_service_credentials,public.data_service_quota,public.data_acquisition_jobs,public.data_acquisition_items,public.data_service_custom_requests TO data_services_runtime",
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return dataservice.ErrUnavailable
			}
		}
		return nil
	})
}

// Reuse the Product catalog check, including forbidden column grants, owner
// membership, DDL, aliases, RLS and sequences. This is strictly read-only.
func VerifyDataServicesRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return dataservice.ErrUnavailable
	}
	pool, err := db.DB()
	if err != nil {
		return dataservice.ErrUnavailable
	}
	maximum := pool.Stats().MaxOpenConnections
	if maximum < 1 || maximum > 4 {
		return dataservice.ErrUnavailable
	}
	var user string
	var required, forbidden bool
	query := strings.Replace(runtimePermissionQuery, admittedPrivileges, dataServicesPrivileges, 1)
	if err := pool.QueryRowContext(ctx, query).Scan(&user, &required, &forbidden); err != nil || user != DataServicesRuntimeRole || !required || forbidden {
		return dataservice.ErrUnavailable
	}
	return verifyDataServicesSchema(ctx, db)
}
