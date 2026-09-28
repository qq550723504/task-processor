package storecenter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"
)

// Installation only creates the current schema on an empty Store database.
// No statement alters, backfills or interprets historical Store rows.
var currentSchemaStatements = []string{
	`CREATE TABLE public.workbench_stores (
 id CHAR(36) PRIMARY KEY NOT NULL, organization_id VARCHAR(200) NOT NULL,
 name TEXT NOT NULL, platform TEXT NOT NULL, region TEXT NOT NULL, external_store_id TEXT NOT NULL,
 record_status VARCHAR(32) NOT NULL, service_status VARCHAR(32), service_started_at TIMESTAMPTZ, service_expires_at TIMESTAMPTZ,
 connection_ref TEXT NOT NULL, quota_allocation_id CHAR(36) NOT NULL, version BIGINT NOT NULL,
 created_by VARCHAR(200) NOT NULL, updated_by VARCHAR(200) NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 deleted_at TIMESTAMPTZ, create_idempotency_key CHAR(36) NOT NULL, delete_operation_key VARCHAR(36) NOT NULL,
 identity_key VARCHAR(64) NOT NULL, create_request_fingerprint VARCHAR(64) NOT NULL,
 CONSTRAINT store_record_status CHECK (record_status IN ('provisioning','active','disabled','deleting','deleted')),
 CONSTRAINT store_service_shape CHECK (
   (record_status IN ('provisioning','deleting','deleted') AND service_status IS NULL AND service_started_at IS NULL AND service_expires_at IS NULL) OR
   (record_status IN ('active','disabled') AND service_status IS NOT NULL AND service_status IN ('pending_activation','active','expired','suspended'))),
 CONSTRAINT store_service_period CHECK (
   (service_status IS NULL AND service_started_at IS NULL AND service_expires_at IS NULL) OR
   (service_status='pending_activation' AND service_started_at IS NULL AND service_expires_at IS NULL) OR
   (service_status IN ('active','expired') AND service_started_at IS NOT NULL AND service_expires_at IS NOT NULL AND service_expires_at > service_started_at) OR
   (service_status='suspended' AND ((service_started_at IS NULL AND service_expires_at IS NULL) OR (service_started_at IS NOT NULL AND service_expires_at IS NOT NULL AND service_expires_at > service_started_at)))),
 CONSTRAINT store_delete_shape CHECK ((record_status IN ('deleting','deleted') AND delete_operation_key <> '') OR (record_status NOT IN ('deleting','deleted') AND delete_operation_key = '')),
 CONSTRAINT store_deleted_shape CHECK ((record_status='deleted') = (deleted_at IS NOT NULL)),
 CONSTRAINT store_version CHECK (version > 0), CONSTRAINT store_platform CHECK (platform='shein'),
 CONSTRAINT store_organization_identity UNIQUE (organization_id,id),
 CONSTRAINT store_times CHECK (updated_at >= created_at AND (deleted_at IS NULL OR deleted_at >= updated_at))
)`,
	`CREATE UNIQUE INDEX ux_workbench_stores_org_create_key ON public.workbench_stores (organization_id,create_idempotency_key)`,
	`CREATE UNIQUE INDEX ux_workbench_stores_org_identity_key ON public.workbench_stores (organization_id,identity_key)`,
	`CREATE INDEX idx_workbench_stores_org_record_status_updated ON public.workbench_stores (organization_id,record_status,updated_at)`,
	`CREATE INDEX idx_workbench_stores_org_platform_region ON public.workbench_stores (organization_id,platform,region)`,
	`CREATE INDEX idx_workbench_stores_deleted_at ON public.workbench_stores (deleted_at)`,
	`CREATE TABLE public.workbench_store_audit_logs (
 event_id CHAR(36) PRIMARY KEY NOT NULL, organization_id VARCHAR(200) NOT NULL, store_id CHAR(36) NOT NULL,
 allocation_id CHAR(36) NOT NULL, request_key CHAR(36) NOT NULL, action VARCHAR(64) NOT NULL, outcome VARCHAR(32) NOT NULL,
 actor_subject VARCHAR(200) NOT NULL, safe_field_names TEXT NOT NULL, payload_fingerprint VARCHAR(64) NOT NULL DEFAULT '',
 previous_state VARCHAR(32) NOT NULL, new_state VARCHAR(32) NOT NULL, failure_code VARCHAR(64) NOT NULL, store_version BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL, occurred_at TIMESTAMPTZ NOT NULL
)`,
	`CREATE UNIQUE INDEX ux_workbench_store_audit_org_request_action ON public.workbench_store_audit_logs (organization_id,request_key,action)`,
	`CREATE INDEX idx_workbench_store_audit_org_store_created ON public.workbench_store_audit_logs (organization_id,store_id,created_at)`,
	`CREATE TABLE public.workbench_store_member_grants (
 organization_id VARCHAR(200) NOT NULL, store_id CHAR(36) NOT NULL, member_id VARCHAR(200) NOT NULL,
 active BOOLEAN NOT NULL, version BIGINT NOT NULL, updated_by VARCHAR(200) NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY (organization_id,store_id,member_id),
 CONSTRAINT store_member_grant_store FOREIGN KEY (organization_id,store_id) REFERENCES public.workbench_stores (organization_id,id),
 CONSTRAINT store_member_grant_version CHECK (version > 0)
)`,
	`CREATE TABLE public.workbench_store_member_grant_operations (
 organization_id VARCHAR(200) NOT NULL, operation_id CHAR(36) NOT NULL, store_id CHAR(36) NOT NULL, member_id VARCHAR(200) NOT NULL,
 active BOOLEAN NOT NULL, expected_version BIGINT NOT NULL, result_version BIGINT NOT NULL, actor_id VARCHAR(200) NOT NULL,
 fingerprint VARCHAR(64) NOT NULL, created_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY (organization_id,operation_id),
 CONSTRAINT store_member_operation_grant FOREIGN KEY (organization_id,store_id,member_id) REFERENCES public.workbench_store_member_grants (organization_id,store_id,member_id),
 CONSTRAINT store_member_operation_version CHECK (expected_version >= 0 AND result_version > 0)
)`,
	`CREATE TABLE public.workbench_store_service_operations (
 organization_id VARCHAR(200) NOT NULL, operation_id CHAR(36) NOT NULL, store_id CHAR(36) NOT NULL,
 intent_json TEXT NOT NULL, reservation_id VARCHAR(36) NOT NULL DEFAULT '', state VARCHAR(32) NOT NULL,
 snapshot_json TEXT NOT NULL DEFAULT '', evidence_id VARCHAR(128) NOT NULL DEFAULT '', failure_code VARCHAR(64) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY (organization_id,operation_id),
 CONSTRAINT store_service_operation_store FOREIGN KEY (organization_id,store_id) REFERENCES public.workbench_stores (organization_id,id),
 CONSTRAINT store_service_operation_bound CHECK (octet_length(intent_json)<=16384 AND octet_length(snapshot_json)<=16384 AND updated_at>=created_at),
 CONSTRAINT store_service_operation_shape CHECK (
 (state='admitted' AND reservation_id='' AND snapshot_json='' AND evidence_id='' AND failure_code='') OR
 (state='bound' AND reservation_id<>'' AND snapshot_json='' AND evidence_id='' AND failure_code='') OR
 (state='succeeded' AND reservation_id<>'' AND snapshot_json<>'' AND evidence_id<>'' AND failure_code='') OR
 (state='failed_fenced' AND snapshot_json='' AND evidence_id<>'' AND failure_code IN ('NOT_ALLOWED','STORE_CHANGED','CONNECTION_CHANGED','INVALID_STATE','QUOTA_INSUFFICIENT')))
)`,
	`CREATE TABLE public.workbench_store_connections (
 organization_id VARCHAR(200) NOT NULL, store_id CHAR(36) NOT NULL, attempt_id CHAR(36) NOT NULL,
 version BIGINT NOT NULL, status VARCHAR(32) NOT NULL, observed_at TIMESTAMPTZ,
 PRIMARY KEY (organization_id,store_id),
 CONSTRAINT store_connection_store FOREIGN KEY (organization_id,store_id) REFERENCES public.workbench_stores (organization_id,id),
 CONSTRAINT store_connection_shape CHECK (version>0 AND status IN ('disconnected','connected','expired','unavailable'))
)`,
	`CREATE TABLE public.workbench_store_connection_attempts (
 organization_id VARCHAR(200) NOT NULL, attempt_id CHAR(36) NOT NULL, store_id CHAR(36) NOT NULL,
 actor_id VARCHAR(200) NOT NULL, member_id VARCHAR(200) NOT NULL, app_id VARCHAR(200) NOT NULL, app_version VARCHAR(200) NOT NULL,
 state_hash VARCHAR(64) NOT NULL, store_version BIGINT NOT NULL, connection_version BIGINT NOT NULL,
 state VARCHAR(32) NOT NULL, key_id VARCHAR(128) NOT NULL, ciphertext TEXT NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY (organization_id,attempt_id),
 CONSTRAINT store_connection_attempt_store FOREIGN KEY (organization_id,store_id) REFERENCES public.workbench_stores (organization_id,id),
 CONSTRAINT store_connection_attempt_bounds CHECK (store_version>0 AND connection_version>0 AND octet_length(state_hash)=64 AND octet_length(ciphertext)<=16384 AND updated_at>=created_at AND expires_at>created_at AND expires_at<=created_at+INTERVAL '5 minutes'),
 CONSTRAINT store_connection_attempt_shape CHECK (
 (state IN ('awaiting_consent','exchange_dispatched','failed') AND key_id='' AND ciphertext='') OR
 (state IN ('credential_received','verified') AND key_id<>'' AND ciphertext<>''))
)`,
	`CREATE TABLE public.workbench_store_merchant_bindings (
 app_id VARCHAR(200) NOT NULL, open_key_id VARCHAR(200) NOT NULL, organization_id VARCHAR(200) NOT NULL, store_id CHAR(36) NOT NULL,
 PRIMARY KEY (app_id,open_key_id),
 CONSTRAINT store_merchant_binding_store FOREIGN KEY (organization_id,store_id) REFERENCES public.workbench_stores (organization_id,id)
)`,
}

func InstallCurrentSchemaTx(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return errors.New("store schema transaction unavailable")
	}
	for _, statement := range currentSchemaStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

var currentColumnNames = map[string][]string{
	"workbench_stores":                        strings.Fields("id organization_id name platform region external_store_id record_status service_status service_started_at service_expires_at connection_ref quota_allocation_id version created_by updated_by created_at updated_at deleted_at create_idempotency_key delete_operation_key identity_key create_request_fingerprint"),
	"workbench_store_audit_logs":              strings.Fields("event_id organization_id store_id allocation_id request_key action outcome actor_subject safe_field_names payload_fingerprint previous_state new_state failure_code store_version created_at occurred_at"),
	"workbench_store_member_grants":           strings.Fields("organization_id store_id member_id active version updated_by updated_at"),
	"workbench_store_member_grant_operations": strings.Fields("organization_id operation_id store_id member_id active expected_version result_version actor_id fingerprint created_at"),
	"workbench_store_service_operations":      strings.Fields("organization_id operation_id store_id intent_json reservation_id state snapshot_json evidence_id failure_code created_at updated_at"),
	"workbench_store_connections":             strings.Fields("organization_id store_id attempt_id version status observed_at"),
	"workbench_store_connection_attempts":     strings.Fields("organization_id attempt_id store_id actor_id member_id app_id app_version state_hash store_version connection_version state key_id ciphertext expires_at created_at updated_at"),
	"workbench_store_merchant_bindings":       strings.Fields("app_id open_key_id organization_id store_id"),
}

func VerifyCurrentSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return errors.New("store center requires installed PostgreSQL schema")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	for table, want := range currentColumnNames {
		rows, err := sqlDB.QueryContext(ctx, `SELECT column_name,is_nullable,data_type,coalesce(character_maximum_length,0) FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 ORDER BY ordinal_position`, table)
		if err != nil {
			return err
		}
		var got []string
		for rows.Next() {
			var name, nullable, dataType string
			var length int
			if err := rows.Scan(&name, &nullable, &dataType, &length); err != nil {
				rows.Close()
				return err
			}

			expected := currentColumnType(table, name)
			if dataType != expected.kind || length != expected.length {
				rows.Close()
				return fmt.Errorf("store schema type mismatch: %s.%s", table, name)
			}
			got = append(got, name)
			optional := (table == "workbench_stores" && slices.Contains([]string{"service_status", "service_started_at", "service_expires_at", "deleted_at"}, name)) || (table == "workbench_store_connections" && name == "observed_at")
			if (nullable == "YES") != optional {
				rows.Close()
				return fmt.Errorf("store schema nullability mismatch: %s.%s", table, name)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if !slices.Equal(got, want) {
			return fmt.Errorf("store schema columns mismatch: %s", table)
		}
	}
	for name, want := range currentConstraintDefinitions {
		var definition string
		if err := sqlDB.QueryRowContext(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_catalog.pg_constraint WHERE conrelid='public.workbench_stores'::regclass AND contype='c' AND conname=$1 AND convalidated`, name).Scan(&definition); err != nil {
			return errors.New("store schema state constraints unavailable")
		}
		if strings.Join(strings.Fields(definition), " ") != want {
			return fmt.Errorf("store schema state constraint mismatch: %s", name)
		}
	}
	var valid bool
	for _, target := range []struct {
		table, name, columns string
		primary              bool
	}{
		{"workbench_stores", "ux_workbench_stores_org_create_key", "organization_id,create_idempotency_key", false},
		{"workbench_stores", "ux_workbench_stores_org_identity_key", "organization_id,identity_key", false},
		{"workbench_store_audit_logs", "ux_workbench_store_audit_org_request_action", "organization_id,request_key,action", false},
		{"workbench_stores", "workbench_stores_pkey", "id", true},
		{"workbench_store_audit_logs", "workbench_store_audit_logs_pkey", "event_id", true},
		{"workbench_store_member_grants", "workbench_store_member_grants_pkey", "organization_id,store_id,member_id", true},
		{"workbench_store_member_grant_operations", "workbench_store_member_grant_operations_pkey", "organization_id,operation_id", true},
		{"workbench_store_service_operations", "workbench_store_service_operations_pkey", "organization_id,operation_id", true},
		{"workbench_store_connections", "workbench_store_connections_pkey", "organization_id,store_id", true},
		{"workbench_store_connection_attempts", "workbench_store_connection_attempts_pkey", "organization_id,attempt_id", true},
		{"workbench_store_merchant_bindings", "workbench_store_merchant_bindings_pkey", "app_id,open_key_id", true},
	} {
		if err := sqlDB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid WHERE i.indrelid=('public.'||$1)::regclass AND c.relname=$2 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indpred IS NULL AND (NOT $4 OR i.indisprimary) AND (SELECT string_agg(a.attname,',' ORDER BY k.n) FROM unnest(i.indkey) WITH ORDINALITY k(attnum,n) JOIN pg_catalog.pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum)=$3)`, target.table, target.name, target.columns, target.primary).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return errors.New("store schema uniqueness boundary unavailable")
		}
	}
	for _, boundary := range []struct{ table, name, kind string }{
		{"workbench_store_member_grants", "store_member_grant_store", "f"},
		{"workbench_store_member_grants", "store_member_grant_version", "c"},
		{"workbench_store_member_grant_operations", "store_member_operation_grant", "f"},
		{"workbench_store_member_grant_operations", "store_member_operation_version", "c"},
		{"workbench_store_service_operations", "store_service_operation_store", "f"},
		{"workbench_store_service_operations", "store_service_operation_bound", "c"},
		{"workbench_store_service_operations", "store_service_operation_shape", "c"},
		{"workbench_store_connections", "store_connection_store", "f"},
		{"workbench_store_connections", "store_connection_shape", "c"},
		{"workbench_store_connection_attempts", "store_connection_attempt_store", "f"},
		{"workbench_store_connection_attempts", "store_connection_attempt_bounds", "c"},
		{"workbench_store_connection_attempts", "store_connection_attempt_shape", "c"},
		{"workbench_store_merchant_bindings", "store_merchant_binding_store", "f"},
	} {
		if err := sqlDB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_constraint WHERE conrelid=('public.'||$1)::regclass AND conname=$2 AND contype::text=$3 AND convalidated)`, boundary.table, boundary.name, boundary.kind).Scan(&valid); err != nil || !valid {
			return errors.New("store member schema boundary unavailable")
		}
	}
	return nil
}

const storeRuntimePermissionQuery = `SELECT current_user,
 current_schema()='public' AND current_schemas(false)=ARRAY['public']::name[]
 AND pg_catalog.to_regclass('workbench_stores')=pg_catalog.to_regclass('public.workbench_stores') AND pg_catalog.to_regclass('workbench_store_audit_logs')=pg_catalog.to_regclass('public.workbench_store_audit_logs')
 AND has_database_privilege(current_user,current_database(),'CONNECT') AND has_schema_privilege(current_user,'public','USAGE')
 AND has_table_privilege(current_user,'public.workbench_stores','SELECT') AND has_table_privilege(current_user,'public.workbench_stores','INSERT') AND has_table_privilege(current_user,'public.workbench_stores','UPDATE')
 AND has_table_privilege(current_user,'public.workbench_store_audit_logs','SELECT') AND has_table_privilege(current_user,'public.workbench_store_audit_logs','INSERT')
 AND has_table_privilege(current_user,'public.workbench_store_member_grants','SELECT') AND has_table_privilege(current_user,'public.workbench_store_member_grants','INSERT') AND has_table_privilege(current_user,'public.workbench_store_member_grants','UPDATE')
 AND has_table_privilege(current_user,'public.workbench_store_member_grant_operations','SELECT') AND has_table_privilege(current_user,'public.workbench_store_member_grant_operations','INSERT')
 AND has_table_privilege(current_user,'public.workbench_store_service_operations','SELECT') AND has_table_privilege(current_user,'public.workbench_store_service_operations','INSERT') AND has_table_privilege(current_user,'public.workbench_store_service_operations','UPDATE')
 AND has_table_privilege(current_user,'public.workbench_store_connections','SELECT') AND has_table_privilege(current_user,'public.workbench_store_connections','INSERT') AND has_table_privilege(current_user,'public.workbench_store_connections','UPDATE')
 AND has_table_privilege(current_user,'public.workbench_store_connection_attempts','SELECT') AND has_table_privilege(current_user,'public.workbench_store_connection_attempts','INSERT') AND has_table_privilege(current_user,'public.workbench_store_connection_attempts','UPDATE')
 AND has_table_privilege(current_user,'public.workbench_store_merchant_bindings','SELECT') AND has_table_privilege(current_user,'public.workbench_store_merchant_bindings','INSERT'),
 has_database_privilege(current_user,current_database(),'CREATE') OR has_database_privilege(current_user,current_database(),'TEMP') OR has_schema_privilege(current_user,'public','CREATE')
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_' AND has_schema_privilege(current_user,n.oid,'CREATE'))
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=current_user AND (rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls))
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_class s JOIN pg_catalog.pg_namespace n ON n.oid=s.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_' AND s.relkind='S' AND (has_sequence_privilege(current_user,s.oid,'SELECT') OR has_sequence_privilege(current_user,s.oid,'UPDATE') OR has_sequence_privilege(current_user,s.oid,'USAGE')))
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_class r JOIN pg_catalog.pg_namespace n ON n.oid=r.relnamespace
 CROSS JOIN LATERAL pg_catalog.aclexplode(pg_catalog.acldefault('r',r.relowner)) p
 WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_' AND r.relkind IN ('r','p','v','m','f')
 AND CASE WHEN p.privilege_type IN ('SELECT','INSERT','UPDATE','REFERENCES') THEN pg_catalog.has_any_column_privilege(current_user,r.oid,p.privilege_type) ELSE pg_catalog.has_table_privilege(current_user,r.oid,p.privilege_type) END
 AND NOT (n.nspname='public' AND (r.relname,p.privilege_type) IN (('workbench_stores','SELECT'),('workbench_stores','INSERT'),('workbench_stores','UPDATE'),('workbench_store_audit_logs','SELECT'),('workbench_store_audit_logs','INSERT'),('workbench_store_member_grants','SELECT'),('workbench_store_member_grants','INSERT'),('workbench_store_member_grants','UPDATE'),('workbench_store_member_grant_operations','SELECT'),('workbench_store_member_grant_operations','INSERT'),('workbench_store_service_operations','SELECT'),('workbench_store_service_operations','INSERT'),('workbench_store_service_operations','UPDATE'),('workbench_store_connections','SELECT'),('workbench_store_connections','INSERT'),('workbench_store_connections','UPDATE'),('workbench_store_connection_attempts','SELECT'),('workbench_store_connection_attempts','INSERT'),('workbench_store_connection_attempts','UPDATE'),('workbench_store_merchant_bindings','SELECT'),('workbench_store_merchant_bindings','INSERT'))))`

func VerifyRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("store runtime pool unavailable")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	var role string
	var required, forbidden bool
	if err := sqlDB.QueryRowContext(ctx, storeRuntimePermissionQuery).Scan(&role, &required, &forbidden); err != nil {
		return err
	}
	if role != "store_center_runtime" || !required || forbidden {
		return errors.New("store runtime permissions do not match the admitted boundary")
	}
	return nil
}

type schemaColumnType struct {
	kind   string
	length int
}

func currentColumnType(table, name string) schemaColumnType {
	switch name {
	case "service_started_at", "service_expires_at", "created_at", "updated_at", "deleted_at", "occurred_at", "observed_at", "expires_at":
		return schemaColumnType{kind: "timestamp with time zone"}
	case "version", "store_version", "expected_version", "result_version", "connection_version":
		return schemaColumnType{kind: "bigint"}
	case "active":
		return schemaColumnType{kind: "boolean"}
	case "id", "event_id", "store_id", "allocation_id", "quota_allocation_id", "request_key", "create_idempotency_key", "operation_id", "attempt_id":
		return schemaColumnType{kind: "character", length: 36}
	case "organization_id", "created_by", "updated_by", "actor_subject", "member_id", "actor_id", "app_id", "app_version", "open_key_id":
		return schemaColumnType{kind: "character varying", length: 200}
	case "record_status", "service_status", "outcome", "previous_state", "new_state", "state", "status":
		return schemaColumnType{kind: "character varying", length: 32}
	case "identity_key", "create_request_fingerprint", "payload_fingerprint", "action", "failure_code", "fingerprint", "state_hash":
		return schemaColumnType{kind: "character varying", length: 64}
	case "delete_operation_key", "reservation_id":
		return schemaColumnType{kind: "character varying", length: 36}
	case "evidence_id", "key_id":
		return schemaColumnType{kind: "character varying", length: 128}
	default:
		return schemaColumnType{kind: "text"}
	}
}

// PostgreSQL 16 canonical definitions for the installed contract. Startup only
// reads catalogs; an identically named but weakened constraint is rejected.
var currentConstraintDefinitions = map[string]string{
	"store_delete_shape":   `CHECK (((((record_status)::text = ANY ((ARRAY['deleting'::character varying, 'deleted'::character varying])::text[])) AND ((delete_operation_key)::text <> ''::text)) OR (((record_status)::text <> ALL ((ARRAY['deleting'::character varying, 'deleted'::character varying])::text[])) AND ((delete_operation_key)::text = ''::text))))`,
	"store_deleted_shape":  `CHECK ((((record_status)::text = 'deleted'::text) = (deleted_at IS NOT NULL)))`,
	"store_platform":       `CHECK ((platform = 'shein'::text))`,
	"store_record_status":  `CHECK (((record_status)::text = ANY ((ARRAY['provisioning'::character varying, 'active'::character varying, 'disabled'::character varying, 'deleting'::character varying, 'deleted'::character varying])::text[])))`,
	"store_service_period": `CHECK ((((service_status IS NULL) AND (service_started_at IS NULL) AND (service_expires_at IS NULL)) OR (((service_status)::text = 'pending_activation'::text) AND (service_started_at IS NULL) AND (service_expires_at IS NULL)) OR (((service_status)::text = ANY ((ARRAY['active'::character varying, 'expired'::character varying])::text[])) AND (service_started_at IS NOT NULL) AND (service_expires_at IS NOT NULL) AND (service_expires_at > service_started_at)) OR (((service_status)::text = 'suspended'::text) AND (((service_started_at IS NULL) AND (service_expires_at IS NULL)) OR ((service_started_at IS NOT NULL) AND (service_expires_at IS NOT NULL) AND (service_expires_at > service_started_at))))))`,
	"store_service_shape":  `CHECK (((((record_status)::text = ANY ((ARRAY['provisioning'::character varying, 'deleting'::character varying, 'deleted'::character varying])::text[])) AND (service_status IS NULL) AND (service_started_at IS NULL) AND (service_expires_at IS NULL)) OR (((record_status)::text = ANY ((ARRAY['active'::character varying, 'disabled'::character varying])::text[])) AND (service_status IS NOT NULL) AND ((service_status)::text = ANY ((ARRAY['pending_activation'::character varying, 'active'::character varying, 'expired'::character varying, 'suspended'::character varying])::text[])))))`,
	"store_times":          `CHECK (((updated_at >= created_at) AND ((deleted_at IS NULL) OR (deleted_at >= updated_at))))`,
	"store_version":        `CHECK ((version > 0))`,
}
