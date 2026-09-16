#!/bin/sh
set -eu

private=/private
terraform_source=/terraform-source

umask 077
test -f "$private/secrets/operator-password"

until [ -f "$private/.terraform-complete" ]; do
  sleep 1
done

if [ -f "$private/.init-complete" ]; then
  exit 0
fi
if [ -f "$private/.init-started" ]; then
  echo "local initialization is incomplete; recreate this Compose project's named volumes with the documented destroy command" >&2
  exit 1
fi
touch "$private/.init-started"
chmod 600 "$private/.init-started"

mkdir -p "$private/runtime"

for name in lookup-key proof-key encryption-key service-credential; do
  openssl rand -hex 32 > "$private/runtime/$name.tmp"
  chmod 600 "$private/runtime/$name.tmp"
  mv "$private/runtime/$name.tmp" "$private/runtime/$name"
done

cat > "$private/runtime/current-application.json.tmp" <<EOF
{
  "schemaVersion": 1,
  "listen": {"host": "127.0.0.1", "port": 8085},
  "identity": {
    "issuerURL": "https://localhost:18443",
    "authorizationAPIURL": "https://localhost:18443",
    "clientID": "$(tr -d '\r\n' < "$private/runtime/api-client-id")",
    "clientSecret": "$(tr -d '\r\n' < "$private/runtime/api-client-secret")",
    "projectID": "$(tr -d '\r\n' < "$private/runtime/project-id")"
  },
  "sourceAccountDatabase": {"host": "127.0.0.1", "port": 5433, "user": "source_account_runtime", "password": "$(tr -d '\r\n' < "$private/secrets/source-runtime-password")", "database": "source_accounts", "maxConnections": 4},
  "commercialDatabase": {"host": "127.0.0.1", "port": 5434, "user": "commercial_reader", "password": "$(tr -d '\r\n' < "$private/secrets/commercial-reader-password")", "database": "commercial", "maxConnections": 4},
  "referrals": {
    "enabled": true,
    "issuer": "https://localhost:18443",
    "instanceID": "local-referrals-compose",
    "signupOrganizationID": "$(tr -d '\r\n' < "$private/runtime/signup-org-id")",
    "providerOrigin": "https://localhost:18443",
    "officialLoginOrigin": "https://localhost:18443",
    "publicAppOrigin": "https://localhost:18444",
    "credentialFile": "/private/runtime/provider-machine.pat",
    "serviceCredentialFile": "/private/runtime/service-credential",
    "lookupKeyFile": "/private/runtime/lookup-key",
    "keyID": "local-v1",
    "proofKeyFiles": {"local-v1": "/private/runtime/proof-key"},
    "encryptionKeyFiles": {"local-v1": "/private/runtime/encryption-key"},
    "providerCAFile": "/private/ca/root-ca.pem",
    "referralDatabase": {"host": "127.0.0.1", "port": 5435, "user": "referral_runtime", "password": "$(tr -d '\r\n' < "$private/secrets/referral-runtime-password")", "database": "referrals", "maxConnections": 4}
  }
}
EOF
chmod 600 "$private/runtime/current-application.json.tmp"
mv "$private/runtime/current-application.json.tmp" "$private/runtime/current-application.json"

cat > "$private/runtime/source-account-schema.yaml.tmp" <<EOF
database:
  host: 127.0.0.1
  port: 5433
  user: postgres
  password: "$(tr -d '\r\n' < "$private/secrets/source-db-password")"
  database: source_accounts
  max_connections: 2
  max_idle_connections: 1
  connection_max_lifetime: 1h
EOF
chmod 600 "$private/runtime/source-account-schema.yaml.tmp"
mv "$private/runtime/source-account-schema.yaml.tmp" "$private/runtime/source-account-schema.yaml"
source-account-registry-schema-init -config "$private/runtime/source-account-schema.yaml"
psql "postgresql://postgres:$(tr -d '\r\n' < "$private/secrets/source-db-password")@127.0.0.1:5433/source_accounts?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
CREATE ROLE source_account_runtime LOGIN PASSWORD '$(tr -d '\r\n' < "$private/secrets/source-runtime-password")';
GRANT CONNECT ON DATABASE source_accounts TO source_account_runtime;
GRANT USAGE ON SCHEMA public TO source_account_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.source_account_resources TO source_account_runtime;
GRANT SELECT, INSERT ON TABLE public.source_account_operations TO source_account_runtime;
ALTER ROLE source_account_runtime SET statement_timeout='10s';
SQL

psql "postgresql://postgres:$(tr -d '\r\n' < "$private/secrets/commercial-db-password")@127.0.0.1:5434/commercial?sslmode=disable" -v ON_ERROR_STOP=1 -v "commercial_password=$(tr -d '\r\n' < "$private/secrets/commercial-reader-password")" -f "$terraform_source/commercial-schema.sql"
psql "postgresql://postgres:$(tr -d '\r\n' < "$private/secrets/referral-db-password")@127.0.0.1:5435/referrals?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
CREATE ROLE referral_runtime LOGIN PASSWORD '$(tr -d '\r\n' < "$private/secrets/referral-runtime-password")';
SQL
printf 'postgresql://postgres:%s@127.0.0.1:5435/referrals?sslmode=disable\n' "$(tr -d '\r\n' < "$private/secrets/referral-db-password")" > "$private/runtime/referral-owner-dsn.tmp"
chmod 600 "$private/runtime/referral-owner-dsn.tmp"
mv "$private/runtime/referral-owner-dsn.tmp" "$private/runtime/referral-owner-dsn"
referral-schema-init -dsn-file "$private/runtime/referral-owner-dsn"
psql "postgresql://postgres:$(tr -d '\r\n' < "$private/secrets/referral-db-password")@127.0.0.1:5435/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -f "$terraform_source/referral-grants.sql"

printf '%s\n' 'local-bootstrap-operator@localhost' > "$private/runtime/operator-login.txt.tmp"
chmod 600 "$private/runtime/operator-login.txt.tmp"
mv "$private/runtime/operator-login.txt.tmp" "$private/runtime/operator-login.txt"
mv "$private/.init-started" "$private/.init-complete"
