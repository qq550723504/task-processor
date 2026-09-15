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
