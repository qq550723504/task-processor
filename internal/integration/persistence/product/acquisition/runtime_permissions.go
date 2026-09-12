package acquisition

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"task-processor/internal/product/sourcing"
)

const RuntimeRole = "source_acquisition_runtime"

// Read-only readiness for the existing SRC-1/Catalog schema consumed by this
// module. This is not a schema installer or another owner of publication facts.
// Required columns, publication identities and bounds must exist before ingress.
const publicationSchemaQuery = `WITH shared(name,kind,required) AS (VALUES
 ('organization_id','character varying(128)',true),('publication_id','character varying(128)',true),
 ('input_hash','character varying(64)',true),('producer_kind','character varying(128)',true),
 ('producer_version','character varying(128)',true),('product_key','character varying(128)',true),
 ('expected_base_version','bigint',false),('catalog_version','bigint',true),
 ('catalog_publication_id','character varying(128)',true),('envelope_hash','character varying(64)',true),
 ('snapshot_hash','character varying(64)',true),('actor_id','character varying(128)',true),
 ('published_at','timestamp with time zone',true)),
 expected(table_name,name,kind,required) AS (
 SELECT t.table_name,s.* FROM (VALUES('product_source_publications'),('product_source_publication_receipts')) t(table_name) CROSS JOIN shared s
 UNION ALL VALUES
 ('product_source_publications','envelope_json','bytea',true),('product_source_publications','snapshot_json','bytea',true),
 ('product_snapshot_versions','tenant_id','character varying(128)',true),('product_snapshot_versions','product_key','character varying(128)',true),
 ('product_snapshot_versions','version','bigint',true),('product_snapshot_versions','publication_id','character varying(128)',true),
 ('product_snapshot_versions','payload_hash','character varying(64)',true),('product_snapshot_versions','snapshot_json','json',true),
 ('product_snapshot_heads','tenant_id','character varying(128)',true),('product_snapshot_heads','product_key','character varying(128)',true),
 ('product_snapshot_heads','current_version','bigint',true),('product_snapshot_heads','publication_id','character varying(128)',true)),
 actual AS (SELECT c.relname::text AS table_name,a.attname::text AS name,format_type(a.atttypid,a.atttypmod) AS kind,a.attnotnull AS required
 FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relname IN (SELECT table_name FROM expected) AND a.attnum>0 AND NOT a.attisdropped),
 primary_keys(table_name,keys) AS (VALUES
 ('product_source_publications',ARRAY['organization_id','publication_id']),
 ('product_source_publication_receipts',ARRAY['organization_id','publication_id']),
 ('product_snapshot_versions',ARRAY['tenant_id','product_key','version']),
 ('product_snapshot_heads',ARRAY['tenant_id','product_key'])),
 checks(table_name,name,definition) AS (VALUES
 ('product_source_publications','ck_source_publication_envelope_size','CHECK (((octet_length(envelope_json) >= 1) AND (octet_length(envelope_json) <= 2097152)))'),
 ('product_source_publications','ck_source_publication_snapshot_size','CHECK (((octet_length(snapshot_json) >= 1) AND (octet_length(snapshot_json) <= 2097152)))'),
 ('product_source_publications','ck_source_publication_catalog_version','CHECK ((catalog_version > 0))'),
 ('product_source_publication_receipts','ck_source_receipt_catalog_version','CHECK ((catalog_version > 0))'))
 SELECT NOT EXISTS(SELECT 1 FROM expected e FULL JOIN actual a USING(table_name,name)
 WHERE e.kind IS DISTINCT FROM a.kind OR e.required IS DISTINCT FROM a.required)
 AND (SELECT count(*)=4 FROM primary_keys e JOIN pg_constraint c ON c.conrelid=to_regclass('public.'||e.table_name)
 WHERE c.contype='p' AND c.convalidated AND NOT c.condeferrable
 AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY k(number,position)
 JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.number ORDER BY k.position)=e.keys)
 AND EXISTS(SELECT 1 FROM pg_index i WHERE i.indrelid='public.product_snapshot_versions'::regclass
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indimmediate AND i.indpred IS NULL AND i.indexprs IS NULL
 AND ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(number,position)
 JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.number ORDER BY k.position)=ARRAY['tenant_id','product_key','publication_id'])
 AND (SELECT count(*)=4 FROM checks e JOIN pg_constraint c ON c.conrelid=to_regclass('public.'||e.table_name) AND c.conname=e.name
 WHERE c.contype='c' AND c.convalidated AND pg_get_constraintdef(c.oid)=e.definition)`

// These are table-level grants: column-only grants cannot satisfy a required
// operation, but forbidden column grants must still cause startup to fail.
const admittedPrivileges = `(VALUES
 ('product_acquisition_operations','SELECT'),('product_acquisition_operations','INSERT'),('product_acquisition_operations','UPDATE'),
 ('product_source_publications','SELECT'),('product_source_publications','INSERT'),
 ('product_source_publication_receipts','SELECT'),('product_source_publication_receipts','INSERT'),
 ('product_snapshot_versions','SELECT'),('product_snapshot_versions','INSERT'),
 ('product_snapshot_heads','SELECT'),('product_snapshot_heads','INSERT'),('product_snapshot_heads','UPDATE'))`

const runtimePermissionQuery = `WITH admitted(table_name,privilege) AS ` + admittedPrivileges + `
 SELECT current_user::text AS role_name,
 session_user=current_user
 AND current_schemas(false)=ARRAY['public']::name[]
 AND pg_my_temp_schema()=0
 AND has_database_privilege(current_user,current_database(),'CONNECT')
 AND has_schema_privilege(current_user,'public','USAGE')
 AND EXISTS(SELECT 1 FROM pg_roles WHERE rolname=current_user AND rolcanlogin
   AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls)
 AND NOT EXISTS(SELECT 1 FROM admitted WHERE NOT has_table_privilege(current_user,format('public.%I',table_name),privilege))
 AS required_privileges,
 has_database_privilege(current_user,current_database(),'CREATE')
 OR has_database_privilege(current_user,current_database(),'TEMPORARY')
 OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=(SELECT oid FROM pg_roles WHERE rolname=current_user))
 OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'
   AND (has_schema_privilege(current_user,n.oid,'CREATE') OR (n.nspname<>'public' AND has_schema_privilege(current_user,n.oid,'USAGE'))))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   CROSS JOIN LATERAL aclexplode(acldefault('r',c.relowner)) p
   WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'
   AND c.relkind IN ('r','p','v','m','f')
   AND CASE WHEN p.privilege_type IN ('SELECT','INSERT','UPDATE','REFERENCES')
     THEN has_any_column_privilege(current_user,c.oid,p.privilege_type)
     ELSE has_table_privilege(current_user,c.oid,p.privilege_type) END
   AND NOT EXISTS(SELECT 1 FROM admitted a WHERE n.nspname='public' AND a.table_name=c.relname AND a.privilege=p.privilege_type))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname='public' AND c.relname IN (SELECT table_name FROM admitted)
   AND (c.relkind<>'r' OR c.relrowsecurity OR c.relforcerowsecurity OR c.relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user)))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE c.relkind='S' AND n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'
   AND has_sequence_privilege(current_user,c.oid,'USAGE,SELECT,UPDATE')) AS forbidden_privileges`

// GrantRuntimePermissions is an explicit, dedicated-database initialization
// operation. The deployment owner provisions the login separately; this code
// never reads, generates or changes a runtime credential or another role.
func GrantRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return sourcing.ErrAcquisitionUnavailable
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var roleSafe bool
		if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname=? AND r.rolcanlogin
 AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication AND NOT r.rolbypassrls
 AND NOT EXISTS(SELECT 1 FROM pg_auth_members m WHERE m.member=r.oid))`, RuntimeRole).Scan(&roleSafe).Error; err != nil || !roleSafe {
			return sourcing.ErrAcquisitionUnavailable
		}
		var database string
		if err := tx.Raw("SELECT current_database()").Scan(&database).Error; err != nil || database == "" || database == "postgres" || database == "template0" || database == "template1" {
			return sourcing.ErrAcquisitionUnavailable
		}
		quoted := "\"" + strings.ReplaceAll(database, "\"", "\"\"") + "\""
		statements := []string{
			"REVOKE ALL PRIVILEGES ON DATABASE " + quoted + " FROM PUBLIC,source_acquisition_runtime",
			"GRANT CONNECT ON DATABASE " + quoted + " TO source_acquisition_runtime",
			"REVOKE ALL PRIVILEGES ON SCHEMA public FROM PUBLIC,source_acquisition_runtime",
			"GRANT USAGE ON SCHEMA public TO source_acquisition_runtime",
			"REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM source_acquisition_runtime",
			"GRANT SELECT,INSERT,UPDATE ON public.product_acquisition_operations,public.product_snapshot_heads TO source_acquisition_runtime",
			"GRANT SELECT,INSERT ON public.product_source_publications,public.product_source_publication_receipts,public.product_snapshot_versions TO source_acquisition_runtime",
		}
		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return sourcing.ErrAcquisitionUnavailable
			}
		}
		return nil
	})
}

// VerifyRuntimePermissions reuses the current application's catalog-based
// least-privilege check, narrowed to the five Product acquisition tables.
// It performs no grant, repair, DDL, business write or provider access.
func VerifyRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return sourcing.ErrAcquisitionUnavailable
	}
	pool, err := db.DB()
	if err != nil {
		return sourcing.ErrAcquisitionUnavailable
	}
	maximum := pool.Stats().MaxOpenConnections
	if maximum < 1 || maximum > 8 {
		return sourcing.ErrAcquisitionUnavailable
	}
	var user string
	var required, forbidden bool
	if err := pool.QueryRowContext(ctx, runtimePermissionQuery).Scan(&user, &required, &forbidden); err != nil {
		return sourcing.ErrAcquisitionUnavailable
	}
	if user != RuntimeRole || !required || forbidden {
		return sourcing.ErrAcquisitionUnavailable
	}
	var schemaReady bool
	if err := pool.QueryRowContext(ctx, publicationSchemaQuery).Scan(&schemaReady); err != nil || !schemaReady {
		return sourcing.ErrAcquisitionUnavailable
	}
	_, err = NewRepository(ctx, db)
	return err
}
