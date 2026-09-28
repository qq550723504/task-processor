package listingsubscription

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// VerifyStoreQuotaRuntime only inspects schema and effective permissions. It
// cannot install schema, modify entitlements, seed plans or create a second ledger.
func VerifyStoreQuotaRuntime(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return errors.New("store quota requires existing canonical PostgreSQL owner")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	for _, query := range []string{
		`SELECT allocation_id,organization_id,store_id,request_key,request_fingerprint,status,created_by,updated_by,allocated_at,released_at,created_at,updated_at FROM public.saas_store_quota_allocations WHERE FALSE`,
		`SELECT organization_id,committed,reserved,version,updated_at FROM public.saas_store_quota_buckets WHERE FALSE`,
		`SELECT tenant_id,module_code,status,starts_at,expires_at,limits FROM public.saas_tenant_entitlements WHERE FALSE`,
	} {
		rows, err := sqlDB.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}

	for _, target := range []struct {
		table, name, columns string
		primary              bool
	}{
		{"saas_store_quota_allocations", "saas_store_quota_allocations_pkey", "allocation_id", true},
		{"saas_store_quota_buckets", "saas_store_quota_buckets_pkey", "organization_id", true},
		{"saas_store_quota_allocations", "idx_saas_store_quota_org_request", "organization_id,request_key", false},
		{"saas_store_quota_allocations", "idx_saas_store_quota_org_store", "organization_id,store_id", false},
		{"saas_tenant_entitlements", "idx_saas_tenant_module", "tenant_id,module_code", false},
	} {
		var valid bool
		if err := sqlDB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid WHERE i.indrelid=('public.'||$1)::regclass AND c.relname=$2 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indpred IS NULL AND (NOT $4 OR i.indisprimary) AND (SELECT string_agg(a.attname,',' ORDER BY k.n) FROM unnest(i.indkey) WITH ORDINALITY k(attnum,n) JOIN pg_catalog.pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum)=$3)`, target.table, target.name, target.columns, target.primary).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return errors.New("store quota canonical identity constraints unavailable")
		}
	}
	for _, target := range []struct{ table, name, definition string }{
		{"saas_store_quota_allocations", "chk_saas_store_quota_allocations_status", `CHECK (((status)::text = ANY ((ARRAY['reserved'::character varying, 'allocated'::character varying, 'released'::character varying])::text[])))`},
		{"saas_store_quota_buckets", "chk_saas_store_quota_buckets_committed", `CHECK ((committed >= 0))`},
		{"saas_store_quota_buckets", "chk_saas_store_quota_buckets_reserved", `CHECK ((reserved >= 0))`},
		{"saas_store_quota_buckets", "chk_saas_store_quota_buckets_version", `CHECK ((version > 0))`},
	} {
		var definition string
		if err := sqlDB.QueryRowContext(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_catalog.pg_constraint WHERE conrelid=('public.'||$1)::regclass AND conname=$2 AND contype='c' AND convalidated`, target.table, target.name).Scan(&definition); err != nil {
			return errors.New("store quota canonical state constraints unavailable")
		}
		if definition != target.definition {
			return errors.New("store quota canonical state constraints mismatch")
		}
	}
	var role string
	var required, forbidden bool
	err = sqlDB.QueryRowContext(ctx, storeQuotaPermissionQuery).Scan(&role, &required, &forbidden)
	if err != nil {
		return err
	}
	if role != "store_quota_runtime" || !required || forbidden {
		return errors.New("store quota runtime permissions do not match the admitted boundary")
	}
	return nil
}

const storeQuotaPermissionQuery = `SELECT current_user,
 current_schema()='public' AND current_schemas(false)=ARRAY['public']::name[]
 AND pg_catalog.to_regclass('saas_store_quota_allocations')=pg_catalog.to_regclass('public.saas_store_quota_allocations') AND pg_catalog.to_regclass('saas_store_quota_buckets')=pg_catalog.to_regclass('public.saas_store_quota_buckets') AND pg_catalog.to_regclass('saas_tenant_entitlements')=pg_catalog.to_regclass('public.saas_tenant_entitlements')
 AND has_database_privilege(current_user,current_database(),'CONNECT') AND has_schema_privilege(current_user,'public','USAGE')
 AND has_table_privilege(current_user,'public.saas_store_quota_allocations','SELECT') AND has_table_privilege(current_user,'public.saas_store_quota_allocations','INSERT') AND has_table_privilege(current_user,'public.saas_store_quota_allocations','UPDATE')
 AND has_table_privilege(current_user,'public.saas_store_quota_buckets','SELECT') AND has_table_privilege(current_user,'public.saas_store_quota_buckets','INSERT') AND has_table_privilege(current_user,'public.saas_store_quota_buckets','UPDATE')
 AND has_table_privilege(current_user,'public.saas_tenant_entitlements','SELECT'),
 has_database_privilege(current_user,current_database(),'CREATE') OR has_database_privilege(current_user,current_database(),'TEMP') OR has_schema_privilege(current_user,'public','CREATE')
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_' AND has_schema_privilege(current_user,n.oid,'CREATE'))
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=current_user AND (rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls))
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_class s JOIN pg_catalog.pg_namespace n ON n.oid=s.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_' AND s.relkind='S' AND (has_sequence_privilege(current_user,s.oid,'SELECT') OR has_sequence_privilege(current_user,s.oid,'UPDATE') OR has_sequence_privilege(current_user,s.oid,'USAGE')))
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_class r JOIN pg_catalog.pg_namespace n ON n.oid=r.relnamespace
 CROSS JOIN LATERAL pg_catalog.aclexplode(pg_catalog.acldefault('r',r.relowner)) p
 WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_' AND r.relkind IN ('r','p','v','m','f')
 AND CASE WHEN p.privilege_type IN ('SELECT','INSERT','UPDATE','REFERENCES') THEN pg_catalog.has_any_column_privilege(current_user,r.oid,p.privilege_type) ELSE pg_catalog.has_table_privilege(current_user,r.oid,p.privilege_type) END
 AND NOT (n.nspname='public' AND (r.relname,p.privilege_type) IN (('saas_store_quota_allocations','SELECT'),('saas_store_quota_allocations','INSERT'),('saas_store_quota_allocations','UPDATE'),('saas_store_quota_buckets','SELECT'),('saas_store_quota_buckets','INSERT'),('saas_store_quota_buckets','UPDATE'),('saas_tenant_entitlements','SELECT'))))`

// GrantStoreQuotaRuntimeAccess runs only in the explicit schema-owner command.
func GrantStoreQuotaRuntimeAccess(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("commercial schema owner unavailable")
	}
	var name string
	if err := db.WithContext(ctx).Raw("SELECT current_database()").Scan(&name).Error; err != nil {
		return err
	}
	for _, query := range []string{
		"GRANT CONNECT ON DATABASE " + `"` + strings.ReplaceAll(name, `"`, `""`) + `"` + " TO store_quota_runtime",
		`GRANT USAGE ON SCHEMA public TO store_quota_runtime`,
		`GRANT SELECT,INSERT,UPDATE ON public.saas_store_quota_allocations,public.saas_store_quota_buckets TO store_quota_runtime`,
		`GRANT SELECT ON public.saas_tenant_entitlements TO store_quota_runtime`,
	} {
		if err := db.WithContext(ctx).Exec(query).Error; err != nil {
			return err
		}
	}
	return nil
}
