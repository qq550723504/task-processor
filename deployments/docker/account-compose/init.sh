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
store_owner_secret=/secrets/store-owner
store_runtime_secret=/secrets/store-runtime
issue36_trial_runtime_secret=/secrets/issue36-trial-runtime
image_db_owner_secret=/secrets/image-owner
product_agent_db_owner_secret=/secrets/product-agent-owner
image_audit_reader_secret=/secrets/image-audit-reader
product_audit_reader_secret=/secrets/product-audit-reader
runtime=/runtime
frontend=/frontend
identity_port=${ACCOUNT_IDENTITY_PORT:?ACCOUNT_IDENTITY_PORT is required}
application_port=${ACCOUNT_APPLICATION_PORT:?ACCOUNT_APPLICATION_PORT is required}
commercial_database=${ACCOUNT_COMMERCIAL_DATABASE:-commercial}
case "$commercial_database" in
  ''|[!a-z]*|*[!a-z0-9_]*) echo 'invalid commercial database name' >&2; exit 1 ;;
esac
if [ "${#commercial_database}" -gt 63 ]; then echo 'commercial database name is too long' >&2; exit 1; fi
if [ -n "${ACCOUNT_ISOLATED_TRIAL_CATALOG:-}" ]; then
  echo 'subscription trial catalog is retired; resource prices must be configured separately' >&2
  exit 1
fi
case "${ACCOUNT_ISSUE36_LOCAL_TRIAL:-}" in
  ''|ISOLATED_TRIAL_ONLY) ;;
  *) echo 'invalid #36 local trial opt-in' >&2; exit 1 ;;
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

install_payment_directory_token() {
  if [ ! -s "$runtime/membership-read.pat" ]; then
    echo 'payment directory read credential is unavailable' >&2
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

initialize_store_center() {
  cat > "$work/store-owner-schema.json" <<EOF
{"host":"127.0.0.1","port":5433,"user":"store_center_owner","password":"$(tr -d '\r\n' < "$store_owner_secret/store-owner-password")","database":"store_center","maxConnections":2,"maxIdleConnections":1}
EOF
  store-center-schema-init -config "$work/store-owner-schema.json"
}

initialize_issue36_trial() {
  if [ "${ACCOUNT_ISSUE36_LOCAL_TRIAL:-}" != ISOLATED_TRIAL_ONLY ]; then
    jq -e 'has("localTrial") | not' "$runtime/current-application.json" >/dev/null || { echo 'retained #36 trial requires its original overlay' >&2; exit 1; }
    return
  fi
  test -s "$issue36_trial_runtime_secret/password" || { echo '#36 trial runtime role credential unavailable' >&2; exit 1; }
  if [ -f "$state/.init-complete" ]; then
    jq -e '.localTrial.enabled == true and .localTrial.database.database == "store_center" and .localTrial.database.user == "issue36_trial_runtime" and .localTrial.database.port == 5433' "$runtime/current-application.json" >/dev/null || { echo '#36 trial topology changed; use its original checkout' >&2; exit 1; }
    test -s "$runtime/issue36-sample.json" || { echo '#36 trial sample record unavailable' >&2; exit 1; }
    return
  fi
  owner_dsn="postgresql://store_center_owner:$(tr -d '\r\n' < "$store_owner_secret/store-owner-password")@127.0.0.1:5433/store_center?sslmode=disable"
  issue36-local-trial-init -mode schema -config "$work/store-owner-schema.json"
  psql "$owner_dsn" -v ON_ERROR_STOP=1 -f /etc/issue36-listing-schema.sql
  psql "$owner_dsn" -v ON_ERROR_STOP=1 <<SQL
GRANT CONNECT ON DATABASE store_center TO issue36_trial_runtime;
GRANT USAGE ON SCHEMA public TO issue36_trial_runtime;
GRANT SELECT ON TABLE public.workbench_stores TO issue36_trial_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE
  public.product_snapshot_versions, public.product_snapshot_heads,
  public.product_source_publications, public.product_source_publication_receipts,
  public.product_title_proposals, public.product_title_operations,
  public.product_approved_assets, public.product_approval_receipts,
  public.product_approved_inventory_heads, public.product_approved_inventory_version_heads,
  public.listing_shein_records, public.listing_shein_record_operations
  TO issue36_trial_runtime;
SQL
  jq --rawfile password "$issue36_trial_runtime_secret/password" \
    '.localTrial = {enabled:true,database:{host:"127.0.0.1",port:5433,user:"issue36_trial_runtime",password:($password|rtrimstr("\n")),database:"store_center",maxConnections:4}}' \
    "$runtime/current-application.json" > "$runtime/current-application.json.tmp"
  chmod 600 "$runtime/current-application.json.tmp"
  mv "$runtime/current-application.json.tmp" "$runtime/current-application.json"
  issue36-local-trial-init -mode seed -config "$work/store-owner-schema.json" \
    -organization-id "$(tr -d '\r\n' < "$runtime/signup-org-id")" \
    -actor-id "$(tr -d '\r\n' < "$runtime/bootstrap-user-id")" \
    -sample-output "$runtime/issue36-sample.json"
}

initialize_audit_ledgers() {
  for namespace in image product; do
    case "$namespace" in
      image) owner_secret="$image_db_owner_secret/image-db-password" ;;
      product) owner_secret="$product_agent_db_owner_secret/product-agent-db-password" ;;
    esac
    owner_dsn="postgresql://${namespace}_agent_owner:$(tr -d '\r\n' < "$owner_secret")@127.0.0.1:5433/${namespace}_agent?sslmode=disable"
    printf '%s\n' "$owner_dsn" > "$work/${namespace}-ledger-owner-dsn"
    chmod 600 "$work/${namespace}-ledger-owner-dsn"
    account-audit-ledger-schema-init -namespace "$namespace" -dsn-file "$work/${namespace}-ledger-owner-dsn"
    psql "$owner_dsn" -v ON_ERROR_STOP=1 <<SQL
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE ${namespace}_agent TO account_audit_${namespace}_reader;
GRANT USAGE ON SCHEMA public TO account_audit_${namespace}_reader;
GRANT SELECT ON TABLE public.ai_invocations TO account_audit_${namespace}_reader;
SQL
  done
}

initialize_knowledge() {
 if [ "${ACCOUNT_KNOWLEDGE_ENABLED:-}" != 1 ]; then
  jq -e '.knowledge == null' "$runtime/current-application.json" >/dev/null || { echo 'knowledge topology changed; use its original overlay' >&2; exit 1; }
  return
 fi
 cat > "$work/knowledge-owner.json" <<EOF
{"host":"127.0.0.1","port":5433,"user":"knowledge_owner","password":"$(tr -d '\r\n' < /secrets/knowledge-owner/owner-password)","database":"knowledge","maxConnections":2,"maxIdleConnections":1}
EOF
 knowledge-schema-init -config "$work/knowledge-owner.json"
 if jq -e '.knowledge != null' "$runtime/current-application.json" >/dev/null; then
  jq -e '.knowledge.enabled == true and .knowledge.database.database == "knowledge" and .knowledge.database.user == "knowledge_runtime" and .knowledge.database.port == 5433 and .knowledge.parserEndpoint == "http://knowledge-parser:9998"' "$runtime/current-application.json" >/dev/null || { echo 'knowledge topology changed; use a new project' >&2; exit 1; }
  return
 fi
 if [ -f "$state/.init-complete" ]; then echo 'knowledge requires first initialization in a new empty project' >&2; exit 1; fi
 jq --rawfile password /secrets/knowledge-runtime/runtime-password --rawfile access /secrets/knowledge-storage/access-key --rawfile secret /secrets/knowledge-storage/secret-key \
  '.knowledge = {enabled:true,database:{host:"127.0.0.1",port:5433,user:"knowledge_runtime",password:($password|rtrimstr("\n")),database:"knowledge",maxConnections:4},storage:{endpoint:"http://knowledge-objects:9000",region:"us-east-1",bucket:"knowledge",accessKeyId:($access|rtrimstr("\n")),secretAccessKey:($secret|rtrimstr("\n")),mode:"aws"},parserEndpoint:"http://knowledge-parser:9998"}' \
  "$runtime/current-application.json" > "$runtime/current-application.json.tmp"
 chmod 600 "$runtime/current-application.json.tmp"
 mv "$runtime/current-application.json.tmp" "$runtime/current-application.json"
}

umask 077
if [ -f "$state/.init-complete" ]; then
  for path in "$image_audit_reader_secret/password" "$product_audit_reader_secret/password" "$product_agent_db_owner_secret/product-agent-db-password"; do
    test -s "$path" || { echo 'Account Audit source topology requires a new empty project' >&2; exit 1; }
  done
  jq -e --arg database "$commercial_database" '
    .sourceAccountDatabase.port == 5433 and .sourceAccountDatabase.database == "source_accounts" and
    (has("commercialDatabase") | not) and
    .storeCenter.enabled == true and .storeCenter.database.database == "store_center" and .storeCenter.database.user == "store_center_runtime" and .storeCenter.database.port == 5433 and (.storeCenter | has("quotaDatabase") | not) and
    .commercialOwnerDatabase.port == 5433 and .commercialOwnerDatabase.database == $database and
    .moneyOwnerDatabase.port == 5433 and .moneyOwnerDatabase.database == "referrals" and .moneyOwnerDatabase.user == "money_owner_runtime" and
    .referrals.referralDatabase.port == 5433 and .referrals.referralDatabase.database == "referrals" and
    .membership.database.port == 5433 and .membership.database.database == "membership" and
    .accountAuditUsage.image.database == "image_agent" and .accountAuditUsage.image.user == "account_audit_image_reader" and
    .accountAuditUsage.product.database == "product_agent" and .accountAuditUsage.product.user == "account_audit_product_reader"
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
GRANT SELECT, INSERT, UPDATE ON TABLE public.account_user_preferences TO source_account_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.subject_verification_applications, public.subject_verification_messages, public.personal_verification_applications TO source_account_runtime;
GRANT SELECT, INSERT ON TABLE public.account_business_profile_audit_events TO source_account_runtime;
GRANT USAGE, SELECT ON SEQUENCE public.account_business_profile_audit_events_id_seq TO source_account_runtime;
SQL
  install_listingkit_authorization
  install_payment_directory_token
  psql "postgresql://referral_owner:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5433/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -f "$terraform_source/referral-economics-schema.sql"
  psql "postgresql://referral_owner:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5433/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -v runtime_roles_ready=true -f "$terraform_source/referral-grants.sql"
  migrate_commercial_owner_schema
  initialize_store_center
  initialize_issue36_trial
  initialize_knowledge
  membership_dsn="postgresql://membership_owner:$(tr -d '\r\n' < "$membership_db_owner_secret/membership-db-password")@127.0.0.1:5433/membership?sslmode=disable"
  printf '%s\n' "$membership_dsn" > "$work/membership-owner-dsn"
  chmod 600 "$work/membership-owner-dsn"
  if [ -s "$runtime/enterprise-role-slots.json" ]; then
  organization-membership-schema-init -dsn-file "$work/membership-owner-dsn" -role-slots-file "$runtime/enterprise-role-slots.json"
else
  organization-membership-schema-init -dsn-file "$work/membership-owner-dsn"
fi
  psql "$membership_dsn" -v ON_ERROR_STOP=1 <<SQL
GRANT CONNECT ON DATABASE membership TO organization_membership_runtime;
GRANT USAGE ON SCHEMA public TO organization_membership_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.organization_member_operations, public.organization_member_invitations TO organization_membership_runtime;
GRANT SELECT, INSERT ON TABLE public.organization_member_audit_events TO organization_membership_runtime;
GRANT SELECT ON TABLE public.organization_role_slots TO organization_membership_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.organization_roles TO organization_membership_runtime;
GRANT SELECT, INSERT ON TABLE public.organization_role_mutations TO organization_membership_runtime;
-- A retained project has no newly created reader credential or native slots.
DO \$grant\$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='organization_role_policy_reader') THEN
  GRANT CONNECT ON DATABASE membership TO organization_role_policy_reader;
  GRANT USAGE ON SCHEMA public TO organization_role_policy_reader;
  GRANT SELECT ON TABLE public.organization_role_slots,public.organization_roles TO organization_role_policy_reader;
 END IF;
END \$grant\$;
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
  "commercialOwnerDatabase": {"host": "127.0.0.1", "port": 5433, "user": "commercial_owner_runtime", "password": "$(tr -d '\r\n' < "$commercial_runtime_secret/commercial-owner-password")", "database": "${commercial_database}", "maxConnections": 2},
  "storeCenter": {"enabled": true, "database": {"host":"127.0.0.1","port":5433,"user":"store_center_runtime","password":"$(tr -d '\r\n' < "$store_runtime_secret/store-runtime-password")","database":"store_center","maxConnections":4}},
  "moneyOwnerDatabase": {"host": "127.0.0.1", "port": 5433, "user": "money_owner_runtime", "password": "$(tr -d '\r\n' < "$commercial_runtime_secret/money-owner-password")", "database": "referrals", "maxConnections": 4},
  "accountAuditUsage": {
    "image": {"host":"127.0.0.1","port":5433,"user":"account_audit_image_reader","password":"$(tr -d '\r\n' < "$image_audit_reader_secret/password")","database":"image_agent","maxConnections":2},
    "product": {"host":"127.0.0.1","port":5433,"user":"account_audit_product_reader","password":"$(tr -d '\r\n' < "$product_audit_reader_secret/password")","database":"product_agent","maxConnections":2}
  },
  "membership": {
    "invitationMail": {"host": "127.0.0.1", "port": 1025, "from": "invitations@localhost", "publicOrigin": "${public_app}", "localPlaintext": true},
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
install_payment_directory_token

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
GRANT SELECT, INSERT, UPDATE ON TABLE public.account_user_preferences TO source_account_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.subject_verification_applications, public.subject_verification_messages, public.personal_verification_applications TO source_account_runtime;
GRANT SELECT, INSERT ON TABLE public.account_business_profile_audit_events TO source_account_runtime;
GRANT USAGE, SELECT ON SEQUENCE public.account_business_profile_audit_events_id_seq TO source_account_runtime;
SQL

printf 'postgresql://referral_owner:%s@127.0.0.1:5433/referrals?sslmode=disable\n' "$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")" > "$work/referral-owner-dsn"
chmod 600 "$work/referral-owner-dsn"
referral-schema-init -dsn-file "$work/referral-owner-dsn"
psql "postgresql://referral_owner:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5433/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -f "$terraform_source/referral-economics-schema.sql"
psql "postgresql://referral_owner:$(tr -d '\r\n' < "$referral_db_owner_secret/referral-db-password")@127.0.0.1:5433/referrals?sslmode=disable" -v ON_ERROR_STOP=1 -v runtime_roles_ready=true -f "$terraform_source/referral-grants.sql"
migrate_commercial_owner_schema
initialize_store_center
initialize_issue36_trial
initialize_audit_ledgers
initialize_knowledge

printf 'postgresql://membership_owner:%s@127.0.0.1:5433/membership?sslmode=disable\n' "$(tr -d '\r\n' < "$membership_db_owner_secret/membership-db-password")" > "$work/membership-owner-dsn"
chmod 600 "$work/membership-owner-dsn"
if [ -s "$runtime/enterprise-role-slots.json" ]; then
  organization-membership-schema-init -dsn-file "$work/membership-owner-dsn" -role-slots-file "$runtime/enterprise-role-slots.json"
else
  organization-membership-schema-init -dsn-file "$work/membership-owner-dsn"
fi
psql "postgresql://membership_owner:$(tr -d '\r\n' < "$membership_db_owner_secret/membership-db-password")@127.0.0.1:5433/membership?sslmode=disable" -v ON_ERROR_STOP=1 <<SQL
REVOKE CREATE, TEMPORARY ON DATABASE membership FROM PUBLIC;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE membership TO organization_membership_runtime;
GRANT USAGE ON SCHEMA public TO organization_membership_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.organization_member_operations, public.organization_member_invitations TO organization_membership_runtime;
GRANT SELECT, INSERT ON TABLE public.organization_member_audit_events TO organization_membership_runtime;
GRANT SELECT ON TABLE public.organization_role_slots TO organization_membership_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE public.organization_roles TO organization_membership_runtime;
GRANT SELECT, INSERT ON TABLE public.organization_role_mutations TO organization_membership_runtime;
-- A retained project has no newly created reader credential or native slots.
DO \$grant\$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='organization_role_policy_reader') THEN
  GRANT CONNECT ON DATABASE membership TO organization_role_policy_reader;
  GRANT USAGE ON SCHEMA public TO organization_role_policy_reader;
  GRANT SELECT ON TABLE public.organization_role_slots,public.organization_roles TO organization_role_policy_reader;
 END IF;
END \$grant\$;
SQL

if [ "${ACCOUNT_DATA_SERVICES_ENABLED:-}" = 1 ]; then data-services-init; fi
mv "$state/.init-started" "$state/.init-complete"
