package assetpersistence

import (
	"context"
	"gorm.io/gorm"
	"strings"
	"task-processor/internal/product/asset"
)

const SourceRuntimeRole = "supply_asset_runtime"

// Explicit initialization of the existing Asset owner only. Ordinary serving
// verifies readiness and never calls this installer.
func InstallSourceApprovalSchema(db *gorm.DB) error { return AutoMigrate(db) }
func VerifySourceApprovalSchema(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return asset.ErrRepositoryUnavailable
	}
	for table, columns := range map[string][]string{
		"product_approved_assets":                  {"tenant_id", "action_id", "asset_id", "origin_kind", "origin_identity", "product_key", "target_platform", "source_snapshot_version", "payload_json"},
		"product_approval_receipts":                {"tenant_id", "action_id", "payload_hash", "asset_ids_json"},
		"product_approved_inventory_heads":         {"tenant_id", "product_key", "target_platform", "action_id"},
		"product_approved_inventory_version_heads": {"tenant_id", "product_key", "target_platform", "source_snapshot_version", "action_id"},
	} {
		for _, column := range columns {
			if !db.WithContext(ctx).Migrator().HasColumn(table, column) {
				return asset.ErrRepositoryUnavailable
			}
		}
	}
	var valid bool
	query := `WITH expected(name,keys) AS (VALUES
 ('product_approved_assets',ARRAY['tenant_id','action_id','asset_id']),('product_approval_receipts',ARRAY['tenant_id','action_id']),
 ('product_approved_inventory_heads',ARRAY['tenant_id','product_key','target_platform']),('product_approved_inventory_version_heads',ARRAY['tenant_id','product_key','target_platform','source_snapshot_version']))
 SELECT (SELECT count(*)=4 FROM expected e JOIN pg_constraint c ON c.conrelid=to_regclass('public.'||e.name)
 WHERE c.contype='p' AND c.convalidated AND NOT c.condeferrable AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY k(number,position) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.number ORDER BY k.position)=e.keys)
 AND (SELECT count(*)=2 FROM (VALUES('ux_product_approved_asset_id',ARRAY['tenant_id','asset_id']),('ux_product_approval_origin',ARRAY['tenant_id','action_id','origin_kind','origin_identity'])) e(name,keys)
 JOIN pg_class c ON c.relname=e.name JOIN pg_index i ON i.indexrelid=c.oid WHERE i.indrelid='public.product_approved_assets'::regclass AND i.indisunique AND i.indisvalid AND i.indisready AND i.indimmediate AND i.indpred IS NULL AND i.indexprs IS NULL AND i.indnkeyatts=array_length(e.keys,1)
 AND ARRAY(SELECT a.attname::text FROM unnest(i.indkey::smallint[]) WITH ORDINALITY k(number,position) JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.number WHERE k.position<=i.indnkeyatts ORDER BY k.position)=e.keys)`
	if db.WithContext(ctx).Raw(query).Scan(&valid).Error != nil || !valid {
		return asset.ErrRepositoryUnavailable
	}
	return nil
}
func GrantSourceRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	if VerifySourceApprovalSchema(ctx, db) != nil {
		return asset.ErrRepositoryUnavailable
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var safe bool
		if tx.Raw(`SELECT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname=? AND r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication AND NOT r.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_auth_members WHERE member=r.oid))`, SourceRuntimeRole).Scan(&safe).Error != nil || !safe {
			return asset.ErrRepositoryUnavailable
		}
		var database string
		if tx.Raw("SELECT current_database()").Scan(&database).Error != nil || database == "" || database == "postgres" || database == "template0" || database == "template1" {
			return asset.ErrRepositoryUnavailable
		}
		quoted := "\"" + strings.ReplaceAll(database, "\"", "\"\"") + "\""
		for _, statement := range []string{"REVOKE ALL PRIVILEGES ON DATABASE " + quoted + " FROM PUBLIC,supply_asset_runtime", "GRANT CONNECT ON DATABASE " + quoted + " TO supply_asset_runtime", "REVOKE ALL PRIVILEGES ON SCHEMA public FROM PUBLIC,supply_asset_runtime", "GRANT USAGE ON SCHEMA public TO supply_asset_runtime", "REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM supply_asset_runtime", "GRANT SELECT,INSERT ON public.product_approved_assets,public.product_approval_receipts TO supply_asset_runtime", "GRANT SELECT,INSERT,UPDATE ON public.product_approved_inventory_heads,public.product_approved_inventory_version_heads TO supply_asset_runtime"} {
			if tx.Exec(statement).Error != nil {
				return asset.ErrRepositoryUnavailable
			}
		}
		return nil
	})
}
func VerifySourceRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	if VerifySourceApprovalSchema(ctx, db) != nil {
		return asset.ErrRepositoryUnavailable
	}
	pool, err := db.DB()
	if err != nil || pool.Stats().MaxOpenConnections < 1 || pool.Stats().MaxOpenConnections > 8 {
		return asset.ErrRepositoryUnavailable
	}
	var role string
	var ready bool
	query := `WITH admitted(name,privilege) AS (VALUES('product_approved_assets','SELECT'),('product_approved_assets','INSERT'),('product_approval_receipts','SELECT'),('product_approval_receipts','INSERT'),('product_approved_inventory_heads','SELECT'),('product_approved_inventory_heads','INSERT'),('product_approved_inventory_heads','UPDATE'),('product_approved_inventory_version_heads','SELECT'),('product_approved_inventory_version_heads','INSERT'),('product_approved_inventory_version_heads','UPDATE'))
 SELECT current_user::text,session_user=current_user AND current_schemas(false)=ARRAY['public']::name[] AND pg_my_temp_schema()=0
 AND EXISTS(SELECT 1 FROM pg_roles WHERE rolname=current_user AND rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls)
 AND has_database_privilege(current_user,current_database(),'CONNECT') AND has_schema_privilege(current_user,'public','USAGE')
 AND NOT has_database_privilege(current_user,current_database(),'CREATE,TEMPORARY')
 AND NOT EXISTS(SELECT 1 FROM pg_auth_members WHERE member=(SELECT oid FROM pg_roles WHERE rolname=current_user))
 AND NOT EXISTS(SELECT 1 FROM admitted WHERE NOT has_table_privilege(current_user,format('public.%I',name),privilege))
 AND NOT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname<>'information_schema' AND (has_schema_privilege(current_user,oid,'CREATE') OR nspname<>'public' AND has_schema_privilege(current_user,oid,'USAGE')))
 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(acldefault('r',c.relowner)) p WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','f') AND CASE WHEN p.privilege_type IN ('SELECT','INSERT','UPDATE','REFERENCES') THEN has_any_column_privilege(current_user,c.oid,p.privilege_type) ELSE has_table_privilege(current_user,c.oid,p.privilege_type) END AND NOT EXISTS(SELECT 1 FROM admitted a WHERE n.nspname='public' AND a.name=c.relname AND a.privilege=p.privilege_type))
 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname IN (SELECT name FROM admitted) AND (c.relkind<>'r' OR c.relrowsecurity OR c.relforcerowsecurity OR c.relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user)))`
	if pool.QueryRowContext(ctx, query).Scan(&role, &ready) != nil || role != SourceRuntimeRole || !ready {
		return asset.ErrRepositoryUnavailable
	}
	return nil
}
