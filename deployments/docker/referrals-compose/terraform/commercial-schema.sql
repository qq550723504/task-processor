CREATE TABLE public.saas_plans (
  code text PRIMARY KEY,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  sort_order integer NOT NULL DEFAULT 0,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE public.saas_tenant_subscriptions (
  id bigserial PRIMARY KEY,
  tenant_id text NOT NULL UNIQUE,
  plan_code text NOT NULL,
  status text NOT NULL,
  starts_at timestamptz,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE public.saas_tenant_entitlements (
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
CREATE TABLE public.saas_usage_buckets (
  tenant_id text NOT NULL,
  module_code text NOT NULL,
  period_key text NOT NULL,
  metric text NOT NULL,
  committed bigint NOT NULL DEFAULT 0,
  reserved bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, module_code, period_key, metric)
);
CREATE ROLE commercial_reader LOGIN PASSWORD :'commercial_password';
GRANT CONNECT ON DATABASE commercial TO commercial_reader;
GRANT USAGE ON SCHEMA public TO commercial_reader;
GRANT SELECT ON TABLE public.saas_tenant_subscriptions, public.saas_plans, public.saas_tenant_entitlements, public.saas_usage_buckets TO commercial_reader;
ALTER ROLE commercial_reader SET default_transaction_read_only=on;
ALTER ROLE commercial_reader SET statement_timeout='10s';
