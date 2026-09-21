CREATE TABLE IF NOT EXISTS public.referral_earning_claims (
 payment_id text PRIMARY KEY, issuer text NOT NULL, subject text NOT NULL, referrer text NOT NULL,
 currency char(3) NOT NULL, net_cash_minor bigint NOT NULL, commission_minor bigint NOT NULL,
 refunded_minor bigint NOT NULL DEFAULT 0, available_at timestamptz NOT NULL,
 state text NOT NULL CHECK(state IN ('PENDING','AVAILABLE','REVERSED')),
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 CHECK(net_cash_minor>0 AND commission_minor>0 AND refunded_minor>=0 AND refunded_minor<=net_cash_minor), CHECK(currency='CNY'));
CREATE TABLE IF NOT EXISTS public.referral_earnings_ledger (
 entry_id text PRIMARY KEY, referrer text NOT NULL, currency char(3) NOT NULL, payment_id text NOT NULL,
 entry_type text NOT NULL CHECK(entry_type IN ('COMMISSION','REFUND_ADJUSTMENT','CHARGEBACK_ADJUSTMENT','REVERSAL')),
 amount_minor bigint NOT NULL, reference_id text NOT NULL, occurred_at timestamptz NOT NULL,
 UNIQUE(payment_id,entry_type,reference_id), CHECK(currency='CNY'), CHECK(amount_minor<>0));
CREATE TABLE IF NOT EXISTS public.referral_refund_operations (
 payment_id text NOT NULL REFERENCES public.referral_earning_claims(payment_id), refund_id text NOT NULL,
 amount_minor bigint NOT NULL CHECK(amount_minor>0), refunded_at timestamptz NOT NULL, created_at timestamptz NOT NULL,
 PRIMARY KEY(payment_id,refund_id));
CREATE TABLE IF NOT EXISTS public.referral_chargeback_operations (
 payment_id text NOT NULL REFERENCES public.referral_earning_claims(payment_id), chargeback_id text NOT NULL,
 amount_minor bigint NOT NULL CHECK(amount_minor>0), occurred_at timestamptz NOT NULL, created_at timestamptz NOT NULL,
 PRIMARY KEY(payment_id,chargeback_id));
CREATE TABLE IF NOT EXISTS public.referral_earnings_projection (
 referrer text NOT NULL, currency char(3) NOT NULL, pending_minor bigint NOT NULL DEFAULT 0,
 available_minor bigint NOT NULL DEFAULT 0, reserved_minor bigint NOT NULL DEFAULT 0,
 adjustment_minor bigint NOT NULL DEFAULT 0, version bigint NOT NULL DEFAULT 0, updated_at timestamptz NOT NULL,
 PRIMARY KEY(referrer,currency), CHECK(currency='CNY'));
CREATE TABLE IF NOT EXISTS public.referral_withdrawals (
 id text PRIMARY KEY, referrer text NOT NULL, payout_method_id text NOT NULL, currency char(3) NOT NULL, method text NOT NULL CHECK(method IN ('ALIPAY','BANK_TRANSFER')),
 amount_minor bigint NOT NULL, status text NOT NULL CHECK(status IN ('REQUESTED','APPROVED','PAID','CANCELED','REJECTED')),
 payout_reference text NOT NULL DEFAULT '', version bigint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 CHECK(currency='CNY' AND amount_minor>=10000));
CREATE TABLE IF NOT EXISTS public.referral_withdrawal_operations (
 idempotency_key text PRIMARY KEY, withdrawal_id text NOT NULL REFERENCES public.referral_withdrawals(id),
 fingerprint char(64) NOT NULL, result_version bigint NOT NULL, result_status text NOT NULL, created_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS public.referral_earnings_audit_events (
 id bigserial PRIMARY KEY, referrer text NOT NULL, actor text NOT NULL, object_type text NOT NULL,
 object_reference text NOT NULL, operation text NOT NULL, amount_minor bigint NOT NULL DEFAULT 0,
 idempotency_key text NOT NULL UNIQUE, created_at timestamptz NOT NULL);

-- Canonical money owner facts. Referral economics consumes these facts but
-- never creates or updates a payment settlement.
CREATE TABLE IF NOT EXISTS public.ledger_payment_settlements (
 payment_id text PRIMARY KEY, payer_user_id text NOT NULL, currency char(3) NOT NULL,
 gross_amount_minor bigint NOT NULL, discount_amount_minor bigint NOT NULL,
 commissionable_amount_minor bigint NOT NULL, status text NOT NULL CHECK(status='SETTLED'),
 settled_at timestamptz NOT NULL, provider_reference text NOT NULL, version bigint NOT NULL,
 CHECK(gross_amount_minor>0 AND discount_amount_minor>=0 AND commissionable_amount_minor>0 AND discount_amount_minor<=gross_amount_minor AND commissionable_amount_minor<=gross_amount_minor-discount_amount_minor));
CREATE TABLE IF NOT EXISTS public.ledger_refund_settlements (
 refund_id text PRIMARY KEY, payment_id text NOT NULL REFERENCES public.ledger_payment_settlements(payment_id),
 amount_minor bigint NOT NULL, occurred_at timestamptz NOT NULL, provider_reference text NOT NULL,
 CHECK(amount_minor>0));
CREATE TABLE IF NOT EXISTS public.ledger_chargeback_settlements (
 chargeback_id text PRIMARY KEY, payment_id text NOT NULL REFERENCES public.ledger_payment_settlements(payment_id),
 amount_minor bigint NOT NULL, occurred_at timestamptz NOT NULL, provider_reference text NOT NULL,
 CHECK(amount_minor>0));
CREATE TABLE IF NOT EXISTS public.ledger_payout_methods (
 method_id text PRIMARY KEY, subject_user_id text NOT NULL, type text NOT NULL CHECK(type IN ('ALIPAY','BANK_TRANSFER')),
 display_name text NOT NULL, masked_destination text NOT NULL, secure_reference bytea NOT NULL,
 status text NOT NULL CHECK(status IN ('ACTIVE','DISABLED')), created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL, version bigint NOT NULL);
CREATE TABLE IF NOT EXISTS public.ledger_payout_method_operations (
 idempotency_key text PRIMARY KEY, method_id text NOT NULL REFERENCES public.ledger_payout_methods(method_id),
 fingerprint char(64) NOT NULL, created_at timestamptz NOT NULL);
