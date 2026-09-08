package ownershipmigration

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
)

var preparedSchemaColumns = map[string][]string{
	"organization_source_accounts": {
		"id|bigint||NO|",
		"organization_id|character varying|128|NO|",
		"platform|character varying|32|NO|",
		"label|character varying|128|YES|",
		"profile_ref|character varying|256|NO|",
		"profile_directory|character varying|1024|NO|",
		"proxy_ref|character varying|256|YES|",
		"login_url|text||YES|",
		"status|smallint||NO|",
		"deleted|smallint||NO|",
		"last_verified_at|timestamp with time zone||YES|",
		"created_at|timestamp with time zone||NO|",
		"updated_at|timestamp with time zone||NO|",
	},
	"source_account_ownership_migration_receipts": {
		"contract_version|smallint||NO|",
		"idempotency_key|character varying|128|NO|",
		"stage|character varying|32|NO|",
		"request_sha256|character|64|NO|",
		"preflight_sha256|character|64|NO|",
		"source_id|character varying|256|NO|",
		"source_database|character varying|128|NO|",
		"source_schema|character varying|63|NO|",
		"source_sha256|character|64|NO|",
		"target_sha256|character|64|NO|",
		"account_count|integer||NO|",
		"result_json|jsonb||NO|",
		"prepared_at|timestamp with time zone||NO|",
	},
}

var preparedSchemaConstraints = map[string][]string{
	"organization_source_accounts": {
		"organization_source_accounts_id_positive|c|CHECK (id > 0)",
		"organization_source_accounts_organization_nonempty|c|CHECK (organization_id::text <> ''::text AND organization_id::text = btrim(organization_id::text))",
		"organization_source_accounts_pkey|p|PRIMARY KEY (id)",
		"organization_source_accounts_platform_1688|c|CHECK (platform::text = '1688'::text)",
		"organization_source_accounts_profile_directory_nonempty|c|CHECK (btrim(profile_directory::text) <> ''::text)",
		"organization_source_accounts_profile_ref_nonempty|c|CHECK (btrim(profile_ref::text) <> ''::text)",
	},
	"source_account_ownership_migration_receipts": {
		"source_account_ownership_migration_receipts_pkey|p|PRIMARY KEY (contract_version, idempotency_key)",
		"source_account_ownership_receipts_account_count|c|CHECK (account_count >= 1 AND account_count <= 100000)",
		"source_account_ownership_receipts_database_nonempty|c|CHECK (source_database::text <> ''::text)",
		"source_account_ownership_receipts_key_bytes|c|CHECK (octet_length(idempotency_key::text) >= 1 AND octet_length(idempotency_key::text) <= 128)",
		"source_account_ownership_receipts_preflight_hash|c|CHECK (preflight_sha256 ~ '^[0-9a-f]{64}$'::text)",
		"source_account_ownership_receipts_public_schema|c|CHECK (source_schema::text = 'public'::text)",
		"source_account_ownership_receipts_request_hash|c|CHECK (request_sha256 ~ '^[0-9a-f]{64}$'::text)",
		"source_account_ownership_receipts_source_hash|c|CHECK (source_sha256 ~ '^[0-9a-f]{64}$'::text)",
		"source_account_ownership_receipts_source_id_nonempty|c|CHECK (source_id::text <> ''::text)",
		"source_account_ownership_receipts_stage_prepared|c|CHECK (stage::text = 'prepared_only'::text)",
		"source_account_ownership_receipts_target_hash|c|CHECK (target_sha256 ~ '^[0-9a-f]{64}$'::text)",
		"source_account_ownership_receipts_version_one|c|CHECK (contract_version = 1)",
	},
}

func validatePreparedSchema(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []string{"organization_source_accounts", "source_account_ownership_migration_receipts"} {
		got, err := readPreparedSchemaColumns(ctx, tx, table)
		if err != nil {
			return err
		}
		if !slices.Equal(got, preparedSchemaColumns[table]) {
			return fmt.Errorf("source account ownership migration schema mismatch for public.%s columns: got %v", table, got)
		}
		constraints, err := readPreparedSchemaConstraints(ctx, tx, table)
		if err != nil {
			return err
		}
		if !slices.Equal(constraints, preparedSchemaConstraints[table]) {
			return fmt.Errorf("source account ownership migration schema mismatch for public.%s constraints: got %v", table, constraints)
		}
	}
	var columns string
	var unique, valid, noPredicate bool
	err := tx.QueryRowContext(ctx, "SELECT string_agg(attribute.attname, ',' ORDER BY keys.ordinality), index.indisunique, index.indisvalid, index.indpred IS NULL FROM pg_index AS index JOIN pg_class AS index_class ON index_class.oid = index.indexrelid JOIN pg_namespace AS index_namespace ON index_namespace.oid = index_class.relnamespace JOIN pg_class AS table_class ON table_class.oid = index.indrelid JOIN pg_namespace AS table_namespace ON table_namespace.oid = table_class.relnamespace CROSS JOIN LATERAL unnest(index.indkey) WITH ORDINALITY AS keys(attnum, ordinality) JOIN pg_attribute AS attribute ON attribute.attrelid = index.indrelid AND attribute.attnum = keys.attnum WHERE index_namespace.nspname = 'public' AND index_class.relname = 'idx_organization_source_accounts_reader' AND table_namespace.nspname = 'public' AND table_class.relname = 'organization_source_accounts' GROUP BY index.indisunique, index.indisvalid, index.indpred").
		Scan(&columns, &unique, &valid, &noPredicate)
	if err != nil {
		return fmt.Errorf("inspect Organization source account reader index: %w", err)
	}
	if columns != "organization_id,id,status,deleted" || unique || !valid || !noPredicate {
		return fmt.Errorf("source account ownership migration reader index mismatch: columns=%q unique=%t valid=%t no_predicate=%t", columns, unique, valid, noPredicate)
	}
	return nil
}

func readPreparedSchemaColumns(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT column_name, data_type, COALESCE(character_maximum_length::text, ''), is_nullable, COALESCE(column_default, '') FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 ORDER BY ordinal_position", table)
	if err != nil {
		return nil, fmt.Errorf("inspect source account ownership migration table %s: %w", table, err)
	}
	defer rows.Close()
	columns := make([]string, 0, len(preparedSchemaColumns[table]))
	for rows.Next() {
		var name, dataType, maximumLength, nullable, defaultValue string
		if err = rows.Scan(&name, &dataType, &maximumLength, &nullable, &defaultValue); err != nil {
			return nil, fmt.Errorf("scan source account ownership migration table %s: %w", table, err)
		}
		columns = append(columns, strings.Join([]string{name, dataType, maximumLength, nullable, defaultValue}, "|"))
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source account ownership migration table %s: %w", table, err)
	}
	return columns, nil
}

func readPreparedSchemaConstraints(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT constraint_row.conname, constraint_row.contype::text, pg_get_constraintdef(constraint_row.oid, true) FROM pg_constraint AS constraint_row JOIN pg_class AS table_row ON table_row.oid = constraint_row.conrelid JOIN pg_namespace AS namespace_row ON namespace_row.oid = table_row.relnamespace WHERE namespace_row.nspname = 'public' AND table_row.relname = $1 AND constraint_row.contype <> 'n' ORDER BY constraint_row.conname", table)
	if err != nil {
		return nil, fmt.Errorf("inspect source account ownership migration constraints for %s: %w", table, err)
	}
	defer rows.Close()
	constraints := make([]string, 0, len(preparedSchemaConstraints[table]))
	for rows.Next() {
		var name, constraintType, definition string
		if err = rows.Scan(&name, &constraintType, &definition); err != nil {
			return nil, fmt.Errorf("scan source account ownership migration constraint for %s: %w", table, err)
		}
		constraints = append(constraints, name+"|"+constraintType+"|"+definition)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source account ownership migration constraints for %s: %w", table, err)
	}
	return constraints, nil
}

func lockPreparedSchemaReadTables(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		"LOCK TABLE public.organization_source_accounts IN ACCESS SHARE MODE",
		"LOCK TABLE public.source_account_ownership_migration_receipts IN ACCESS SHARE MODE",
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("lock source account ownership migration schema for read: %w", err)
		}
	}
	return nil
}
