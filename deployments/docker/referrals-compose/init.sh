#!/bin/sh
set -eu

state=/schema-state
terraform_source=/terraform-source
source_db_owner_secret=/secrets/source-owner
source_runtime_secret=/secrets/source-runtime
commercial_db_owner_secret=/secrets/commercial-owner
commercial_runtime_secret=/secrets/commercial-runtime
referral_db_owner_secret=/secrets/referral-owner
referral_runtime_secret=/secrets/referral-runtime
runtime=/runtime
frontend=/frontend

umask 077
if [ -f "$state/.init-complete" ]; then
  exit 0
fi
if [ -f "$state/.init-started" ]; then
  echo "local initialization is incomplete; recreate this Compose project's named volumes with the documented destroy command" >&2
  exit 1
fi
touch "$state/.init-started"
chmod 600 "$state/.init-started"

mkdir -p "$runtime" "$frontend"
for name in lookup-key proof-key encryption-key; do
  openssl rand -hex 32 > "$runtime/$name.tmp"
  chmod 600 "$runtime/$name.tmp"
  mv "$runtime/$name.tmp" "$runtime/$name"
done
openssl rand -hex 32 > "$runtime/service-credential.tmp"
chmod 600 "$runtime/service-credential.tmp"
mv "$runtime/service-credential.tmp" "$runtime/service-credential"
cp "$runtime/service-credential" "$frontend/service-credential.tmp"
chmod 600 "$frontend/service-credential.tmp"
mv "$frontend/service-credential.tmp" "$frontend/service-credential"

cat > "$runtime/current-application.json.tmp" <<EOF
{
  "schemaVersion": 1,
  "listen": {"host": "127.0.0.1", "port": 8085},
  "identity": {
    "issuerURL": "https://localhost:18443",
    "authorizationAPIURL": "https://localhost:18443",
    "clientID": "$(tr -d '\r\n' < "$runtime/api-client-id")",
    "clientSecret": "$(tr -d '\r\n' < "$runtime/api-client-secret")",
    "projectID": "$(tr -d '\r\n' < "$runtime/project-id")"
  },
  "sourceAccountDatabase": {"host": "127.0.0.1", "port": 5433, "user": "source_account_runtime", "password": "$(tr -d '\r\n' < "$source_runtime_secret/source-runtime-password")", "database": "source_accounts", "maxConnections": 4},
  "commercialDatabase": {"host": "127.0.0.1", "port": 5434, "user": "commercial_runtime", "password": "$(tr -d '\r\n' < "$commercial_runtime_secret/commercial-reader-password")", "database": "commercial", "maxConnections": 4},
  "referrals": {
    "enabled": true,
    "issuer": "https://localhost:18443",
    "instanceID": "local-referrals-compose",
    "signupOrganizationID": "$(tr -d '\r\n' < "$runtime/signup-org-id")",
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
    "referralDatabase": {"host": "127.0.0.1", "port": 5435, "user": "referral_runtime", "password": "$(tr -d '\r\n' < "$referral_runtime_secret/referral-runtime-password")", "database": "referrals", "maxConnections": 4}
  }
}
EOF
chmod 600 "$runtime/current-application.json.tmp"
mv "$runtime/current-application.json.tmp" "$runtime/current-application.json"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cat > "$work/source-account-schema.yaml" <<EOF
database:
  host: 127.0.0.1
  port: 5433
  user: postgres
  password: "$(tr -d '\r\n' < "$source_db_owner_secret/source-db-password")"
  database: source_accounts
  max_connections: 2
  max_idle_connections: 1
  connection_max_lifetime: 1h
EOF
source-account-registry-schema-init -config "$work/source-account-schema.yaml"
psql "postgresql://postgres:$(tr -d '\r\n' < "$source_db_owner_secret/source-db-password")@127.0.0.1:5433/source_accounts?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
CREATE ROLE source_account_runtime LOGIN PASSWORD '$(tr -d '\r\n' < "$source_runtime_secret/source-runtime-password")';
GRANT CONNECT ON DATABASE source_accounts TO source_account_runtime;
GRANT USAGE ON SCHEMA public TO source_account_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.source_account_resources TO source_account_runtime;
GRANT SELECT, INSERT ON TABLE public.source_account_operations TO source_account_runtime;
ALTER ROLE source_account_runtime SET statement_timeout='10s';
SQL

psql "postgresql://postgres:$(tr -d '\r\n' < "$commercial_db_owner_secret/commercial-db-password")@127.0.0.1:5434/commercial?sslmode=disable" -v ON_ERROR_STOP=1 -v "commercial_password=$(tr -d '\r\n' < "$commercial_runtime_secret/commercial-reader-password")" -f "$terraform_source/commercial-schema.sql"
psql "postgresql://postgres:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5435/referrals?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
CREATE ROLE referral_runtime LOGIN PASSWORD '$(tr -d '\r\n' < "$referral_runtime_secret/referral-runtime-password")';
SQL
printf 'postgresql://postgres:%s@127.0.0.1:5435/referrals?sslmode=disable\n' "$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")" > "$work/referral-owner-dsn"
chmod 600 "$work/referral-owner-dsn"
referral-schema-init -dsn-file "$work/referral-owner-dsn"
psql "postgresql://postgres:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5435/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -f "$terraform_source/referral-grants.sql"

mv "$state/.init-started" "$state/.init-complete"
