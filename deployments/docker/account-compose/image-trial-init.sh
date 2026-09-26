#!/bin/sh
set -eu

test "${ACCOUNT_IMAGE_AGENT_TRIAL:-}" = ISOLATED_TRIAL_ONLY || { echo 'isolated image trial is not enabled' >&2; exit 1; }
state=/image-trial-state
runtime=/runtime
worker_secrets=/worker-secrets
fixture=/acceptance/manifest.json
image_owner=/secrets/image-owner/image-db-password
image_runtime=/secrets/image-runtime/image-runtime-password
image_worker=/secrets/image-worker/image-worker-password
acquisition_owner=/secrets/acquisition-owner/acquisition-db-password
acquisition_runtime=/secrets/acquisition-runtime/acquisition-runtime-password
minio_password=/secrets/minio/minio-root-password
application_port=${ACCOUNT_APPLICATION_PORT:?}
identity_port=${ACCOUNT_IDENTITY_PORT:?}
commercial_database=${ACCOUNT_COMMERCIAL_DATABASE:-commercial}
case "$application_port:$identity_port" in *[!0-9:]*|'') echo 'invalid isolated trial ports' >&2; exit 1;; esac
case "$commercial_database" in ''|[!a-z]*|*[!a-z0-9_]*) echo 'invalid commercial database name' >&2; exit 1;; esac
case "$commercial_database" in *trial*|*isolated*) ;; *) echo 'isolated trial requires an isolated commercial database' >&2; exit 1;; esac
for path in "$fixture" "$image_owner" "$image_runtime" "$image_worker" "$acquisition_owner" "$acquisition_runtime" "$minio_password" "$runtime/current-application.json" "$runtime/project-id" "$runtime/bootstrap-user-id" "$runtime/membership-read.pat" /secrets/commercial-runtime/commercial-reader-password "$worker_secrets/config.yaml"; do
  if [ "$path" = "$worker_secrets/config.yaml" ] && [ ! -f "$state/.init-complete" ]; then continue; fi
  test -s "$path" || { echo 'isolated image trial prerequisite missing' >&2; exit 1; }
done
project_id=$(tr -d '\r\n' < "$runtime/project-id")
operator_id=$(tr -d '\r\n' < "$runtime/bootstrap-user-id")
org_id=$(jq -er '.organizations[0].id' "$fixture")
case "$project_id:$operator_id:$org_id" in *[!0-9:]*|'') echo 'invalid isolated trial identity' >&2; exit 1;; esac
jq -e --arg issuer "https://localhost:${identity_port}" --arg project "$project_id" --arg operator "$operator_id" \
  '.schemaVersion == 1 and .status == "ready" and .issuerUrl == $issuer and .projectId == $project and .operatorUserId == $operator and (.organizations | length) == 2' "$fixture" >/dev/null
commercial_dsn="postgresql://commercial_runtime:$(tr -d '\r\n' < /secrets/commercial-runtime/commercial-reader-password)@127.0.0.1:5433/${commercial_database}?sslmode=disable"
test "$(psql "$commercial_dsn" -At -v ON_ERROR_STOP=1 -c "SELECT EXISTS(SELECT 1 FROM saas_plans WHERE code='paid_pilot') AND has_table_privilege(current_user,'saas_usage_events','SELECT,INSERT,UPDATE') AND has_table_privilege(current_user,'account_member_token_allocations','SELECT,INSERT,UPDATE')")" = t || { echo 'isolated trial plan or commercial runtime rights unavailable' >&2; exit 1; }
trial_base="https://localhost:${application_port}/image-agent-assets/image-agent-trial"
mc alias set local http://127.0.0.1:9000 issue487_local "$(tr -d '\r\n' < "$minio_password")" >/dev/null
if [ -f "$state/.init-complete" ]; then
  jq -e --arg base "$trial_base" --arg org "$org_id" \
    '.imageAgent.isolatedTrialGeneratedUrls == true and .imageAgent.publicBase == $base and .imageAgent.bucket == "image-agent-trial" and .imageAgent.allowedOrganizationIds == [$org] and .productAcquisitionDatabase.database == "product_acquisition"' "$runtime/current-application.json" >/dev/null
  test -s "$worker_secrets/config.yaml"
  actual_policy=$(mc anonymous get-json local/image-agent-trial | jq -Sc .)
  expected_policy=$(jq -Sc . /etc/image-trial-public-policy.json)
  test "$actual_policy" = "$expected_policy" || { echo 'isolated image trial public policy changed' >&2; exit 1; }
  exit 0
fi
if [ -f "$state/.init-started" ]; then echo 'isolated image trial initialization incomplete; do not auto-repair' >&2; exit 1; fi
umask 077
mkdir -p "$state" "$runtime" "$worker_secrets"
touch "$state/.init-started"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

image_dsn="postgresql://image_agent_owner:$(tr -d '\r\n' < "$image_owner")@127.0.0.1:5433/image_agent?sslmode=disable"
acquisition_dsn="postgresql://acquisition_owner:$(tr -d '\r\n' < "$acquisition_owner")@127.0.0.1:5433/product_acquisition?sslmode=disable"
image_runtime_password=$(tr -d '\r\n' < "$image_runtime")
image_worker_password=$(tr -d '\r\n' < "$image_worker")
acquisition_runtime_password=$(tr -d '\r\n' < "$acquisition_runtime")

cat > "$work/acquisition-owner.json" <<EOF
{"schemaVersion":1,"database":{"host":"127.0.0.1","port":5433,"user":"acquisition_owner","password":"$(tr -d '\r\n' < "$acquisition_owner")","database":"product_acquisition"}}
EOF
product-acquisition-init -config "$work/acquisition-owner.json" -confirm-empty-database product_acquisition

jq --arg password "$image_runtime_password" --arg org "$org_id" --arg base "$trial_base" \
  '.productAcquisitionDatabase = {host:"127.0.0.1",port:5433,user:"source_acquisition_runtime",password:"__ACQUISITION_PASSWORD__",database:"product_acquisition",maxConnections:4} | .imageAgent = {database:{host:"127.0.0.1",port:5433,user:"image_agent_runtime",password:$password,database:"image_agent",maxConnections:4},temporalAddress:"127.0.0.1:7233",temporalNamespace:"default",allowedOrganizationIds:[$org],publicBase:$base,bucket:"image-agent-trial",isolatedTrialGeneratedUrls:true}' \
  "$runtime/current-application.json" > "$work/current.json"
jq --arg password "$acquisition_runtime_password" '.productAcquisitionDatabase.password = $password' "$work/current.json" > "$runtime/current-application.json.tmp"
chmod 600 "$runtime/current-application.json.tmp"
mv "$runtime/current-application.json.tmp" "$runtime/current-application.json"
cat > "$work/image-owner.yaml" <<EOF
database:
  host: 127.0.0.1
  port: 5433
  user: image_agent_owner
  password: "$(tr -d '\r\n' < "$image_owner")"
  database: image_agent
  max_connections: 2
  max_idle_connections: 1
EOF
product-listing-api-schema-migrate -initialize-organization-image-agent -config "$work/image-owner.yaml" -current-application-manifest "$runtime/current-application.json"
product-listing-api-schema-migrate -grant-image-agent-runtime -config "$work/image-owner.yaml" -current-application-manifest "$runtime/current-application.json"
product-listing-api-schema-migrate -grant-image-agent-worker-runtime -config "$work/image-owner.yaml" -current-application-manifest "$runtime/current-application.json"

psql "$image_dsn" -v ON_ERROR_STOP=1 -v "org=$org_id" -v "actor=$operator_id" <<'SQL'
INSERT INTO ai_client_credentials (tenant_id,user_id,client_name,api_key,base_url,model,api_style,timeout_second,enabled,created_at,updated_at)
VALUES (:'org',:'actor','default','controlled-local-only','http://127.0.0.1:18080/v1','controlled-review','','5',true,now(),now()),
       (:'org',:'actor','image_gpt_image_2','controlled-local-only','http://127.0.0.1:18080/v1','controlled-image','','5',true,now(),now());
SQL

mc mb local/image-agent-trial >/dev/null
mc anonymous set-json /etc/image-trial-public-policy.json local/image-agent-trial >/dev/null

cat > "$worker_secrets/config.yaml.tmp" <<EOF
database:
  host: 127.0.0.1
  port: 5433
  user: image_agent_worker_runtime
  password: "$(tr -d '\r\n' < "$image_worker")"
  database: image_agent
  max_connections: 4
commercialDatabase:
  host: 127.0.0.1
  port: 5433
  user: commercial_runtime
  password: "$(tr -d '\r\n' < /secrets/commercial-runtime/commercial-reader-password)"
  database: ${commercial_database}
  max_connections: 4
commercialOwnerDatabase:
  host: 127.0.0.1
  port: 5434
  user: commercial_owner_runtime
  password: "$(tr -d '\r\n' < /secrets/commercial-runtime/commercial-reader-password)"
  database: ${commercial_database}
  max_connections: 4
openai:
  apiKey: controlled-local-only
  model: controlled-review
  baseURL: http://127.0.0.1:18080/v1
  timeout: 5
  clients:
    image_gpt_image_2:
      apiKey: controlled-local-only
      model: controlled-image
      baseURL: http://127.0.0.1:18080/v1
      timeout: 5
listingkit:
  platformAdminUsers: ["${operator_id}"]
  zitadel:
    issuerURL: https://localhost:${identity_port}
    authorizationAPIURL: https://localhost:${identity_port}
    projectID: "${project_id}"
    tenantDirectoryToken: "$(tr -d '\r\n' < "$runtime/membership-read.pat")"
imageagent:
  admission:
    enabled: true
    allowedTenantIDs: ["${org_id}"]
  artifactStore:
    enabled: true
    provider: s3
    publicBase: ${trial_base}
    isolatedTrialGeneratedURLs: true
    s3:
      bucket: image-agent-trial
      region: us-east-1
      endpoint: http://127.0.0.1:9000
      accessKeyID: issue487_local
      secretAccessKey: "$(tr -d '\r\n' < "$minio_password")"
      usePathStyle: true
      artifactMode: aws
EOF
chmod 600 "$worker_secrets/config.yaml.tmp"
mv "$worker_secrets/config.yaml.tmp" "$worker_secrets/config.yaml"
mv "$state/.init-started" "$state/.init-complete"
