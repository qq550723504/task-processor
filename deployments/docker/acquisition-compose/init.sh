#!/bin/sh
set -eu
state=/schema-state; source_owner=/secrets/source-owner; source_runtime=/secrets/source-runtime; commercial_owner=/secrets/commercial-owner; commercial_runtime=/secrets/commercial-runtime; product_owner=/secrets/product-owner; product_runtime=/secrets/product-runtime; runtime=/runtime; frontend=/frontend
umask 077
if [ -f "$state/.init-complete" ]; then exit 0; fi
if [ -f "$state/.init-started" ]; then echo 'local initialization is incomplete; do not retry this project' >&2; exit 1; fi
touch "$state/.init-started"; chmod 600 "$state/.init-started"; mkdir -p "$runtime" "$frontend"
cat > "$runtime/current-application.json.tmp" <<EOF
{
  "schemaVersion": 1,
  "listen": {"host": "127.0.0.1", "port": 8085},
  "identity": {"issuerURL": "https://localhost:18443", "authorizationAPIURL": "https://localhost:18443", "clientID": "$(tr -d '\r\n' < "$runtime/api-client-id")", "clientSecret": "$(tr -d '\r\n' < "$runtime/api-client-secret")", "projectID": "$(tr -d '\r\n' < "$runtime/project-id")"},
  "sourceAccountDatabase": {"host": "127.0.0.1", "port": 5433, "user": "source_account_runtime", "password": "$(tr -d '\r\n' < "$source_runtime/source-runtime-password")", "database": "source_accounts", "maxConnections": 4},
  "commercialDatabase": {"host": "127.0.0.1", "port": 5434, "user": "commercial_runtime", "password": "$(tr -d '\r\n' < "$commercial_runtime/commercial-reader-password")", "database": "commercial", "maxConnections": 4},
  "productAcquisitionDatabase": {"host": "127.0.0.1", "port": 5435, "user": "source_acquisition_runtime", "password": "$(tr -d '\r\n' < "$product_runtime/product-runtime-password")", "database": "product_acquisition", "maxConnections": 4},
  "referrals": {"enabled": false}
}
EOF
chmod 600 "$runtime/current-application.json.tmp"; mv "$runtime/current-application.json.tmp" "$runtime/current-application.json"
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
cat > "$work/source-account-schema.yaml" <<EOF
database:
  host: 127.0.0.1
  port: 5433
  user: postgres
  password: "$(tr -d '\r\n' < "$source_owner/source-db-password")"
  database: source_accounts
  max_connections: 2
  max_idle_connections: 1
  connection_max_lifetime: 1h
EOF
source-account-registry-schema-init -config "$work/source-account-schema.yaml"
psql "postgresql://postgres:$(tr -d '\r\n' < "$source_owner/source-db-password")@127.0.0.1:5433/source_accounts?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
CREATE ROLE source_account_runtime LOGIN PASSWORD '$(tr -d '\r\n' < "$source_runtime/source-runtime-password")';
GRANT CONNECT ON DATABASE source_accounts TO source_account_runtime;
GRANT USAGE ON SCHEMA public TO source_account_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.source_account_resources TO source_account_runtime;
GRANT SELECT, INSERT ON TABLE public.source_account_operations TO source_account_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.account_business_profiles TO source_account_runtime;
GRANT SELECT, INSERT ON TABLE public.account_business_profile_audit_events TO source_account_runtime;
GRANT USAGE, SELECT ON SEQUENCE public.account_business_profile_audit_events_id_seq TO source_account_runtime;
ALTER ROLE source_account_runtime SET statement_timeout='10s';
SQL
psql "postgresql://postgres:$(tr -d '\r\n' < "$commercial_owner/commercial-db-password")@127.0.0.1:5434/commercial?sslmode=disable" -v ON_ERROR_STOP=1 -v "commercial_password=$(tr -d '\r\n' < "$commercial_runtime/commercial-reader-password")" -f /schemas/commercial-schema.sql
psql "postgresql://postgres:$(tr -d '\r\n' < "$product_owner/product-db-password")@127.0.0.1:5435/product_acquisition?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
CREATE ROLE source_acquisition_runtime LOGIN PASSWORD '$(tr -d '\r\n' < "$product_runtime/product-runtime-password")';
SQL
cat > "$work/product-acquisition-init.json" <<EOF
{"schemaVersion":1,"database":{"host":"127.0.0.1","port":5435,"user":"postgres","password":"$(tr -d '\r\n' < "$product_owner/product-db-password")","database":"product_acquisition"}}
EOF
chmod 600 "$work/product-acquisition-init.json"
product-acquisition-init -config "$work/product-acquisition-init.json" -confirm-empty-database product_acquisition
mv "$state/.init-started" "$state/.init-complete"
