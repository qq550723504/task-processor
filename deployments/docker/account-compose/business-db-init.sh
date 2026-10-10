#!/bin/sh
# Run by the official PostgreSQL entrypoint only for a new PGDATA directory.
# Cluster administration ends here; schema installers use their own non-superuser.
set -eu

commercial_database=${ACCOUNT_COMMERCIAL_DATABASE:-commercial}
case "$commercial_database" in
  ''|[!a-z]*|*[!a-z0-9_]*) echo 'invalid commercial database name' >&2; exit 1 ;;
  postgres|template0|template1|source_accounts|referrals|membership|product_acquisition|image_agent|product_agent|store_center|knowledge)
    echo 'commercial database must have its own name' >&2; exit 1 ;;
esac
test "${#commercial_database}" -le 63
case "${ACCOUNT_ISSUE36_LOCAL_TRIAL:-}" in
  ''|ISOLATED_TRIAL_ONLY) ;;
  *) echo 'invalid #36 local trial opt-in' >&2; exit 1 ;;
esac

create_role() {
  # A failing substitution inside psql's arguments does not trigger set -e.
  # Read separately so unreadable/empty secrets cannot create passwordless roles.
  role_password=$(cat "$2")
  case "$role_password" in ''|*[!0-9a-f]*) echo 'invalid database credential file' >&2; exit 1 ;; esac
  test "${#role_password}" -eq 48
  psql -X -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
    -v "role=$1" -v "password=$role_password" <<'SQL'
CREATE ROLE :"role" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'password';
SQL
  case "$1" in *_runtime)
    psql -X -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres -v "role=$1" <<'SQL'
ALTER ROLE :"role" SET statement_timeout='10s';
SQL
    ;;
  esac
}

create_database() {
  psql -X -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
    -v "database=$1" -v "owner=$2" <<'SQL'
CREATE DATABASE :"database" OWNER :"owner";
REVOKE ALL ON DATABASE :"database" FROM PUBLIC;
SQL
  psql -X -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$1" <<'SQL'
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
SQL
}

create_role store_center_owner /secrets/store-owner/store-owner-password
create_role store_center_runtime /secrets/store-runtime/store-runtime-password
create_role source_account_owner /secrets/source-owner/source-db-password
create_role commercial_schema_owner /secrets/commercial-owner/commercial-db-password
create_role referral_owner /secrets/referral-owner/referral-db-password
create_role membership_owner /secrets/membership-owner/membership-db-password
create_role acquisition_owner /secrets/acquisition-owner/acquisition-db-password
create_role image_agent_owner /secrets/image-owner/image-db-password
create_role product_agent_owner /secrets/product-agent-owner/product-agent-db-password
create_role account_audit_image_reader /secrets/image-audit-reader/password
create_role account_audit_product_reader /secrets/product-audit-reader/password
create_role source_account_runtime /secrets/source-runtime/source-runtime-password
create_role commercial_runtime /secrets/commercial-runtime/commercial-reader-password
create_role commercial_owner_runtime /secrets/commercial-runtime/commercial-owner-password
create_role money_owner_runtime /secrets/commercial-runtime/money-owner-password
create_role referral_runtime /secrets/referral-runtime/referral-runtime-password
create_role organization_membership_runtime /secrets/membership-runtime/membership-runtime-password
create_role organization_role_policy_reader /secrets/role-policy-reader/password
create_role source_acquisition_runtime /secrets/acquisition-runtime/acquisition-runtime-password
if [ "${ACCOUNT_DATA_SERVICES_ENABLED:-}" = 1 ]; then
 create_role data_services_runtime /secrets/data-services-runtime/password
fi
create_role image_agent_runtime /secrets/image-runtime/image-runtime-password
create_role image_agent_worker_runtime /secrets/image-worker/image-worker-password
if [ "${ACCOUNT_ISSUE36_LOCAL_TRIAL:-}" = ISOLATED_TRIAL_ONLY ]; then
 create_role issue36_trial_runtime /secrets/issue36-trial-runtime/password
fi

create_database store_center store_center_owner
create_database source_accounts source_account_owner
create_database "$commercial_database" commercial_schema_owner
create_database referrals referral_owner
create_database membership membership_owner
create_database product_acquisition acquisition_owner
create_database image_agent image_agent_owner
create_database product_agent product_agent_owner

psql -X -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<'SQL'
ALTER ROLE account_audit_image_reader SET default_transaction_read_only=on;
ALTER ROLE account_audit_product_reader SET default_transaction_read_only=on;
ALTER ROLE account_audit_image_reader SET statement_timeout='10s';
ALTER ROLE account_audit_product_reader SET statement_timeout='10s';
SQL

if [ "${ACCOUNT_KNOWLEDGE_ENABLED:-}" = 1 ]; then
 create_role knowledge_owner /secrets/knowledge-owner/owner-password
 create_role knowledge_runtime /secrets/knowledge-runtime/runtime-password
 create_database knowledge knowledge_owner
fi

psql -X -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<'SQL'
REVOKE ALL ON DATABASE postgres, template1 FROM PUBLIC;
SQL
# No runtime gets CONNECT until its existing schema installer grants it.
# Incomplete initialization stays unhealthy on subsequent container starts.
printf '%s\n' "$commercial_database" > "$PGDATA/.business-db-ready"
