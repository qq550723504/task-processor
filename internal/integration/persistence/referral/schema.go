// Package referral implements the personal referral PostgreSQL boundary.
package referral

import (
	"context"
	domain "task-processor/internal/referral"

	"gorm.io/gorm"
)

var schema = []string{
	`CREATE TABLE public.referral_codes (issuer text NOT NULL, subject text NOT NULL, code text NOT NULL UNIQUE, PRIMARY KEY(issuer,subject))`,
	`CREATE TABLE public.registration_intents (
 id text PRIMARY KEY, issuer text NOT NULL, instance text NOT NULL, organization text NOT NULL,
 subject text NOT NULL, referrer text NOT NULL, key_hash text NOT NULL, email_hash text NOT NULL,
 fingerprint text NOT NULL, secret_hash text NOT NULL, key_id text NOT NULL, ciphertext bytea,
 created_at timestamptz NOT NULL, create_expires_at timestamptz NOT NULL, completion_expires_at timestamptz NOT NULL,
 lease_until timestamptz NOT NULL, state text NOT NULL CHECK(state IN ('PREPARED','CREATED','CONSUMED')),
 UNIQUE(issuer,subject), UNIQUE(issuer,key_hash), UNIQUE(issuer,email_hash),
 CHECK(create_expires_at=created_at+interval '15 minutes'), CHECK(completion_expires_at=created_at+interval '24 hours'),
 CHECK(subject<>referrer), FOREIGN KEY(issuer,referrer) REFERENCES public.referral_codes(issuer,subject))`,
	`CREATE TABLE public.referral_relations (issuer text NOT NULL, subject text NOT NULL, referrer text NOT NULL,
 intent_id text NOT NULL REFERENCES public.registration_intents(id), bound_at timestamptz NOT NULL,
 PRIMARY KEY(issuer,subject), CHECK(subject<>referrer), FOREIGN KEY(issuer,referrer) REFERENCES public.referral_codes(issuer,subject))`,
	`CREATE TABLE public.referral_receipts (intent_id text PRIMARY KEY REFERENCES public.registration_intents(id),
 issuer text NOT NULL, subject text NOT NULL, referrer text NOT NULL, fingerprint text NOT NULL, bound_at timestamptz NOT NULL,
 UNIQUE(issuer,subject), FOREIGN KEY(issuer,subject) REFERENCES public.referral_relations(issuer,subject))`,
	`CREATE TABLE public.registration_admission_buckets (kind text NOT NULL, key text NOT NULL,
 window_start timestamptz NOT NULL, hits integer NOT NULL CHECK(hits>0), PRIMARY KEY(kind,key,window_start))`,
	`CREATE INDEX registration_intents_payload_expiry_idx ON public.registration_intents(completion_expires_at) WHERE ciphertext IS NOT NULL`,
	`CREATE INDEX registration_admission_buckets_expiry_idx ON public.registration_admission_buckets(window_start)`,
	`CREATE TABLE public.referral_earning_claims (
 payment_id text PRIMARY KEY, issuer text NOT NULL, subject text NOT NULL, referrer text NOT NULL,
 currency char(3) NOT NULL, net_cash_minor bigint NOT NULL, commission_minor bigint NOT NULL,
 refunded_minor bigint NOT NULL DEFAULT 0, available_at timestamptz NOT NULL,
 state text NOT NULL CHECK(state IN ('PENDING','AVAILABLE','REVERSED')), created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 CHECK(net_cash_minor>0 AND commission_minor>0 AND refunded_minor>=0 AND refunded_minor<=net_cash_minor), CHECK(currency='CNY'))`,
	`CREATE TABLE public.referral_earnings_ledger (
 entry_id text PRIMARY KEY, referrer text NOT NULL, currency char(3) NOT NULL, payment_id text NOT NULL,
 entry_type text NOT NULL CHECK(entry_type IN ('COMMISSION','REFUND_ADJUSTMENT','REVERSAL')),
 amount_minor bigint NOT NULL, reference_id text NOT NULL, occurred_at timestamptz NOT NULL,
 UNIQUE(payment_id,entry_type,reference_id), CHECK(currency='CNY'), CHECK(amount_minor<>0))`,
	`CREATE TABLE public.referral_refund_operations (
 payment_id text NOT NULL REFERENCES public.referral_earning_claims(payment_id), refund_id text NOT NULL,
 amount_minor bigint NOT NULL CHECK(amount_minor>0), refunded_at timestamptz NOT NULL, created_at timestamptz NOT NULL,
 PRIMARY KEY(payment_id,refund_id))`,
	`CREATE TABLE public.referral_earnings_projection (
 referrer text NOT NULL, currency char(3) NOT NULL, pending_minor bigint NOT NULL DEFAULT 0,
 available_minor bigint NOT NULL DEFAULT 0, reserved_minor bigint NOT NULL DEFAULT 0,
 adjustment_minor bigint NOT NULL DEFAULT 0, version bigint NOT NULL DEFAULT 0, updated_at timestamptz NOT NULL,
 PRIMARY KEY(referrer,currency), CHECK(currency='CNY'))`,
	`CREATE TABLE public.referral_withdrawals (
 id text PRIMARY KEY, referrer text NOT NULL, payout_method_id text NOT NULL, currency char(3) NOT NULL, method text NOT NULL CHECK(method IN ('ALIPAY','BANK_TRANSFER')),
 amount_minor bigint NOT NULL, status text NOT NULL CHECK(status IN ('REQUESTED','APPROVED','PAID','CANCELED','REJECTED')),
 payout_reference text NOT NULL DEFAULT '', version bigint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 CHECK(currency='CNY' AND amount_minor>=10000))`,
	`CREATE TABLE public.referral_withdrawal_operations (
 idempotency_key text PRIMARY KEY, withdrawal_id text NOT NULL REFERENCES public.referral_withdrawals(id),
 fingerprint char(64) NOT NULL, result_version bigint NOT NULL, result_status text NOT NULL, created_at timestamptz NOT NULL)`,
	`CREATE TABLE public.referral_earnings_audit_events (
 id bigserial PRIMARY KEY, referrer text NOT NULL, actor text NOT NULL, object_type text NOT NULL,
 object_reference text NOT NULL, operation text NOT NULL, amount_minor bigint NOT NULL DEFAULT 0,
 idempotency_key text NOT NULL UNIQUE, created_at timestamptz NOT NULL)`,
}

// Install is an explicit, atomic greenfield operation, never called by serving.
// Existing tables fail closed rather than being silently repaired or migrated.
func Install(ctx context.Context, db *gorm.DB) error {
	if db == nil || ctx == nil {
		return domain.ErrInvalid
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, statement := range schema {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.ErrUnavailable
	}
	return nil
}
