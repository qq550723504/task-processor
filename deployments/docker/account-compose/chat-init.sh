#!/bin/sh
# One-time installation of the existing Conversation owner; no provider setup.
set -eu
umask 077
test -s /base/current-application.json
source_hash=$(sha256sum /base/current-application.json | cut -d ' ' -f 1)
organizations=$(printf '%s' "${CHAT_ALLOWED_ORGANIZATION_IDS:?explicit trial organizations required}" | jq -ce 'select(type=="array" and length>0 and length<=64 and length==(unique|length) and all(.[]; type=="string" and test("^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$")))')
if test -f /state/.complete; then
 test "$(cat /state/.complete)" = chat-conversations-v1
 test "$source_hash" = "$(cat /state/source-manifest.sha256)" || { echo 'Base manifest changed; inspect Chat reassembly, retained facts untouched' >&2; exit 1; }
 test -s /state/runtime-password
 test -s /runtime/current-application.json
 test "$(sha256sum /runtime/current-application.json | cut -d ' ' -f 1)" = "$(cat /state/runtime-manifest.sha256)"
 jq -e --argjson organizations "$organizations" --rawfile password /state/runtime-password '.aiWorkbench.enabled==true and .aiWorkbench.conversationOnly==true and .aiWorkbench.allowedOrganizationIds==$organizations and .aiWorkbench.database.password==$password' /runtime/current-application.json >/dev/null
 echo 'Chat conversations and original private runtime configuration retained'
 exit 0
fi
test ! -f /state/.started || { echo 'Inspect interrupted Chat installation before retry; facts retained' >&2; exit 1; }
jq -e '.ecoservices.enabled==true and .productAgent.enabled==false and .productAgent.database.database=="product_agent" and .aiWorkbench==null' /base/current-application.json >/dev/null
export PGHOST=127.0.0.1 PGPORT=5433 PGUSER=business_cluster_admin
export PGPASSWORD="$(cat /secrets/admin/admin-password)"
psql -X -v ON_ERROR_STOP=1 -d product_agent -At -c "SELECT (SELECT count(*) FROM pg_namespace WHERE nspname='ai_workbench')+(SELECT count(*) FROM pg_roles WHERE rolname='ai_workbench_runtime')" | grep -qx 0
touch /state/.started
openssl rand -hex 24 | tr -d '\n' > /state/runtime-password
psql -X -v ON_ERROR_STOP=1 -d product_agent -v runtimepassword="$(cat /state/runtime-password)" <<'SQL'
CREATE ROLE ai_workbench_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'runtimepassword';
ALTER ROLE ai_workbench_runtime SET statement_timeout='10s';
GRANT CONNECT ON DATABASE product_agent TO ai_workbench_runtime;
SQL
unset PGPASSWORD
AI_WORKBENCH_SCHEMA_DSN="host=127.0.0.1 port=5433 user=product_agent_owner password=$(cat /secrets/product-agent-owner/product-agent-db-password) dbname=product_agent sslmode=disable" ai-workbench-schema-init --runtime-role ai_workbench_runtime
jq --argjson organizations "$organizations" --rawfile password /state/runtime-password '
 .aiWorkbench={enabled:true,conversationOnly:true,allowedOrganizationIds:$organizations,
 database:(.productAgent.database+{user:"ai_workbench_runtime",password:$password,maxConnections:4})}
' /base/current-application.json > /runtime/current-application.json.tmp
chmod 600 /runtime/current-application.json.tmp
mv /runtime/current-application.json.tmp /runtime/current-application.json
printf '%s\n' "$source_hash" > /state/source-manifest.sha256
sha256sum /runtime/current-application.json | cut -d ' ' -f 1 > /state/runtime-manifest.sha256
printf '%s\n' chat-conversations-v1 > /state/.complete
echo 'Conversation owner installed; model execution remains closed'
