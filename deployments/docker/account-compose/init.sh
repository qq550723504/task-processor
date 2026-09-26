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
membership_db_owner_secret=/secrets/membership-owner
membership_runtime_secret=/secrets/membership-runtime
runtime=/runtime
frontend=/frontend
identity_port=${ACCOUNT_IDENTITY_PORT:?ACCOUNT_IDENTITY_PORT is required}
application_port=${ACCOUNT_APPLICATION_PORT:?ACCOUNT_APPLICATION_PORT is required}
commercial_database=${ACCOUNT_COMMERCIAL_DATABASE:-commercial}
case "$commercial_database" in
  ''|[!a-z]*|*[!a-z0-9_]*) echo 'invalid commercial database name' >&2; exit 1 ;;
esac
if [ "${#commercial_database}" -gt 63 ]; then echo 'commercial database name is too long' >&2; exit 1; fi
case "${ACCOUNT_ISOLATED_TRIAL_CATALOG:-}" in
  ''|ISOLATED_TRIAL_ONLY) ;;
  *) echo 'invalid isolated trial catalog confirmation' >&2; exit 1 ;;
esac

read_bootstrap_user_id() {
  bootstrap_user_id=$(tr -d '\r\n' < "$runtime/bootstrap-user-id")
  case "$bootstrap_user_id" in
    ''|*[!ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-]*) echo 'invalid local bootstrap identity output' >&2; exit 1 ;;
  esac
  if [ "${#bootstrap_user_id}" -gt 256 ]; then echo 'invalid local bootstrap identity output' >&2; exit 1; fi
}

install_listingkit_authorization() {
  read_bootstrap_user_id
  jq --rawfile bootstrap_user_id "$runtime/bootstrap-user-id" \
    '.listingKitAuthorization = {platformAdminUsers: [($bootstrap_user_id | sub("[\\r\\n]+$"; ""))], platformAdminRoles: []}' \
    "$runtime/current-application.json" > "$runtime/current-application.json.tmp"
  chmod 600 "$runtime/current-application.json.tmp"
  mv "$runtime/current-application.json.tmp" "$runtime/current-application.json"
}

install_subscription_directory_token() {
  if [ ! -s "$runtime/membership-read.pat" ]; then
    echo 'subscription directory read credential is unavailable' >&2
    exit 1
  fi
  jq --rawfile directory_token "$runtime/membership-read.pat" \
    '.identity.tenantDirectoryToken = ($directory_token | sub("[\\r\\n]+$"; ""))' \
    "$runtime/current-application.json" > "$runtime/current-application.json.tmp"
  chmod 600 "$runtime/current-application.json.tmp"
  mv "$runtime/current-application.json.tmp" "$runtime/current-application.json"
}

migrate_commercial_owner_schema() {
  cat > "$work/commercial-owner-schema.json" <<EOF
{"host":"127.0.0.1","port":5433,"user":"commercial_schema_owner","password":"$(tr -d '\r\n' < "$commercial_db_owner_secret/commercial-db-password")","database":"${commercial_database}","maxConnections":2,"maxIdleConnections":1}
EOF
  cat > "$work/canonical-money-schema.json" <<EOF
{"host":"127.0.0.1","port":5433,"user":"referral_owner","password":"$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")","database":"referrals","maxConnections":2,"maxIdleConnections":1}
EOF
  commercial-owner-schema-migrate -config "$work/commercial-owner-schema.json" -money-config "$work/canonical-money-schema.json"
}

umask 077
if [ -f "$state/.init-complete" ]; then
  if [ "${ACCOUNT_ISOLATED_TRIAL_CATALOG:-}" = ISOLATED_TRIAL_ONLY ]; then
    echo 'isolated trial catalog can only be provisioned on first initialization' >&2
    exit 1
  fi
  jq -e --arg database "$commercial_database" '
    .sourceAccountDatabase.port == 5433 and .sourceAccountDatabase.database == "source_accounts" and
    .commercialDatabase.port == 5433 and .commercialDatabase.database == $database and
    .commercialOwnerDatabase.port == 5433 and .commercialOwnerDatabase.database == $database and
    .referrals.referralDatabase.port == 5433 and .referrals.referralDatabase.database == "referrals" and
    .membership.database.port == 5433 and .membership.database.database == "membership"
  ' "$runtime/current-application.json" >/dev/null || { echo 'database topology changed; use a new project' >&2; exit 1; }
  work=$(mktemp -d)
  trap 'rm -rf "$work"' EXIT
  cat > "$work/source-account-schema.yaml" <<EOF
database:
  host: 127.0.0.1
  port: 5433
  user: source_account_owner
  password: "$(tr -d '\r\n' < "$source_db_owner_secret/source-db-password")"
  database: source_accounts
  max_connections: 2
  max_idle_connections: 1
  connection_max_lifetime: 1h
EOF
  source-account-registry-schema-init -config "$work/source-account-schema.yaml"
  psql "postgresql://source_account_owner:$(tr -d '\r\n' < "$source_db_owner_secret/source-db-password")@127.0.0.1:5433/source_accounts?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
GRANT CONNECT ON DATABASE source_accounts TO source_account_runtime;
GRANT USAGE ON SCHEMA public TO source_account_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.account_business_profiles TO source_account_runtime;
GRANT SELECT, INSERT ON TABLE public.account_business_profile_audit_events TO source_account_runtime;
GRANT USAGE, SELECT ON SEQUENCE public.account_business_profile_audit_events_id_seq TO source_account_runtime;
SQL
  commercial_dsn="postgresql://commercial_schema_owner:$(tr -d '\r\n' < "$commercial_db_owner_secret/commercial-db-password")@127.0.0.1:5433/${commercial_database}?sslmode=disable"
  psql "$commercial_dsn" -v ON_ERROR_STOP=1 -v "runtime_roles_ready=true" -f "$terraform_source/commercial-schema.sql"
  cat > "$work/commercial-schema.yaml" <<EOF
commercialDatabase:
  host: 127.0.0.1
  port: 5433
  user: commercial_schema_owner
  password: "$(tr -d '\r\n' < "$commercial_db_owner_secret/commercial-db-password")"
  database: ${commercial_database}
  max_connections: 2
  max_idle_connections: 1
  connection_max_lifetime: 1h
EOF
  listingkit-schema-migrate -config "$work/commercial-schema.yaml" -scope commercial -log-level warn
  psql "$commercial_dsn" -v ON_ERROR_STOP=1 -v "runtime_roles_ready=true" <<SQL
GRANT CONNECT ON DATABASE ${commercial_database} TO commercial_runtime;
GRANT USAGE ON SCHEMA public TO commercial_runtime;
REVOKE INSERT, UPDATE, DELETE ON TABLE public.saas_plans, public.saas_plan_modules, public.saas_tenant_subscriptions, public.saas_tenant_entitlements FROM commercial_runtime;
GRANT CONNECT ON DATABASE ${commercial_database} TO commercial_owner_runtime;
GRANT USAGE ON SCHEMA public TO commercial_owner_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.saas_modules, public.saas_plans, public.saas_plan_modules, public.saas_tenant_subscriptions, public.saas_tenant_entitlements, public.saas_usage_counters, public.saas_usage_counter_adjustments, public.saas_subscription_audit_logs TO commercial_owner_runtime;
GRANT DELETE ON TABLE public.saas_modules, public.saas_plan_modules, public.saas_tenant_entitlements TO commercial_owner_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO commercial_owner_runtime;
SQL
  install_listingkit_authorization
  install_subscription_directory_token
  psql "postgresql://referral_owner:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5433/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -f "$terraform_source/referral-economics-schema.sql"
  psql "postgresql://referral_owner:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5433/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -v runtime_roles_ready=true -f "$terraform_source/referral-grants.sql"
  migrate_commercial_owner_schema
  membership_dsn="postgresql://membership_owner:$(tr -d '\r\n' < "$membership_db_owner_secret/membership-db-password")@127.0.0.1:5433/membership?sslmode=disable"
  printf '%s\n' "$membership_dsn" > "$work/membership-owner-dsn"
  chmod 600 "$work/membership-owner-dsn"
  organization-membership-schema-init -dsn-file "$work/membership-owner-dsn"
  psql "$membership_dsn" -v ON_ERROR_STOP=1 <<SQL
GRANT CONNECT ON DATABASE membership TO organization_membership_runtime;
GRANT USAGE ON SCHEMA public TO organization_membership_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.organization_member_operations TO organization_membership_runtime;
GRANT SELECT, INSERT ON TABLE public.organization_member_audit_events TO organization_membership_runtime;
SQL
  exit 0
fi
if [ -f "$state/.init-started" ]; then echo 'local initialization is incomplete; recreate this Compose project' >&2; exit 1; fi
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

read_bootstrap_user_id
issuer="https://localhost:${identity_port}"
public_app="https://localhost:${application_port}"
cat > "$runtime/current-application.json.tmp" <<EOF
{
  "schemaVersion": 1,
  "listingKitAuthorization": {"platformAdminUsers": ["${bootstrap_user_id}"], "platformAdminRoles": []},
  "listen": {"host": "127.0.0.1", "port": 8085},
  "identity": {
    "issuerURL": "${issuer}",
    "authorizationAPIURL": "${issuer}",
    "clientID": "$(tr -d '\r\n' < "$runtime/api-client-id")",
    "clientSecret": "$(tr -d '\r\n' < "$runtime/api-client-secret")",
    "projectID": "$(tr -d '\r\n' < "$runtime/project-id")"
  },
  "sourceAccountDatabase": {"host": "127.0.0.1", "port": 5433, "user": "source_account_runtime", "password": "$(tr -d '\r\n' < "$source_runtime_secret/source-runtime-password")", "database": "source_accounts", "maxConnections": 4},
  "commercialDatabase": {"host": "127.0.0.1", "port": 5433, "user": "commercial_runtime", "password": "$(tr -d '\r\n' < "$commercial_runtime_secret/commercial-reader-password")", "database": "${commercial_database}", "maxConnections": 4},
  "commercialOwnerDatabase": {"host": "127.0.0.1", "port": 5433, "user": "commercial_owner_runtime", "password": "$(tr -d '\r\n' < "$commercial_runtime_secret/commercial-owner-password")", "database": "${commercial_database}", "maxConnections": 2},
  "membership": {
    "providerOrigin": "${issuer}",
    "readToken": "$(tr -d '\r\n' < "$runtime/membership-read.pat")",
    "writeToken": "$(tr -d '\r\n' < "$runtime/membership-write.pat")",
    "database": {"host": "127.0.0.1", "port": 5433, "user": "organization_membership_runtime", "password": "$(tr -d '\r\n' < "$membership_runtime_secret/membership-runtime-password")", "database": "membership", "maxConnections": 4}
  },
  "referrals": {
    "enabled": true,
    "issuer": "${issuer}",
    "instanceID": "local-account-center-compose",
    "signupOrganizationID": "$(tr -d '\r\n' < "$runtime/signup-org-id")",
    "providerOrigin": "${issuer}",
    "officialLoginOrigin": "${issuer}",
    "publicAppOrigin": "${public_app}",
    "credentialFile": "/private/runtime/provider-machine.pat",
    "serviceCredentialFile": "/private/runtime/service-credential",
    "lookupKeyFile": "/private/runtime/lookup-key",
    "keyID": "local-v1",
    "proofKeyFiles": {"local-v1": "/private/runtime/proof-key"},
    "encryptionKeyFiles": {"local-v1": "/private/runtime/encryption-key"},
    "providerCAFile": "/private/ca/root-ca.pem",
    "referralDatabase": {"host": "127.0.0.1", "port": 5433, "user": "referral_runtime", "password": "$(tr -d '\r\n' < "$referral_runtime_secret/referral-runtime-password")", "database": "referrals", "maxConnections": 4}
  }
}
EOF
chmod 600 "$runtime/current-application.json.tmp"
mv "$runtime/current-application.json.tmp" "$runtime/current-application.json"
install_subscription_directory_token

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cat > "$work/source-account-schema.yaml" <<EOF
database:
  host: 127.0.0.1
  port: 5433
  user: source_account_owner
  password: "$(tr -d '\r\n' < "$source_db_owner_secret/source-db-password")"
  database: source_accounts
  max_connections: 2
  max_idle_connections: 1
  connection_max_lifetime: 1h
EOF
source-account-registry-schema-init -config "$work/source-account-schema.yaml"
psql "postgresql://source_account_owner:$(tr -d '\r\n' < "$source_db_owner_secret/source-db-password")@127.0.0.1:5433/source_accounts?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE source_accounts TO source_account_runtime;
GRANT USAGE ON SCHEMA public TO source_account_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.source_account_resources TO source_account_runtime;
GRANT SELECT, INSERT ON TABLE public.source_account_operations TO source_account_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.account_business_profiles TO source_account_runtime;
GRANT SELECT, INSERT ON TABLE public.account_business_profile_audit_events TO source_account_runtime;
GRANT USAGE, SELECT ON SEQUENCE public.account_business_profile_audit_events_id_seq TO source_account_runtime;
SQL

psql "postgresql://commercial_schema_owner:$(tr -d '\r\n' < "$commercial_db_owner_secret/commercial-db-password")@127.0.0.1:5433/${commercial_database}?sslmode=disable" -v ON_ERROR_STOP=1 -v runtime_roles_ready=true -f "$terraform_source/commercial-schema.sql"
cat > "$work/commercial-schema.yaml" <<EOF
commercialDatabase:
  host: 127.0.0.1
  port: 5433
  user: commercial_schema_owner
  password: "$(tr -d '\r\n' < "$commercial_db_owner_secret/commercial-db-password")"
  database: ${commercial_database}
  max_connections: 2
  max_idle_connections: 1
  connection_max_lifetime: 1h
EOF
listingkit-schema-migrate -config "$work/commercial-schema.yaml" -scope commercial -log-level warn
psql "postgresql://commercial_schema_owner:$(tr -d '\r\n' < "$commercial_db_owner_secret/commercial-db-password")@127.0.0.1:5433/${commercial_database}?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
REVOKE INSERT, UPDATE, DELETE ON TABLE public.saas_plans, public.saas_plan_modules, public.saas_tenant_subscriptions, public.saas_tenant_entitlements FROM commercial_runtime;
GRANT CONNECT ON DATABASE ${commercial_database} TO commercial_owner_runtime;
GRANT USAGE ON SCHEMA public TO commercial_owner_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.saas_modules, public.saas_plans, public.saas_plan_modules, public.saas_tenant_subscriptions, public.saas_tenant_entitlements, public.saas_usage_counters, public.saas_usage_counter_adjustments, public.saas_subscription_audit_logs TO commercial_owner_runtime;
GRANT DELETE ON TABLE public.saas_modules, public.saas_plan_modules, public.saas_tenant_entitlements TO commercial_owner_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO commercial_owner_runtime;
SQL

printf 'postgresql://referral_owner:%s@127.0.0.1:5433/referrals?sslmode=disable\n' "$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")" > "$work/referral-owner-dsn"
chmod 600 "$work/referral-owner-dsn"
referral-schema-init -dsn-file "$work/referral-owner-dsn"
psql "postgresql://referral_owner:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5433/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -f "$terraform_source/referral-economics-schema.sql"
psql "postgresql://referral_owner:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5433/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -v runtime_roles_ready=true -f "$terraform_source/referral-grants.sql"
migrate_commercial_owner_schema
if [ "${ACCOUNT_ISOLATED_TRIAL_CATALOG:-}" = ISOLATED_TRIAL_ONLY ]; then
  commercial-owner-schema-migrate -isolated-trial-catalog \
    -config "$work/commercial-owner-schema.json" \
    -expected-database "$commercial_database" -confirm ISOLATED_TRIAL_ONLY
fi

printf 'postgresql://membership_owner:%s@127.0.0.1:5433/membership?sslmode=disable\n' "$(tr -d '\r\n' < "$membership_db_owner_secret/membership-db-password")" > "$work/membership-owner-dsn"
chmod 600 "$work/membership-owner-dsn"
organization-membership-schema-init -dsn-file "$work/membership-owner-dsn"
psql "postgresql://membership_owner:$(tr -d '\r\n' < "$membership_db_owner_secret/membership-db-password")@127.0.0.1:5433/membership?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
REVOKE CREATE, TEMPORARY ON DATABASE membership FROM PUBLIC;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE membership TO organization_membership_runtime;
GRANT USAGE ON SCHEMA public TO organization_membership_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.organization_member_operations TO organization_membership_runtime;
GRANT SELECT, INSERT ON TABLE public.organization_member_audit_events TO organization_membership_runtime;
SQL

mv "$state/.init-started" "$state/.init-complete"
