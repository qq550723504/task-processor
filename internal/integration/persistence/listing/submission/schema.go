package submissionpersistence

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/gorm"
)

const (
	AttemptTable     = "listing_submission_execution_attempts"
	TargetFenceTable = "listing_submission_target_fences"
	// executionEvidenceTrimSpaceCharacters is the Unicode White_Space set used
	// by Go strings.TrimSpace. Required evidence reasons use this exact set in
	// PostgreSQL so write-time and read-time validation cannot disagree.
	executionEvidenceTrimSpaceCharacters = "\t\n\v\f\r \u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000"
)

var schemaStatements = []string{
	`CREATE TABLE public.listing_submission_execution_attempts (
    organization_id VARCHAR(128) NOT NULL,
    attempt_id UUID NOT NULL,
    intent_key VARCHAR(128) NOT NULL,
    platform VARCHAR(128) NOT NULL,
    store_id VARCHAR(128) NOT NULL,
    subject_id VARCHAR(128) NOT NULL,
    action VARCHAR(128) NOT NULL,
    payload_fingerprint CHAR(64) NOT NULL,
    provider_execution_key VARCHAR(73) NOT NULL,
    status VARCHAR(32) NOT NULL,
    fence_epoch BIGINT NOT NULL,
    claim_token_hash CHAR(64) NOT NULL,
    claim_owner_id VARCHAR(128) NOT NULL,
    lease_expires_at TIMESTAMPTZ NOT NULL,
    unknown_reason VARCHAR(32),
    evidence_kind VARCHAR(32),
    evidence_outcome VARCHAR(32),
    evidence_reference VARCHAR(128),
    evidence_fingerprint CHAR(64),
    evidence_reason VARCHAR(512),
    evidence_authorized_by VARCHAR(128),
    evidence_observed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    CONSTRAINT listing_submission_execution_attempts_pkey PRIMARY KEY (organization_id, attempt_id),
    CONSTRAINT listing_submission_execution_attempts_intent_unique UNIQUE (organization_id, intent_key),
    CONSTRAINT listing_submission_execution_attempts_provider_key_unique UNIQUE (organization_id, provider_execution_key),
    CONSTRAINT listing_submission_execution_attempts_id_v7_check CHECK (
        substring(attempt_id::text FROM 15 FOR 1) = '7' AND
        substring(attempt_id::text FROM 20 FOR 1) ~ '^[89ab]$'
    ),
    CONSTRAINT listing_submission_execution_attempts_identity_check CHECK (
        organization_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$' AND
        intent_key ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$' AND
        platform ~ '^[a-z0-9][a-z0-9._:-]{0,127}$' AND
        store_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$' AND
        subject_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$' AND
        action ~ '^[a-z0-9][a-z0-9._:-]{0,127}$' AND
        claim_owner_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'
    ),
    CONSTRAINT listing_submission_execution_attempts_digest_check CHECK (
        payload_fingerprint ~ '^[0-9a-f]{64}$' AND
        provider_execution_key ~ '^subk1_v1_[0-9a-f]{64}$' AND
        claim_token_hash ~ '^[0-9a-f]{64}$' AND
        (evidence_fingerprint IS NULL OR evidence_fingerprint ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT listing_submission_execution_attempts_status_check CHECK (status IN ('claimed', 'outcome_unknown', 'succeeded', 'failed_definitive', 'cancelled')),
    CONSTRAINT listing_submission_execution_attempts_fence_check CHECK (fence_epoch > 0),
    CONSTRAINT listing_submission_execution_attempts_timestamp_check CHECK (updated_at >= created_at AND lease_expires_at > created_at AND (finished_at IS NULL OR finished_at >= created_at)),
    CONSTRAINT listing_submission_execution_attempts_state_shape_check CHECK (
        ((status = 'claimed' AND unknown_reason IS NULL AND evidence_kind IS NULL AND evidence_outcome IS NULL AND evidence_reference IS NULL AND evidence_fingerprint IS NULL AND evidence_reason IS NULL AND evidence_authorized_by IS NULL AND evidence_observed_at IS NULL AND finished_at IS NULL) OR
        (status = 'outcome_unknown' AND unknown_reason IN ('response_lost', 'lease_expired', 'execution_cancelled') AND evidence_kind IS NULL AND evidence_outcome IS NULL AND evidence_reference IS NULL AND evidence_fingerprint IS NULL AND evidence_reason IS NULL AND evidence_authorized_by IS NULL AND evidence_observed_at IS NULL AND finished_at IS NULL) OR
        (status IN ('succeeded', 'failed_definitive', 'cancelled') AND unknown_reason IS NULL AND evidence_kind IS NOT NULL AND evidence_outcome = status AND evidence_reference IS NOT NULL AND evidence_fingerprint IS NOT NULL AND evidence_observed_at IS NOT NULL AND finished_at IS NOT NULL)
        ) IS TRUE
    ),
    CONSTRAINT listing_submission_execution_attempts_evidence_check CHECK (
        evidence_kind IS NULL OR
        (evidence_kind = 'provider_response' AND evidence_outcome IN ('succeeded', 'failed_definitive') AND evidence_authorized_by IS NULL) OR
        (evidence_kind = 'provider_readback' AND evidence_outcome IN ('succeeded', 'failed_definitive') AND evidence_authorized_by IS NULL) OR
        (evidence_kind = 'manual_resolution' AND evidence_outcome IN ('succeeded', 'failed_definitive', 'cancelled') AND evidence_authorized_by IS NOT NULL AND evidence_authorized_by ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$' AND evidence_reason IS NOT NULL)
    ),
    CONSTRAINT listing_submission_execution_attempts_evidence_value_check CHECK (
        (evidence_kind IS NULL OR (
            evidence_reference ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$' AND
            evidence_observed_at >= created_at
        )) IS TRUE
    ),
    CONSTRAINT listing_submission_execution_attempts_definitive_reason_check CHECK (
        (evidence_kind = 'manual_resolution' OR evidence_outcome IN ('failed_definitive', 'cancelled')) IS NOT TRUE OR
        (evidence_reason IS NOT NULL AND btrim(evidence_reason, '` + executionEvidenceTrimSpaceCharacters + `') <> '')
    )
)`,
	`CREATE TABLE public.listing_submission_target_fences (
    organization_id VARCHAR(128) NOT NULL,
    platform VARCHAR(128) NOT NULL,
    store_id VARCHAR(128) NOT NULL,
    subject_id VARCHAR(128) NOT NULL,
    epoch BIGINT NOT NULL,
    current_attempt_id UUID NOT NULL,
    current_status VARCHAR(32) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT listing_submission_target_fences_pkey PRIMARY KEY (organization_id, platform, store_id, subject_id),
    CONSTRAINT listing_submission_target_fences_attempt_fkey FOREIGN KEY (organization_id, current_attempt_id) REFERENCES public.listing_submission_execution_attempts (organization_id, attempt_id) ON DELETE RESTRICT,
    CONSTRAINT listing_submission_target_fences_identity_check CHECK (
        organization_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$' AND
        platform ~ '^[a-z0-9][a-z0-9._:-]{0,127}$' AND
        store_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$' AND
        subject_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'
    ),
    CONSTRAINT listing_submission_target_fences_epoch_check CHECK (epoch > 0),
    CONSTRAINT listing_submission_target_fences_status_check CHECK (current_status IN ('claimed', 'outcome_unknown', 'succeeded', 'failed_definitive', 'cancelled'))
)`,
}

type columnContract struct {
	name      string
	typeSQL   string
	notNull   bool
	identity  string
	generated string
}

var expectedColumns = map[string][]columnContract{
	AttemptTable: {
		{name: "organization_id", typeSQL: "character varying(128)", notNull: true},
		{name: "attempt_id", typeSQL: "uuid", notNull: true},
		{name: "intent_key", typeSQL: "character varying(128)", notNull: true},
		{name: "platform", typeSQL: "character varying(128)", notNull: true},
		{name: "store_id", typeSQL: "character varying(128)", notNull: true},
		{name: "subject_id", typeSQL: "character varying(128)", notNull: true},
		{name: "action", typeSQL: "character varying(128)", notNull: true},
		{name: "payload_fingerprint", typeSQL: "character(64)", notNull: true},
		{name: "provider_execution_key", typeSQL: "character varying(73)", notNull: true},
		{name: "status", typeSQL: "character varying(32)", notNull: true},
		{name: "fence_epoch", typeSQL: "bigint", notNull: true},
		{name: "claim_token_hash", typeSQL: "character(64)", notNull: true},
		{name: "claim_owner_id", typeSQL: "character varying(128)", notNull: true},
		{name: "lease_expires_at", typeSQL: "timestamp with time zone", notNull: true},
		{name: "unknown_reason", typeSQL: "character varying(32)"},
		{name: "evidence_kind", typeSQL: "character varying(32)"},
		{name: "evidence_outcome", typeSQL: "character varying(32)"},
		{name: "evidence_reference", typeSQL: "character varying(128)"},
		{name: "evidence_fingerprint", typeSQL: "character(64)"},
		{name: "evidence_reason", typeSQL: "character varying(512)"},
		{name: "evidence_authorized_by", typeSQL: "character varying(128)"},
		{name: "evidence_observed_at", typeSQL: "timestamp with time zone"},
		{name: "created_at", typeSQL: "timestamp with time zone", notNull: true},
		{name: "updated_at", typeSQL: "timestamp with time zone", notNull: true},
		{name: "finished_at", typeSQL: "timestamp with time zone"},
	},
	TargetFenceTable: {
		{name: "organization_id", typeSQL: "character varying(128)", notNull: true},
		{name: "platform", typeSQL: "character varying(128)", notNull: true},
		{name: "store_id", typeSQL: "character varying(128)", notNull: true},
		{name: "subject_id", typeSQL: "character varying(128)", notNull: true},
		{name: "epoch", typeSQL: "bigint", notNull: true},
		{name: "current_attempt_id", typeSQL: "uuid", notNull: true},
		{name: "current_status", typeSQL: "character varying(32)", notNull: true},
		{name: "updated_at", typeSQL: "timestamp with time zone", notNull: true},
	},
}

type constraintContract struct {
	kind            string
	exactDefinition string
}

var expectedConstraints = map[string]map[string]constraintContract{
	AttemptTable: {
		"listing_submission_execution_attempts_pkey": {
			kind: "p", exactDefinition: "PRIMARY KEY (organization_id, attempt_id)",
		},
		"listing_submission_execution_attempts_intent_unique": {
			kind: "u", exactDefinition: "UNIQUE (organization_id, intent_key)",
		},
		"listing_submission_execution_attempts_provider_key_unique": {
			kind: "u", exactDefinition: "UNIQUE (organization_id, provider_execution_key)",
		},
		"listing_submission_execution_attempts_id_v7_check": {
			kind: "c", exactDefinition: `CHECK (((SUBSTRING((attempt_id)::text FROM 15 FOR 1) = '7'::text) AND (SUBSTRING((attempt_id)::text FROM 20 FOR 1) ~ '^[89ab]$'::text)))`,
		},
		"listing_submission_execution_attempts_identity_check": {
			kind: "c", exactDefinition: `CHECK ((((organization_id)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text) AND ((intent_key)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text) AND ((platform)::text ~ '^[a-z0-9][a-z0-9._:-]{0,127}$'::text) AND ((store_id)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text) AND ((subject_id)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text) AND ((action)::text ~ '^[a-z0-9][a-z0-9._:-]{0,127}$'::text) AND ((claim_owner_id)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text)))`,
		},
		"listing_submission_execution_attempts_digest_check": {
			kind: "c", exactDefinition: `CHECK (((payload_fingerprint ~ '^[0-9a-f]{64}$'::text) AND ((provider_execution_key)::text ~ '^subk1_v1_[0-9a-f]{64}$'::text) AND (claim_token_hash ~ '^[0-9a-f]{64}$'::text) AND ((evidence_fingerprint IS NULL) OR (evidence_fingerprint ~ '^[0-9a-f]{64}$'::text))))`,
		},
		"listing_submission_execution_attempts_status_check": {
			kind: "c", exactDefinition: `CHECK (((status)::text = ANY ((ARRAY['claimed'::character varying, 'outcome_unknown'::character varying, 'succeeded'::character varying, 'failed_definitive'::character varying, 'cancelled'::character varying])::text[])))`,
		},
		"listing_submission_execution_attempts_fence_check": {
			kind: "c", exactDefinition: `CHECK ((fence_epoch > 0))`,
		},
		"listing_submission_execution_attempts_timestamp_check": {
			kind: "c", exactDefinition: `CHECK (((updated_at >= created_at) AND (lease_expires_at > created_at) AND ((finished_at IS NULL) OR (finished_at >= created_at))))`,
		},
		"listing_submission_execution_attempts_state_shape_check": {
			kind: "c", exactDefinition: `CHECK ((((((status)::text = 'claimed'::text) AND (unknown_reason IS NULL) AND (evidence_kind IS NULL) AND (evidence_outcome IS NULL) AND (evidence_reference IS NULL) AND (evidence_fingerprint IS NULL) AND (evidence_reason IS NULL) AND (evidence_authorized_by IS NULL) AND (evidence_observed_at IS NULL) AND (finished_at IS NULL)) OR (((status)::text = 'outcome_unknown'::text) AND ((unknown_reason)::text = ANY ((ARRAY['response_lost'::character varying, 'lease_expired'::character varying, 'execution_cancelled'::character varying])::text[])) AND (evidence_kind IS NULL) AND (evidence_outcome IS NULL) AND (evidence_reference IS NULL) AND (evidence_fingerprint IS NULL) AND (evidence_reason IS NULL) AND (evidence_authorized_by IS NULL) AND (evidence_observed_at IS NULL) AND (finished_at IS NULL)) OR (((status)::text = ANY ((ARRAY['succeeded'::character varying, 'failed_definitive'::character varying, 'cancelled'::character varying])::text[])) AND (unknown_reason IS NULL) AND (evidence_kind IS NOT NULL) AND ((evidence_outcome)::text = (status)::text) AND (evidence_reference IS NOT NULL) AND (evidence_fingerprint IS NOT NULL) AND (evidence_observed_at IS NOT NULL) AND (finished_at IS NOT NULL))) IS TRUE))`,
		},
		"listing_submission_execution_attempts_evidence_check": {
			kind: "c", exactDefinition: `CHECK (((evidence_kind IS NULL) OR (((evidence_kind)::text = 'provider_response'::text) AND ((evidence_outcome)::text = ANY ((ARRAY['succeeded'::character varying, 'failed_definitive'::character varying])::text[])) AND (evidence_authorized_by IS NULL)) OR (((evidence_kind)::text = 'provider_readback'::text) AND ((evidence_outcome)::text = ANY ((ARRAY['succeeded'::character varying, 'failed_definitive'::character varying])::text[])) AND (evidence_authorized_by IS NULL)) OR (((evidence_kind)::text = 'manual_resolution'::text) AND ((evidence_outcome)::text = ANY ((ARRAY['succeeded'::character varying, 'failed_definitive'::character varying, 'cancelled'::character varying])::text[])) AND (evidence_authorized_by IS NOT NULL) AND ((evidence_authorized_by)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text) AND (evidence_reason IS NOT NULL))))`,
		},
		"listing_submission_execution_attempts_evidence_value_check": {
			kind: "c", exactDefinition: `CHECK ((((evidence_kind IS NULL) OR (((evidence_reference)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text) AND (evidence_observed_at >= created_at))) IS TRUE))`,
		},
		"listing_submission_execution_attempts_definitive_reason_check": {
			kind: "c", exactDefinition: `CHECK ((((((evidence_kind)::text = 'manual_resolution'::text) OR ((evidence_outcome)::text = ANY ((ARRAY['failed_definitive'::character varying, 'cancelled'::character varying])::text[]))) IS NOT TRUE) OR ((evidence_reason IS NOT NULL) AND (btrim((evidence_reason)::text, '` + executionEvidenceTrimSpaceCharacters + `'::text) <> ''::text))))`,
		},
	},
	TargetFenceTable: {
		"listing_submission_target_fences_pkey": {
			kind: "p", exactDefinition: "PRIMARY KEY (organization_id, platform, store_id, subject_id)",
		},
		"listing_submission_target_fences_attempt_fkey": {
			kind: "f", exactDefinition: "FOREIGN KEY (organization_id, current_attempt_id) REFERENCES public.listing_submission_execution_attempts(organization_id, attempt_id) ON DELETE RESTRICT",
		},
		"listing_submission_target_fences_identity_check": {
			kind: "c", exactDefinition: `CHECK ((((organization_id)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text) AND ((platform)::text ~ '^[a-z0-9][a-z0-9._:-]{0,127}$'::text) AND ((store_id)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text) AND ((subject_id)::text ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'::text)))`,
		},
		"listing_submission_target_fences_epoch_check": {
			kind: "c", exactDefinition: `CHECK ((epoch > 0))`,
		},
		"listing_submission_target_fences_status_check": {
			kind: "c", exactDefinition: `CHECK (((current_status)::text = ANY ((ARRAY['claimed'::character varying, 'outcome_unknown'::character varying, 'succeeded'::character varying, 'failed_definitive'::character varying, 'cancelled'::character varying])::text[])))`,
		},
	},
}

func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return fmt.Errorf("install submission execution schema: PostgreSQL database is required")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, statement := range schemaStatements {
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("install submission execution schema: %w", err)
			}
		}
		return verifySchema(tx.Statement.Context, tx)
	})
}

func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return fmt.Errorf("verify submission execution schema: PostgreSQL database is required")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return verifySchema(ctx, tx)
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

func verifySchema(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).Exec(`SELECT set_config('search_path', 'pg_catalog', true)`).Error; err != nil {
		return fmt.Errorf("set submission execution schema verification search_path: %w", err)
	}
	for _, table := range []string{AttemptTable, TargetFenceTable} {
		if err := verifyRelation(ctx, db, table); err != nil {
			return err
		}
		if err := verifyColumns(ctx, db, table, expectedColumns[table]); err != nil {
			return err
		}
		if err := verifyConstraints(ctx, db, table, expectedConstraints[table]); err != nil {
			return err
		}
		if err := verifyNoUserTriggers(ctx, db, table); err != nil {
			return err
		}
		if err := verifyUniqueIndexes(ctx, db, table); err != nil {
			return err
		}
	}
	return nil
}

func verifyUniqueIndexes(ctx context.Context, db *gorm.DB, table string) error {
	rows, err := db.WithContext(ctx).Raw(`
SELECT index_relation.relname, COALESCE(constraint_row.conname, '')
FROM pg_catalog.pg_index AS index_row
JOIN pg_catalog.pg_class AS relation ON relation.oid = index_row.indrelid
JOIN pg_catalog.pg_namespace AS namespace ON namespace.oid = relation.relnamespace
JOIN pg_catalog.pg_class AS index_relation ON index_relation.oid = index_row.indexrelid
LEFT JOIN pg_catalog.pg_constraint AS constraint_row
  ON constraint_row.conrelid = relation.oid AND constraint_row.conindid = index_row.indexrelid
  AND constraint_row.contype IN ('p', 'u')
WHERE namespace.nspname = 'public' AND relation.relname = ?
  AND (index_row.indisunique OR index_row.indisexclusion)
ORDER BY index_relation.relname`, table).Rows()
	if err != nil {
		return fmt.Errorf("inspect submission execution unique indexes for public.%s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var indexName, constraintName string
		if err := rows.Scan(&indexName, &constraintName); err != nil {
			return fmt.Errorf("scan submission execution unique indexes for public.%s: %w", table, err)
		}
		contract, found := expectedConstraints[table][constraintName]
		if !found || contract.kind != "p" && contract.kind != "u" {
			return fmt.Errorf("submission execution relation public.%s has unexpected unique index %s", table, indexName)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate submission execution unique indexes for public.%s: %w", table, err)
	}
	return nil
}

func verifyNoUserTriggers(ctx context.Context, db *gorm.DB, table string) error {
	rows, err := db.WithContext(ctx).Raw(`
SELECT trigger_row.tgname
FROM pg_catalog.pg_trigger AS trigger_row
JOIN pg_catalog.pg_class AS relation ON relation.oid = trigger_row.tgrelid
JOIN pg_catalog.pg_namespace AS namespace ON namespace.oid = relation.relnamespace
WHERE namespace.nspname = 'public' AND relation.relname = ? AND NOT trigger_row.tgisinternal
ORDER BY trigger_row.tgname`, table).Rows()
	if err != nil {
		return fmt.Errorf("inspect submission execution triggers for public.%s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	var triggers []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan submission execution triggers for public.%s: %w", table, err)
		}
		triggers = append(triggers, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate submission execution triggers for public.%s: %w", table, err)
	}
	if len(triggers) != 0 {
		return fmt.Errorf("submission execution relation public.%s has unexpected user triggers: %v", table, triggers)
	}
	return nil
}

func verifyRelation(ctx context.Context, db *gorm.DB, table string) error {
	var kind, persistence string
	var partition bool
	err := db.WithContext(ctx).Raw(`
SELECT relation.relkind::text, relation.relpersistence::text, relation.relispartition
FROM pg_catalog.pg_class AS relation
JOIN pg_catalog.pg_namespace AS namespace ON namespace.oid = relation.relnamespace
WHERE namespace.nspname = 'public' AND relation.relname = ?`, table).
		Row().Scan(&kind, &persistence, &partition)
	if err != nil {
		return fmt.Errorf("inspect submission execution relation public.%s: %w", table, err)
	}
	if kind != "r" || persistence != "p" || partition {
		return fmt.Errorf("submission execution relation public.%s must be an ordinary permanent table: kind=%s persistence=%s partition=%t", table, kind, persistence, partition)
	}
	return nil
}

func verifyColumns(ctx context.Context, db *gorm.DB, table string, expected []columnContract) error {
	rows, err := db.WithContext(ctx).Raw(`
SELECT attribute.attname, pg_catalog.format_type(attribute.atttypid, attribute.atttypmod), attribute.attnotnull,
       attribute.attidentity::text, attribute.attgenerated::text
FROM pg_catalog.pg_attribute AS attribute
JOIN pg_catalog.pg_class AS relation ON relation.oid = attribute.attrelid
JOIN pg_catalog.pg_namespace AS namespace ON namespace.oid = relation.relnamespace
WHERE namespace.nspname = 'public' AND relation.relname = ?
  AND attribute.attnum > 0 AND NOT attribute.attisdropped
ORDER BY attribute.attnum`, table).Rows()
	if err != nil {
		return fmt.Errorf("inspect submission execution columns for public.%s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	var actual []columnContract
	for rows.Next() {
		var column columnContract
		if err := rows.Scan(&column.name, &column.typeSQL, &column.notNull, &column.identity, &column.generated); err != nil {
			return fmt.Errorf("scan submission execution columns for public.%s: %w", table, err)
		}
		actual = append(actual, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate submission execution columns for public.%s: %w", table, err)
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("submission execution schema mismatch for public.%s columns: got %d, want %d", table, len(actual), len(expected))
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("submission execution schema mismatch for public.%s column %d: got %+v, want %+v", table, index+1, actual[index], expected[index])
		}
	}
	return nil
}

func verifyConstraints(ctx context.Context, db *gorm.DB, table string, expected map[string]constraintContract) error {
	rows, err := db.WithContext(ctx).Raw(`
SELECT constraint_row.conname, constraint_row.contype::text, pg_catalog.pg_get_constraintdef(constraint_row.oid, false),
       constraint_row.convalidated, constraint_row.condeferrable, constraint_row.condeferred
FROM pg_catalog.pg_constraint AS constraint_row
JOIN pg_catalog.pg_class AS relation ON relation.oid = constraint_row.conrelid
JOIN pg_catalog.pg_namespace AS namespace ON namespace.oid = relation.relnamespace
WHERE namespace.nspname = 'public' AND relation.relname = ?
ORDER BY constraint_row.conname`, table).Rows()
	if err != nil {
		return fmt.Errorf("inspect submission execution constraints for public.%s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	type observedConstraint struct {
		kind, definition                string
		validated, deferrable, deferred bool
	}
	actual := make(map[string]observedConstraint)
	for rows.Next() {
		var name string
		var observed observedConstraint
		if err := rows.Scan(&name, &observed.kind, &observed.definition, &observed.validated, &observed.deferrable, &observed.deferred); err != nil {
			return fmt.Errorf("scan submission execution constraints for public.%s: %w", table, err)
		}
		actual[name] = observed
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate submission execution constraints for public.%s: %w", table, err)
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("submission execution schema mismatch for public.%s constraints: got %d, want %d", table, len(actual), len(expected))
	}
	for name, contract := range expected {
		observed, found := actual[name]
		if !found {
			return fmt.Errorf("submission execution schema missing constraint public.%s.%s", table, name)
		}
		if observed.kind != contract.kind {
			return fmt.Errorf("submission execution constraint public.%s.%s has kind %s, want %s", table, name, observed.kind, contract.kind)
		}
		if !observed.validated || observed.deferrable || observed.deferred {
			return fmt.Errorf("submission execution constraint public.%s.%s must be validated and immediate", table, name)
		}
		if observed.definition != contract.exactDefinition {
			return fmt.Errorf("submission execution constraint public.%s.%s definition mismatch: got %s", table, name, observed.definition)
		}
	}
	return nil
}
