package acquisition

import (
	"context"
	"strings"
	officialstore "task-processor/internal/integration/persistence/listing/official"

	"gorm.io/gorm"
	preparationstore "task-processor/internal/integration/persistence/listing/preparation"
	recordstore "task-processor/internal/integration/persistence/listing/record"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	podstore "task-processor/internal/integration/persistence/product/pod"
	marketstore "task-processor/internal/integration/persistence/product/supplymarket"
	"task-processor/internal/product/sourcing"
)

const RuntimeRole = "source_acquisition_runtime"

type RuntimeCapabilities struct {
	Collections  bool
	SupplyChain  bool
	SupplyMarket bool
	POD          bool
	ImageSets    bool
}

const collectionPrivileges = `,('product_collection_batches','SELECT'),('product_collection_batches','INSERT'),('product_collection_batches','UPDATE'),
 ('product_collection_items','SELECT'),('product_collection_items','INSERT'),('product_collection_items','UPDATE'),
 ('product_collection_operations','SELECT'),('product_collection_operations','INSERT')`

func runtimePermissionsFor(capability RuntimeCapabilities) string {
	admitted := strings.TrimSuffix(admittedPrivileges, ")")
	if capability.Collections {
		admitted += collectionPrivileges
	}
	if capability.SupplyChain {
		admitted += supplyPrivileges
	}
	if capability.SupplyMarket {
		admitted += marketPrivileges
		if !capability.SupplyChain {
			admitted += `,('product_title_proposals','SELECT')`
		}
	}
	if capability.POD {
		admitted += podPrivileges
		if !capability.SupplyChain {
			admitted += submissionPrivileges
		}
	}
	if capability.ImageSets && !capability.SupplyChain && !capability.SupplyMarket {
		admitted += ",('product_title_proposals','SELECT')"
	}
	admitted += ")"
	return strings.Replace(runtimePermissionQuery, admittedPrivileges, admitted, 1)
}

const marketPrivileges = `,('supply_market_records','SELECT'),('supply_market_records','INSERT'),('supply_market_records','UPDATE'),
 ('supply_market_events','SELECT'),('supply_market_events','INSERT'),
 ('supply_market_releases','SELECT'),('supply_market_releases','INSERT'),('supply_market_releases','UPDATE'),
 ('supply_market_commands','SELECT'),('supply_market_commands','INSERT'),
 ('supply_market_private_uploads','SELECT'),('supply_market_private_uploads','INSERT')`
const podPrivileges = `,('product_pod_operations','SELECT'),('product_pod_operations','INSERT'),('product_pod_operations','UPDATE'),
 ('product_pod_fences','SELECT'),('product_pod_fences','INSERT'),('product_pod_fences','UPDATE'),('product_pod_fences','DELETE'),
 ('product_pod_commands','SELECT'),('product_pod_commands','INSERT')`
const submissionPrivileges = `,('listing_submission_execution_attempts','SELECT'),('listing_submission_execution_attempts','INSERT'),('listing_submission_execution_attempts','UPDATE'),
 ('listing_submission_target_fences','SELECT'),('listing_submission_target_fences','INSERT'),('listing_submission_target_fences','UPDATE')`

const supplyPrivileges = `,('listing_preparations','SELECT'),('listing_preparations','INSERT'),
 ('listing_preparation_sources','SELECT'),('listing_preparation_sources','INSERT'),
 ('listing_preparation_targets','SELECT'),('listing_preparation_targets','INSERT'),('listing_preparation_targets','UPDATE'),
 ('listing_target_records','SELECT'),('listing_target_records','INSERT'),
 ('listing_target_record_commands','SELECT'),('listing_target_record_commands','INSERT'),
 ('listing_preparation_operations','SELECT'),('listing_preparation_operations','INSERT'),('listing_preparation_operations','UPDATE'),
 ('listing_preparation_operation_items','SELECT'),('listing_preparation_operation_items','INSERT'),('listing_preparation_operation_items','UPDATE'),
 ('listing_submission_execution_attempts','SELECT'),('listing_submission_execution_attempts','INSERT'),('listing_submission_execution_attempts','UPDATE'),
 ('listing_submission_target_fences','SELECT'),('listing_submission_target_fences','INSERT'),('listing_submission_target_fences','UPDATE'),
 ('listing_submission_official_intents','SELECT'),('listing_submission_official_intents','INSERT'),
 ('listing_submission_official_receipts','SELECT'),('listing_submission_official_receipts','INSERT'),
 ('product_title_proposals','SELECT')`

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
 ('product_acquisition_charge_intents','SELECT'),('product_acquisition_charge_intents','INSERT'),('product_acquisition_charge_intents','UPDATE'),
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
func GrantRuntimePermissions(ctx context.Context, db *gorm.DB, capabilities ...RuntimeCapabilities) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return sourcing.ErrAcquisitionUnavailable
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(capabilities) > 1 {
			return sourcing.ErrAcquisitionUnavailable
		}
		enabled := len(capabilities) == 1 && capabilities[0].Collections
		supply := len(capabilities) == 1 && capabilities[0].SupplyChain
		images := len(capabilities) == 1 && capabilities[0].ImageSets
		market := len(capabilities) == 1 && capabilities[0].SupplyMarket
		pod := len(capabilities) == 1 && capabilities[0].POD
		if (supply || market || pod) && !enabled || pod && !market {
			return sourcing.ErrAcquisitionUnavailable
		}
		if enabled {
			if err := collectionstore.VerifySchema(ctx, tx); err != nil {
				return err
			}
		}
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
			"GRANT SELECT,INSERT,UPDATE ON public.product_acquisition_operations,public.product_acquisition_charge_intents,public.product_snapshot_heads TO source_acquisition_runtime",
			"GRANT SELECT,INSERT ON public.product_source_publications,public.product_source_publication_receipts,public.product_snapshot_versions TO source_acquisition_runtime",
		}
		if enabled {
			statements = append(statements,
				"GRANT SELECT,INSERT,UPDATE ON public.product_collection_batches,public.product_collection_items TO source_acquisition_runtime",
				"GRANT SELECT,INSERT ON public.product_collection_operations TO source_acquisition_runtime")
		}
		if supply {
			statements = append(statements, "GRANT SELECT,INSERT ON public.listing_preparations,public.listing_preparation_sources,public.listing_target_records,public.listing_target_record_commands,public.listing_submission_official_intents,public.listing_submission_official_receipts TO source_acquisition_runtime",
				"GRANT SELECT,INSERT,UPDATE ON public.listing_preparation_targets,public.listing_preparation_operations,public.listing_preparation_operation_items,public.listing_submission_execution_attempts,public.listing_submission_target_fences TO source_acquisition_runtime",
				"GRANT SELECT ON public.product_title_proposals TO source_acquisition_runtime")
		}
		if market {
			if err := marketstore.VerifySchema(ctx, tx); err != nil {
				return err
			}
			statements = append(statements, "GRANT SELECT,INSERT,UPDATE ON public.supply_market_records,public.supply_market_releases TO source_acquisition_runtime",
				"GRANT SELECT,INSERT ON public.supply_market_events,public.supply_market_commands,public.supply_market_private_uploads TO source_acquisition_runtime",
				"GRANT SELECT ON public.product_title_proposals TO source_acquisition_runtime")
		}
		if pod {
			if err := podstore.VerifySchema(ctx, tx); err != nil {
				return err
			}
			statements = append(statements, "GRANT SELECT,INSERT,UPDATE ON public.product_pod_operations,public.listing_submission_execution_attempts,public.listing_submission_target_fences TO source_acquisition_runtime",
				"GRANT SELECT,INSERT ON public.product_pod_commands TO source_acquisition_runtime",
				"GRANT SELECT,INSERT,UPDATE,DELETE ON public.product_pod_fences TO source_acquisition_runtime")
		}
		if images && !supply && !market {
			statements = append(statements, "GRANT SELECT ON public.product_title_proposals TO source_acquisition_runtime")
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
// least-privilege check, narrowed to the six Product acquisition tables.
// It performs no grant, repair, DDL, business write or provider access.
func VerifyRuntimePermissions(ctx context.Context, db *gorm.DB, capabilities ...RuntimeCapabilities) error {
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
	if len(capabilities) > 1 {
		return sourcing.ErrAcquisitionUnavailable
	}
	capability := RuntimeCapabilities{}
	if len(capabilities) == 1 {
		capability = capabilities[0]
	}
	if (capability.SupplyChain || capability.SupplyMarket || capability.POD) && !capability.Collections || capability.POD && !capability.SupplyMarket {
		return sourcing.ErrAcquisitionUnavailable
	}
	var required, forbidden bool
	if err := pool.QueryRowContext(ctx, runtimePermissionsFor(capability)).Scan(&user, &required, &forbidden); err != nil {
		return sourcing.ErrAcquisitionUnavailable
	}
	if user != RuntimeRole || !required || forbidden {
		return sourcing.ErrAcquisitionUnavailable
	}
	var schemaReady bool
	if err := pool.QueryRowContext(ctx, publicationSchemaQuery).Scan(&schemaReady); err != nil || !schemaReady {
		return sourcing.ErrAcquisitionUnavailable
	}
	if capability.Collections {
		if err := collectionstore.VerifySchema(ctx, db); err != nil {
			return err
		}
	}
	if capability.SupplyChain {
		if err := preparationstore.VerifySchema(ctx, db); err != nil {
			return err
		}
		if err := preparationstore.VerifyOperationSchema(ctx, db); err != nil {
			return err
		}
		if err := recordstore.VerifySchema(ctx, db); err != nil {
			return err
		}
		if err := officialstore.VerifyOfficialSchema(ctx, db); err != nil {
			return err
		}
	}
	if capability.SupplyMarket {
		if err := marketstore.VerifySchema(ctx, db); err != nil {
			return err
		}
	}
	if capability.POD {
		if err := podstore.VerifySchema(ctx, db); err != nil {
			return err
		}
	}
	_, err = NewRepository(ctx, db)
	return err
}
