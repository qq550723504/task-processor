#!/bin/sh
# Install only the new E owner after the retained unified native composition.
set -eu
umask 077
test -s /base/current-application.json
source_hash=$(sha256sum /base/current-application.json | cut -d ' ' -f 1)
if test -f /state/.complete; then
 test "$(cat /state/.complete)" = ecoservices-qualification-v1
 test "$source_hash" = "$(cat /state/source-manifest.sha256)" || { echo 'Base manifest changed; inspect explicit ecosystem reassembly, retained facts untouched' >&2; exit 1; }
 test -s /runtime/current-application.json
 test "$(sha256sum /runtime/current-application.json | cut -d ' ' -f 1)" = "$(cat /state/runtime-manifest.sha256)"
 test -s /object-root/root-password
 test -s /object-runtime/access-key
 test -s /object-runtime/secret-key
 jq -e '.ecoservices.enabled == true and .ecoservices.nonPaymentOnly == true' /runtime/current-application.json >/dev/null
 echo 'Ecoservices qualification facts and original private configuration retained'
 exit 0
fi
test ! -f /state/.started || { echo 'Inspect interrupted empty ecosystem installation before retry; facts retained' >&2; exit 1; }
jq -e '.productCollections == true and .dataServices != null and .ecoservices == null' /base/current-application.json >/dev/null
export PGHOST=127.0.0.1 PGPORT=5433 PGUSER=business_cluster_admin
export PGPASSWORD="$(cat /secrets/admin/admin-password)"
psql -X -v ON_ERROR_STOP=1 -d postgres -At -c "SELECT count(*) FROM pg_database WHERE datname='ecoservices'" | grep -qx 0
touch /state/.started
openssl rand -hex 24 | tr -d '\n' > /state/owner-password
openssl rand -hex 24 | tr -d '\n' > /state/runtime-password
psql -X -v ON_ERROR_STOP=1 -d postgres -v ownerpassword="$(cat /state/owner-password)" -v runtimepassword="$(cat /state/runtime-password)" <<'SQL'
CREATE ROLE ecoservices_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'ownerpassword';
CREATE ROLE ecoservices_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'runtimepassword';
ALTER ROLE ecoservices_runtime SET statement_timeout='10s';
CREATE DATABASE ecoservices OWNER ecoservices_owner;
REVOKE ALL ON DATABASE ecoservices FROM PUBLIC;
GRANT CONNECT ON DATABASE ecoservices TO ecoservices_runtime;
SQL
psql -X -v ON_ERROR_STOP=1 -d ecoservices -c 'REVOKE CREATE ON SCHEMA public FROM PUBLIC;'
ECOSERVICES_SCHEMA_DSN="host=127.0.0.1 port=5433 user=ecoservices_owner password=$(cat /state/owner-password) dbname=ecoservices sslmode=disable" ecoservices-schema-init
openssl rand -hex 24 | tr -d '\n' > /object-root/root-password
openssl rand -hex 16 | tr -d '\n' > /object-runtime/access-key
openssl rand -hex 24 | tr -d '\n' > /object-runtime/secret-key
jq --rawfile password /state/runtime-password --rawfile ak /object-runtime/access-key --rawfile sk /object-runtime/secret-key '
 .ecoservices={enabled:true,nonPaymentOnly:true,
 database:{host:"127.0.0.1",port:5433,user:"ecoservices_runtime",password:$password,database:"ecoservices",maxConnections:4},
 storage:{mode:"aws",region:"us-east-1",bucket:"ecoservices",endpoint:"http://127.0.0.1:9101",accessKeyId:$ak,secretAccessKey:$sk}}
' /base/current-application.json > /runtime/current-application.json.tmp
chmod 600 /runtime/current-application.json.tmp /object-root/root-password /object-runtime/*
mv /runtime/current-application.json.tmp /runtime/current-application.json
printf '%s\n' "$source_hash" > /state/source-manifest.sha256
sha256sum /runtime/current-application.json | cut -d ' ' -f 1 > /state/runtime-manifest.sha256
printf '%s\n' ecoservices-qualification-v1 > /state/.complete
echo 'Ecoservices qualification owner installed; no merchant, service, order or payment created'
