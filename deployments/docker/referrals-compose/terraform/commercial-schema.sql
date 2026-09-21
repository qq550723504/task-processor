CREATE TABLE IF NOT EXISTS public.saas_plans (
  code text PRIMARY KEY,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  sort_order integer NOT NULL DEFAULT 0,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS public.saas_tenant_subscriptions (
  id bigserial PRIMARY KEY,
  tenant_id text NOT NULL UNIQUE,
  plan_code text NOT NULL,
  status text NOT NULL,
  starts_at timestamptz,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS public.saas_tenant_entitlements (
  id bigserial PRIMARY KEY,
  tenant_id text NOT NULL,
  module_code text NOT NULL,
  status text NOT NULL,
  starts_at timestamptz,
  expires_at timestamptz,
  limits text NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, module_code)
);
CREATE TABLE IF NOT EXISTS public.saas_usage_buckets (
  tenant_id text NOT NULL,
  module_code text NOT NULL,
  period_key text NOT NULL,
  metric text NOT NULL,
  committed bigint NOT NULL DEFAULT 0,
  reserved bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, module_code, period_key, metric)
);
CREATE TABLE IF NOT EXISTS public.saas_usage_events (
  event_id text PRIMARY KEY, tenant_id text NOT NULL, module_code text NOT NULL, metric text NOT NULL,
  quantity bigint NOT NULL, period_key text NOT NULL, source_type text NOT NULL, source_id text NOT NULL,
  member_id text NOT NULL DEFAULT '', idempotency_key text NOT NULL, status text NOT NULL,
  occurred_at timestamptz NOT NULL, storage_snapshot bigint, storage_snapshot_at timestamptz,
  reversal_of text UNIQUE, metadata text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key));
CREATE TABLE IF NOT EXISTS public.saas_usage_event_outbox (
  id bigserial PRIMARY KEY, event_id text NOT NULL UNIQUE REFERENCES public.saas_usage_events(event_id), destination text NOT NULL DEFAULT 'openmeter',
  status text NOT NULL DEFAULT 'pending', attempts integer NOT NULL DEFAULT 0, next_attempt_at timestamptz, last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.saas_subscription_audit_logs (
  id bigserial PRIMARY KEY, tenant_id text NOT NULL, module_code text NOT NULL DEFAULT '', action text NOT NULL, actor_id text NOT NULL DEFAULT '', reason text NOT NULL DEFAULT '', payload text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.account_member_token_locks (organization_id varchar(128) PRIMARY KEY, updated_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS public.account_member_token_allocations (
 organization_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL, metric varchar(32) NOT NULL DEFAULT 'token', allocated bigint NOT NULL DEFAULT 0,
 version bigint NOT NULL DEFAULT 0, active boolean NOT NULL DEFAULT false, window_start timestamptz NOT NULL, window_end timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY (organization_id, member_id, metric), CHECK(metric='token'), CHECK(allocated>=0 AND version>=0 AND window_end>window_start));
CREATE TABLE IF NOT EXISTS public.account_member_token_operations (
 idempotency_key varchar(128) PRIMARY KEY, organization_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL, fingerprint char(64) NOT NULL,
 target bigint NOT NULL, version bigint NOT NULL, allocated bigint NOT NULL, consumed bigint NOT NULL, active boolean NOT NULL, window_start timestamptz NOT NULL, window_end timestamptz NOT NULL, created_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS public.account_member_token_audit_events (
 id bigserial PRIMARY KEY, organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL, operation varchar(32) NOT NULL,
 target bigint NOT NULL, allocated bigint NOT NULL, consumed bigint NOT NULL, version bigint NOT NULL, idempotency_key varchar(128) NOT NULL UNIQUE, created_at timestamptz NOT NULL);
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'commercial_reader') THEN
    EXECUTE 'CREATE ROLE commercial_reader LOGIN';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'commercial_runtime') THEN
    EXECUTE 'CREATE ROLE commercial_runtime LOGIN';
  END IF;
END
$$;
ALTER ROLE commercial_reader LOGIN PASSWORD :'commercial_password';
ALTER ROLE commercial_runtime LOGIN PASSWORD :'commercial_password';
GRANT CONNECT ON DATABASE commercial TO commercial_reader, commercial_runtime;
GRANT USAGE ON SCHEMA public TO commercial_reader, commercial_runtime;
GRANT SELECT ON TABLE public.saas_tenant_subscriptions, public.saas_plans, public.saas_tenant_entitlements, public.saas_usage_buckets TO commercial_reader;
ALTER ROLE commercial_reader SET default_transaction_read_only=on;
GRANT SELECT ON TABLE public.saas_tenant_subscriptions, public.saas_plans, public.saas_tenant_entitlements TO commercial_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.saas_usage_buckets, public.saas_usage_events, public.saas_usage_event_outbox, public.saas_subscription_audit_logs TO commercial_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.account_member_token_locks, public.account_member_token_allocations, public.account_member_token_operations, public.account_member_token_audit_events TO commercial_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO commercial_runtime;
ALTER ROLE commercial_runtime SET statement_timeout='10s';
