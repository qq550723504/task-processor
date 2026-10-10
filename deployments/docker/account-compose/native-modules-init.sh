#!/bin/sh
# Consume existing owner installers once, after the base/Product fresh install.
set -eu
umask 077
test "${ACCOUNT_NATIVE_MODULES_ENABLED:-}" = 1
case "${ACCOUNT_APPLICATION_PORT:-}" in ''|*[!0-9]*) echo 'application port required' >&2; exit 1 ;; esac
test "$ACCOUNT_APPLICATION_PORT" -ge 1 && test "$ACCOUNT_APPLICATION_PORT" -le 65535
if test -f /state/.complete; then
 test "$(cat /state/.complete)" = native-modules-v1 && test -s /runtime/current-application.json
 test -s /qualification-storage-secret/root-password || { echo 'Retained qualification storage requires its original private root secret' >&2; exit 1; }
 echo 'Native module facts and configuration retained'
 exit 0
fi
test ! -f /state/.started || { echo 'Inspect interrupted empty installation before retry; facts retained' >&2; exit 1; }
test -s /base/current-application.json
jq -e '.productCollections == true and .dataServices != null' /base/current-application.json >/dev/null
touch /state/.started
export PGHOST=127.0.0.1 PGPORT=5433 PGUSER=business_cluster_admin
export PGPASSWORD="$(cat /secrets/admin/admin-password)"

create_owner() {
 psql -X -v ON_ERROR_STOP=1 -d postgres -At -c "SELECT count(*) FROM pg_database WHERE datname='$1'" | grep -qx 0
 openssl rand -hex 24 | tr -d '\n' > "/state/$1-owner-password"
 openssl rand -hex 24 | tr -d '\n' > "/state/$1-runtime-password"
 psql -X -v ON_ERROR_STOP=1 -d postgres -v database="$1" -v owner="$2" -v runtime="$3" -v ownerpassword="$(cat /state/$1-owner-password)" -v runtimepassword="$(cat /state/$1-runtime-password)" <<'SQL'
CREATE ROLE :"owner" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'ownerpassword';
CREATE ROLE :"runtime" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'runtimepassword';
ALTER ROLE :"runtime" SET statement_timeout='10s';
CREATE DATABASE :"database" OWNER :"owner";
REVOKE ALL ON DATABASE :"database" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"database" TO :"runtime";
SQL
 psql -X -v ON_ERROR_STOP=1 -d "$1" -c 'REVOKE CREATE ON SCHEMA public FROM PUBLIC;'
}

create_owner notification_center notification_center_owner notification_center_runtime
create_owner agent_customization agent_customization_owner agent_customization_runtime
create_owner ai_projects ai_projects_owner ai_projects_runtime
create_owner reports report_center_owner report_center_runtime
psql -X -v ON_ERROR_STOP=1 -d postgres -c 'ALTER ROLE agent_customization_owner NOLOGIN; ALTER ROLE report_center_owner NOLOGIN;'
NOTIFICATION_CENTER_SCHEMA_DSN="host=127.0.0.1 port=5433 user=notification_center_owner password=$(cat /state/notification_center-owner-password) dbname=notification_center sslmode=disable" notification-center-schema-init --runtime-role notification_center_runtime
AGENT_CUSTOMIZATION_SCHEMA_DSN="host=127.0.0.1 port=5433 user=business_cluster_admin password=$PGPASSWORD dbname=agent_customization sslmode=disable" agent-customization-schema-init --runtime-role agent_customization_runtime
PROJECT_CENTER_SCHEMA_DSN="host=127.0.0.1 port=5433 user=ai_projects_owner password=$(cat /state/ai_projects-owner-password) dbname=ai_projects sslmode=disable" project-center-schema-init --runtime-role ai_projects_runtime
printf 'host=127.0.0.1 port=5433 user=business_cluster_admin password=%s dbname=reports sslmode=disable\n' "$PGPASSWORD" > /state/reports-owner-dsn
report-center-schema-init -dsn-file /state/reports-owner-dsn -confirm-database reports -install-empty-schema

openssl rand -hex 24 | tr -d '\n' > /state/product-runtime-password
psql -X -v ON_ERROR_STOP=1 -d postgres -v password="$(cat /state/product-runtime-password)" <<'SQL'
CREATE ROLE product_agent_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'password';
ALTER ROLE product_agent_runtime SET statement_timeout='10s';
GRANT CONNECT ON DATABASE product_agent TO product_agent_runtime;
SQL
psql -X -v ON_ERROR_STOP=1 -d product_agent -c 'GRANT SELECT ON public.ai_invocations TO product_agent_runtime;'
AGENT_CONFIGURATION_SCHEMA_DSN="host=127.0.0.1 port=5433 user=business_cluster_admin password=$PGPASSWORD dbname=product_agent sslmode=disable" agent-configuration-schema-init --runtime-role product_agent_runtime

create_owner tool_market tool_market_owner tool_market_runtime
psql -X -v ON_ERROR_STOP=1 -d tool_market <<'SQL'
ALTER ROLE tool_market_owner NOLOGIN;
CREATE SCHEMA tool_market AUTHORIZATION tool_market_owner;
SET ROLE tool_market_owner;
\i /etc/tool-market-schema.sql
RESET ROLE;
GRANT USAGE ON SCHEMA tool_market TO tool_market_runtime;
GRANT SELECT,INSERT,UPDATE ON tool_market.activations,tool_market.requests,tool_market.commands TO tool_market_runtime;
GRANT SELECT,INSERT ON tool_market.events TO tool_market_runtime;
SQL

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
jq -n --rawfile password /secrets/store-owner/store-owner-password '{host:"127.0.0.1",port:5433,user:"store_center_owner",password:($password|rtrimstr("\n")),database:"store_center",maxConnections:2,maxIdleConnections:1}' > "$work/store-owner.json"
store-center-schema-init -config "$work/store-owner.json" --operations-cockpit
openssl rand -hex 24 | tr -d '\n' > /qualification-storage-secret/root-password
openssl rand -hex 16 | tr -d '\n' > /runtime/qualification-access-key
openssl rand -hex 24 | tr -d '\n' > /runtime/qualification-secret-key
jq --arg appURL "https://localhost:$ACCOUNT_APPLICATION_PORT/capture/1688" --rawfile nc /state/notification_center-runtime-password --rawfile ac /state/agent_customization-runtime-password --rawfile pc /state/ai_projects-runtime-password --rawfile rc /state/reports-runtime-password --rawfile tc /state/tool_market-runtime-password --rawfile pr /state/product-runtime-password --rawfile token /base/membership-read.pat --rawfile ak /runtime/qualification-access-key --rawfile sk /runtime/qualification-secret-key '
 def db($name;$role;$password): {host:"127.0.0.1",port:5433,user:$role,password:($password|rtrimstr("\n")),database:$name,maxConnections:4};
 .notificationCenterDatabase=db("notification_center";"notification_center_runtime";$nc) |
 .agentCustomizationDatabase=db("agent_customization";"agent_customization_runtime";$ac) |
 .projectCenter={database:db("ai_projects";"ai_projects_runtime";$pc)} |
 .reportCenter={database:db("reports";"report_center_runtime";$rc)} |
 .toolMarket={database:db("tool_market";"tool_market_runtime";$tc),captureAppURL:$appURL,packageDirectory:"/private/tool-release"} |
 .productAgent={enabled:false,database:db("product_agent";"product_agent_runtime";$pr)} |
 .storeCenter.operationsCockpit=true |
 .identity.tenantDirectoryToken=($token|rtrimstr("\n")) |
 .supplyMarket={storage:{region:"us-east-1",bucket:"qualifications",accessKeyId:($ak|rtrimstr("\n")),secretAccessKey:($sk|rtrimstr("\n")),mode:"aws",endpoint:"http://127.0.0.1:9000"}}
 ' /base/current-application.json > /runtime/current-application.json.tmp
chmod 600 /runtime/*
mv /runtime/current-application.json.tmp /runtime/current-application.json
printf '%s\n' native-modules-v1 > /state/.complete
echo 'Existing native owners and serving configuration installed; no sample business data or provider calls'
