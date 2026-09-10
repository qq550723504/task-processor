package submissionpersistence

import (
	"context"
	"fmt"
	"slices"

	"gorm.io/gorm"
)

const (
	AttemptTable     = "listing_submission_execution_attempts"
	TargetFenceTable = "listing_submission_target_fences"
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
    CONSTRAINT listing_submission_execution_attempts_id_v7_check CHECK (substring(attempt_id::text FROM 15 FOR 1) = '7'),
    CONSTRAINT listing_submission_execution_attempts_identity_check CHECK (
        octet_length(organization_id) BETWEEN 1 AND 128 AND organization_id = btrim(organization_id) AND
        octet_length(intent_key) BETWEEN 1 AND 128 AND intent_key = btrim(intent_key) AND
        octet_length(platform) BETWEEN 1 AND 128 AND platform = btrim(platform) AND
        octet_length(store_id) BETWEEN 1 AND 128 AND store_id = btrim(store_id) AND
        octet_length(subject_id) BETWEEN 1 AND 128 AND subject_id = btrim(subject_id) AND
        octet_length(action) BETWEEN 1 AND 128 AND action = btrim(action) AND
        octet_length(claim_owner_id) BETWEEN 1 AND 128 AND claim_owner_id = btrim(claim_owner_id)
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
        (status = 'claimed' AND unknown_reason IS NULL AND evidence_kind IS NULL AND finished_at IS NULL) OR
        (status = 'outcome_unknown' AND unknown_reason IN ('response_lost', 'lease_expired', 'execution_cancelled') AND evidence_kind IS NULL AND finished_at IS NULL) OR
        (status IN ('succeeded', 'failed_definitive', 'cancelled') AND unknown_reason IS NULL AND evidence_kind IS NOT NULL AND evidence_outcome = status AND evidence_reference IS NOT NULL AND evidence_fingerprint IS NOT NULL AND evidence_observed_at IS NOT NULL AND finished_at IS NOT NULL)
    ),
    CONSTRAINT listing_submission_execution_attempts_evidence_check CHECK (
        evidence_kind IS NULL OR
        (evidence_kind = 'provider_response' AND evidence_outcome IN ('succeeded', 'failed_definitive') AND evidence_authorized_by IS NULL) OR
        (evidence_kind = 'provider_readback' AND evidence_outcome IN ('succeeded', 'failed_definitive') AND evidence_authorized_by IS NULL) OR
        (evidence_kind = 'manual_resolution' AND evidence_outcome IN ('succeeded', 'failed_definitive', 'cancelled') AND evidence_authorized_by IS NOT NULL AND evidence_reason IS NOT NULL)
    ),
    CONSTRAINT listing_submission_execution_attempts_definitive_reason_check CHECK (evidence_outcome NOT IN ('failed_definitive', 'cancelled') OR evidence_reason IS NOT NULL)
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
        octet_length(organization_id) BETWEEN 1 AND 128 AND organization_id = btrim(organization_id) AND
        octet_length(platform) BETWEEN 1 AND 128 AND platform = btrim(platform) AND
        octet_length(store_id) BETWEEN 1 AND 128 AND store_id = btrim(store_id) AND
        octet_length(subject_id) BETWEEN 1 AND 128 AND subject_id = btrim(subject_id)
    ),
    CONSTRAINT listing_submission_target_fences_epoch_check CHECK (epoch > 0),
    CONSTRAINT listing_submission_target_fences_status_check CHECK (current_status IN ('claimed', 'outcome_unknown', 'succeeded', 'failed_definitive', 'cancelled'))
)`,
}

var expectedColumns = map[string][]string{
	AttemptTable: {
		"organization_id", "attempt_id", "intent_key", "platform", "store_id", "subject_id", "action",
		"payload_fingerprint", "provider_execution_key", "status", "fence_epoch", "claim_token_hash", "claim_owner_id", "lease_expires_at",
		"unknown_reason", "evidence_kind", "evidence_outcome", "evidence_reference", "evidence_fingerprint", "evidence_reason",
		"evidence_authorized_by", "evidence_observed_at", "created_at", "updated_at", "finished_at",
	},
	TargetFenceTable: {"organization_id", "platform", "store_id", "subject_id", "epoch", "current_attempt_id", "current_status", "updated_at"},
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
	return verifySchema(ctx, db.WithContext(ctx))
}

func verifySchema(ctx context.Context, db *gorm.DB) error {
	for _, table := range []string{AttemptTable, TargetFenceTable} {
		rows, err := db.WithContext(ctx).Raw(`SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = ? ORDER BY ordinal_position`, table).Rows()
		if err != nil {
			return fmt.Errorf("inspect submission execution schema: %w", err)
		}
		var columns []string
		for rows.Next() {
			var column string
			if err := rows.Scan(&column); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan submission execution schema: %w", err)
			}
			columns = append(columns, column)
		}
		closeErr := rows.Close()
		if closeErr != nil {
			return closeErr
		}
		if !slices.Equal(columns, expectedColumns[table]) {
			return fmt.Errorf("submission execution schema mismatch for public.%s columns: got %v", table, columns)
		}
	}
	return nil
}
