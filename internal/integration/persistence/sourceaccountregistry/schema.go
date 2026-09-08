package sourceaccountregistry

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"
)

const (
	ResourceTable  = "source_account_resources"
	OperationTable = "source_account_operations"
)

var installationStatements = []string{
	`CREATE TABLE public.source_account_resources (
    organization_id VARCHAR(128) NOT NULL,
    id UUID NOT NULL,
    platform VARCHAR(16) NOT NULL,
    display_name VARCHAR(120) NOT NULL,
    management_status VARCHAR(16) NOT NULL,
    connection_status VARCHAR(32) NOT NULL,
    version BIGINT NOT NULL,
    created_by VARCHAR(256) NOT NULL,
    updated_by VARCHAR(256) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT source_account_resources_pkey PRIMARY KEY (organization_id, id),
    CONSTRAINT source_account_resources_organization_id_check CHECK (octet_length(organization_id) BETWEEN 1 AND 128 AND organization_id = btrim(organization_id)),
    CONSTRAINT source_account_resources_id_v7_check CHECK (substring(id::text FROM 15 FOR 1) = '7'),
    CONSTRAINT source_account_resources_platform_check CHECK (platform = '1688'),
    CONSTRAINT source_account_resources_display_name_check CHECK (octet_length(display_name) BETWEEN 1 AND 120 AND display_name = btrim(display_name) AND display_name !~ '[[:cntrl:]]'),
    CONSTRAINT source_account_resources_management_status_check CHECK (management_status IN ('enabled', 'disabled')),
    CONSTRAINT source_account_resources_connection_status_check CHECK (connection_status = 'pending_connection'),
    CONSTRAINT source_account_resources_version_check CHECK (version > 0),
    CONSTRAINT source_account_resources_created_by_check CHECK (octet_length(created_by) BETWEEN 1 AND 256 AND created_by = btrim(created_by)),
    CONSTRAINT source_account_resources_updated_by_check CHECK (octet_length(updated_by) BETWEEN 1 AND 256 AND updated_by = btrim(updated_by)),
    CONSTRAINT source_account_resources_timestamp_check CHECK (updated_at >= created_at)
)`,
	`CREATE INDEX idx_source_account_resources_page ON public.source_account_resources (organization_id, created_at, id)`,
	`CREATE TABLE public.source_account_operations (
    organization_id VARCHAR(128) NOT NULL,
    actor_subject VARCHAR(256) NOT NULL,
    idempotency_key UUID NOT NULL,
    kind VARCHAR(16) NOT NULL,
    request_fingerprint CHAR(64) NOT NULL,
    account_id UUID NOT NULL,
    resulting_version BIGINT NOT NULL,
    resulting_management_status VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT source_account_operations_pkey PRIMARY KEY (organization_id, actor_subject, idempotency_key),
    CONSTRAINT source_account_operations_organization_id_check CHECK (octet_length(organization_id) BETWEEN 1 AND 128 AND organization_id = btrim(organization_id)),
    CONSTRAINT source_account_operations_actor_subject_check CHECK (octet_length(actor_subject) BETWEEN 1 AND 256 AND actor_subject = btrim(actor_subject)),
    CONSTRAINT source_account_operations_kind_check CHECK (kind IN ('register', 'enable', 'disable')),
    CONSTRAINT source_account_operations_fingerprint_check CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    CONSTRAINT source_account_operations_version_check CHECK (resulting_version > 0),
    CONSTRAINT source_account_operations_status_check CHECK (resulting_management_status IN ('enabled', 'disabled')),
    CONSTRAINT source_account_operations_account_fkey FOREIGN KEY (organization_id, account_id) REFERENCES public.source_account_resources (organization_id, id) ON DELETE RESTRICT
)`,
}

var expectedColumns = map[string][]string{
	ResourceTable: {
		"organization_id|character varying|128|NO", "id|uuid||NO", "platform|character varying|16|NO",
		"display_name|character varying|120|NO", "management_status|character varying|16|NO", "connection_status|character varying|32|NO",
		"version|bigint||NO", "created_by|character varying|256|NO", "updated_by|character varying|256|NO",
		"created_at|timestamp with time zone||NO", "updated_at|timestamp with time zone||NO",
	},
	OperationTable: {
		"organization_id|character varying|128|NO", "actor_subject|character varying|256|NO", "idempotency_key|uuid||NO",
		"kind|character varying|16|NO", "request_fingerprint|character|64|NO", "account_id|uuid||NO",
		"resulting_version|bigint||NO", "resulting_management_status|character varying|16|NO", "created_at|timestamp with time zone||NO",
	},
}

var expectedConstraints = map[string][]string{
	ResourceTable: {
		"source_account_resources_connection_status_check|c|CHECK (connection_status::text = 'pending_connection'::text)",
		"source_account_resources_created_by_check|c|CHECK (octet_length(created_by::text) >= 1 AND octet_length(created_by::text) <= 256 AND created_by::text = btrim(created_by::text))",
		"source_account_resources_display_name_check|c|CHECK (octet_length(display_name::text) >= 1 AND octet_length(display_name::text) <= 120 AND display_name::text = btrim(display_name::text) AND display_name::text !~ '[[:cntrl:]]'::text)",
		"source_account_resources_id_v7_check|c|CHECK (SUBSTRING(id::text FROM 15 FOR 1) = '7'::text)",
		"source_account_resources_management_status_check|c|CHECK (management_status::text = ANY (ARRAY['enabled'::character varying, 'disabled'::character varying]::text[]))",
		"source_account_resources_organization_id_check|c|CHECK (octet_length(organization_id::text) >= 1 AND octet_length(organization_id::text) <= 128 AND organization_id::text = btrim(organization_id::text))",
		"source_account_resources_pkey|p|PRIMARY KEY (organization_id, id)",
		"source_account_resources_platform_check|c|CHECK (platform::text = '1688'::text)",
		"source_account_resources_timestamp_check|c|CHECK (updated_at >= created_at)",
		"source_account_resources_updated_by_check|c|CHECK (octet_length(updated_by::text) >= 1 AND octet_length(updated_by::text) <= 256 AND updated_by::text = btrim(updated_by::text))",
		"source_account_resources_version_check|c|CHECK (version > 0)",
	},
	OperationTable: {
		"source_account_operations_account_fkey|f|FOREIGN KEY (organization_id, account_id) REFERENCES source_account_resources(organization_id, id) ON DELETE RESTRICT",
		"source_account_operations_actor_subject_check|c|CHECK (octet_length(actor_subject::text) >= 1 AND octet_length(actor_subject::text) <= 256 AND actor_subject::text = btrim(actor_subject::text))",
		"source_account_operations_fingerprint_check|c|CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'::text)",
		"source_account_operations_kind_check|c|CHECK (kind::text = ANY (ARRAY['register'::character varying, 'enable'::character varying, 'disable'::character varying]::text[]))",
		"source_account_operations_organization_id_check|c|CHECK (octet_length(organization_id::text) >= 1 AND octet_length(organization_id::text) <= 128 AND organization_id::text = btrim(organization_id::text))",
		"source_account_operations_pkey|p|PRIMARY KEY (organization_id, actor_subject, idempotency_key)",
		"source_account_operations_status_check|c|CHECK (resulting_management_status::text = ANY (ARRAY['enabled'::character varying, 'disabled'::character varying]::text[]))",
		"source_account_operations_version_check|c|CHECK (resulting_version > 0)",
	},
}

type schemaQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func InstallSchemaTx(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return fmt.Errorf("source account schema transaction is nil")
	}
	for _, statement := range installationStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("install source account registry schema: %w", err)
		}
	}
	return verifySchema(ctx, tx)
}

func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("source account registry database is nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get source account registry database: %w", err)
	}
	return verifySchema(ctx, sqlDB)
}

func verifySchema(ctx context.Context, query schemaQuery) error {
	for _, table := range []string{ResourceTable, OperationTable} {
		columns, err := readColumns(ctx, query, table)
		if err != nil {
			return err
		}
		if !slices.Equal(columns, expectedColumns[table]) {
			return fmt.Errorf("source account registry schema mismatch for public.%s columns: got %v", table, columns)
		}
		constraints, err := readConstraints(ctx, query, table)
		if err != nil {
			return err
		}
		expected := make([]string, 0, len(expectedConstraints[table]))
		for _, constraint := range expectedConstraints[table] {
			expected = append(expected, constraint+"|true")
		}
		if !slices.Equal(constraints, expected) {
			return fmt.Errorf("source account registry schema mismatch for public.%s constraints: got %v", table, constraints)
		}
	}
	return verifyIndexes(ctx, query)
}

func readColumns(ctx context.Context, query schemaQuery, table string) ([]string, error) {
	rows, err := query.QueryContext(ctx, `SELECT column_name, data_type, COALESCE(character_maximum_length::text, ''), is_nullable FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, fmt.Errorf("inspect source account registry columns: %w", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var name, dataType, length, nullable string
		if err := rows.Scan(&name, &dataType, &length, &nullable); err != nil {
			return nil, fmt.Errorf("scan source account registry columns: %w", err)
		}
		result = append(result, strings.Join([]string{name, dataType, length, nullable}, "|"))
	}
	return result, rows.Err()
}

func readConstraints(ctx context.Context, query schemaQuery, table string) ([]string, error) {
	rows, err := query.QueryContext(ctx, `SELECT constraint_row.conname, constraint_row.contype::text, pg_get_constraintdef(constraint_row.oid, true), constraint_row.convalidated FROM pg_constraint AS constraint_row JOIN pg_class AS table_row ON table_row.oid = constraint_row.conrelid JOIN pg_namespace AS namespace_row ON namespace_row.oid = table_row.relnamespace WHERE namespace_row.nspname = 'public' AND table_row.relname = $1 ORDER BY constraint_row.conname`, table)
	if err != nil {
		return nil, fmt.Errorf("inspect source account registry constraints: %w", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var name, kind, definition string
		var validated bool
		if err := rows.Scan(&name, &kind, &definition, &validated); err != nil {
			return nil, fmt.Errorf("scan source account registry constraints: %w", err)
		}
		result = append(result, strings.Join([]string{name, kind, definition, fmt.Sprint(validated)}, "|"))
	}
	return result, rows.Err()
}

func verifyIndexes(ctx context.Context, query schemaQuery) error {
	rows, err := query.QueryContext(ctx, `SELECT table_row.relname, index_row.relname, index.indisunique, index.indisvalid, index.indpred IS NULL, string_agg(attribute.attname, ',' ORDER BY keys.ordinality) FROM pg_index AS index JOIN pg_class AS index_row ON index_row.oid = index.indexrelid JOIN pg_namespace AS index_namespace ON index_namespace.oid = index_row.relnamespace JOIN pg_class AS table_row ON table_row.oid = index.indrelid JOIN pg_namespace AS table_namespace ON table_namespace.oid = table_row.relnamespace CROSS JOIN LATERAL unnest(index.indkey) WITH ORDINALITY AS keys(attnum, ordinality) JOIN pg_attribute AS attribute ON attribute.attrelid = index.indrelid AND attribute.attnum = keys.attnum WHERE index_namespace.nspname = 'public' AND table_namespace.nspname = 'public' AND table_row.relname IN ('source_account_resources', 'source_account_operations') GROUP BY table_row.relname, index_row.relname, index.indisunique, index.indisvalid, index.indpred ORDER BY table_row.relname, index_row.relname`)
	if err != nil {
		return fmt.Errorf("inspect source account registry indexes: %w", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var table, indexName, columns string
		var unique, valid, noPredicate bool
		if err := rows.Scan(&table, &indexName, &unique, &valid, &noPredicate, &columns); err != nil {
			return fmt.Errorf("scan source account registry indexes: %w", err)
		}
		result = append(result, fmt.Sprintf("%s|%s|%t|%t|%t|%s", table, indexName, unique, valid, noPredicate, columns))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	expected := []string{
		"source_account_operations|source_account_operations_pkey|true|true|true|organization_id,actor_subject,idempotency_key",
		"source_account_resources|idx_source_account_resources_page|false|true|true|organization_id,created_at,id",
		"source_account_resources|source_account_resources_pkey|true|true|true|organization_id,id",
	}
	if !slices.Equal(result, expected) {
		return fmt.Errorf("source account registry schema mismatch for indexes: got %v", result)
	}
	return nil
}
