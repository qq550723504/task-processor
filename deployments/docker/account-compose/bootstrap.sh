#!/bin/sh
set -eu
# Keep this entrypoint materialized as LF on existing Windows worktrees.

state=/state
trusted_ca=/trusted-ca
traefik_tls=/traefik-tls
business_db_admin_secret=/business-db-admin-secret
identity_db_secret=/identity-db-secret
source_db_owner_secret=/source-db-owner-secret
source_runtime_secret=/source-runtime-secret
commercial_db_owner_secret=/commercial-db-owner-secret
commercial_runtime_secret=/commercial-runtime-secret
referral_db_owner_secret=/referral-db-owner-secret
referral_runtime_secret=/referral-runtime-secret
membership_db_owner_secret=/membership-db-owner-secret
membership_runtime_secret=/membership-runtime-secret
role_policy_reader_secret=/role-policy-reader-secret
zitadel_api_secrets=/zitadel-api-secrets
tofu_inputs=/tofu-inputs
frontend_secrets=/frontend-secrets
image_db_owner_secret=/image-db-owner-secret
product_agent_db_owner_secret=/product-agent-db-owner-secret
image_audit_reader_secret=/image-audit-reader-secret
product_audit_reader_secret=/product-audit-reader-secret
image_runtime_secret=/image-runtime-secret
image_worker_secret=/image-worker-secret
acquisition_db_owner_secret=/acquisition-db-owner-secret
acquisition_runtime_secret=/acquisition-runtime-secret
image_minio_secret=/image-minio-secret
store_owner_secret=/store-owner-secret
store_runtime_secret=/store-runtime-secret
issue36_trial_runtime_secret=/issue36-trial-runtime-secret
marker="$state/.bootstrap-complete"
case "${ACCOUNT_DATA_SERVICES_ENABLED:-}" in ''|1) ;; *) echo 'invalid data services opt-in' >&2; exit 1 ;; esac
case "${ACCOUNT_KNOWLEDGE_ENABLED:-}" in ''|1) ;; *) echo 'invalid knowledge opt-in' >&2; exit 1 ;; esac
case "${ACCOUNT_IMAGE_AGENT_TRIAL:-}" in
  ''|ISOLATED_TRIAL_ONLY) ;;
  *) echo 'invalid image trial confirmation' >&2; exit 1 ;;
esac
case "${ACCOUNT_ISSUE36_LOCAL_TRIAL:-}" in
  ''|ISOLATED_TRIAL_ONLY) ;;
  *) echo 'invalid #36 local trial opt-in' >&2; exit 1 ;;
esac

if [ -f "$marker" ]; then
  if [ "${ACCOUNT_DATA_SERVICES_ENABLED:-}" = 1 ]; then
    test -f "$state/.data-services-enabled" && test -s /data-services-runtime-secret/password || { echo 'data services require a new empty project with their original overlay' >&2; exit 1; }
  else
    test ! -f "$state/.data-services-enabled" || { echo 'retained data services require their original overlay' >&2; exit 1; }
  fi
  if [ "${ACCOUNT_ISSUE36_LOCAL_TRIAL:-}" = ISOLATED_TRIAL_ONLY ]; then
    test -f "$state/.issue36-trial-enabled" && test -s "$issue36_trial_runtime_secret/password" || { echo '#36 local trial requires a new empty project with its original overlay' >&2; exit 1; }
  else
    test ! -f "$state/.issue36-trial-enabled" || { echo 'retained #36 local trial requires its original overlay' >&2; exit 1; }
  fi
  if [ "${ACCOUNT_KNOWLEDGE_ENABLED:-}" = 1 ]; then
    for path in /knowledge-owner-secret/owner-password /knowledge-runtime-secret/runtime-password /knowledge-storage-secret/root-password /knowledge-storage-secret/access-key /knowledge-storage-secret/secret-key; do test -s "$path" || { echo 'Knowledge requires a new empty project' >&2; exit 1; }; done
  fi
  for path in "$store_owner_secret/store-owner-password" "$store_runtime_secret/store-runtime-password"; do test -s "$path" || { echo 'Store Center requires a new empty project' >&2; exit 1; }; done
  test -s "$business_db_admin_secret/admin-password" || { echo 'retained multi-instance project: use its original checkout; create a new project for the consolidated database' >&2; exit 1; }
  for path in \
    "$trusted_ca/root-ca.pem" "$traefik_tls/localhost.crt" "$traefik_tls/localhost.key" \
    "$identity_db_secret/identity-db-password" "$source_db_owner_secret/source-db-password" \
    "$source_runtime_secret/source-runtime-password" "$commercial_db_owner_secret/commercial-db-password" \
    "$commercial_runtime_secret/commercial-reader-password" "$referral_db_owner_secret/referral-db-password" \
    "$referral_runtime_secret/referral-runtime-password" "$membership_db_owner_secret/membership-db-password" \
    "$membership_runtime_secret/membership-runtime-password" "$zitadel_api_secrets/zitadel-masterkey" \
    "$zitadel_api_secrets/runtime-config.yaml" "$tofu_inputs/operator-password" "$tofu_inputs/viewer-password" \
    "$tofu_inputs/insufficient-password" "$frontend_secrets/auth-secret"; do
    test -f "$path"
  done
  test -s "$commercial_runtime_secret/commercial-owner-password"
  test -s "$commercial_runtime_secret/money-owner-password" || { echo 'wallet top-up owner requires a new empty project' >&2; exit 1; }
  for path in "$image_db_owner_secret/image-db-password" "$image_runtime_secret/image-runtime-password" "$image_worker_secret/image-worker-password" \
      "$acquisition_db_owner_secret/acquisition-db-password" "$acquisition_runtime_secret/acquisition-runtime-password"; do test -f "$path"; done
  for path in "$product_agent_db_owner_secret/product-agent-db-password" "$image_audit_reader_secret/password" "$product_audit_reader_secret/password"; do
    test -s "$path" || { echo 'Account Audit sources require a new empty project' >&2; exit 1; }
  done
  if [ "${ACCOUNT_IMAGE_AGENT_TRIAL:-}" = ISOLATED_TRIAL_ONLY ]; then test -s "$image_minio_secret/minio-root-password"; fi
  exit 0
fi

umask 077
mkdir -p "$business_db_admin_secret" "$state" "$trusted_ca" "$traefik_tls" "$identity_db_secret" "$source_db_owner_secret" "$source_runtime_secret" \
  "$commercial_db_owner_secret" "$commercial_runtime_secret" "$referral_db_owner_secret" "$referral_runtime_secret" \
  "$membership_db_owner_secret" "$membership_runtime_secret" "$zitadel_api_secrets" "$tofu_inputs" "$frontend_secrets"

write_random() {
  openssl rand -hex "$1" | tr -d '\n' > "$2.tmp"
  chmod 600 "$2.tmp"
  mv "$2.tmp" "$2"
}

if [ "${ACCOUNT_KNOWLEDGE_ENABLED:-}" = 1 ]; then
 mkdir -p /knowledge-owner-secret /knowledge-runtime-secret /knowledge-storage-secret
 write_random 24 /knowledge-owner-secret/owner-password
 write_random 24 /knowledge-runtime-secret/runtime-password
 write_random 24 /knowledge-storage-secret/root-password
 write_random 12 /knowledge-storage-secret/access-key
 write_random 24 /knowledge-storage-secret/secret-key
 chown 70:70 /knowledge-owner-secret/owner-password /knowledge-runtime-secret/runtime-password
fi

mkdir -p "$store_owner_secret" "$store_runtime_secret"
write_random 24 "$store_owner_secret/store-owner-password"
write_random 24 "$store_runtime_secret/store-runtime-password"
chown 70:70 "$store_owner_secret/store-owner-password" "$store_runtime_secret/store-runtime-password"
if [ "${ACCOUNT_ISSUE36_LOCAL_TRIAL:-}" = ISOLATED_TRIAL_ONLY ]; then
  mkdir -p "$issue36_trial_runtime_secret"
  write_random 24 "$issue36_trial_runtime_secret/password"
  chown 70:70 "$issue36_trial_runtime_secret/password"
  touch "$state/.issue36-trial-enabled"
  chmod 600 "$state/.issue36-trial-enabled"
fi
write_random 24 "$business_db_admin_secret/admin-password"
write_random 16 "$zitadel_api_secrets/zitadel-masterkey"
write_random 24 "$identity_db_secret/identity-db-password"
write_random 24 "$source_db_owner_secret/source-db-password"
write_random 24 "$source_runtime_secret/source-runtime-password"
write_random 24 "$commercial_db_owner_secret/commercial-db-password"
write_random 24 "$commercial_runtime_secret/commercial-reader-password"
write_random 24 "$commercial_runtime_secret/commercial-owner-password"
write_random 24 "$commercial_runtime_secret/money-owner-password"
write_random 24 "$referral_db_owner_secret/referral-db-password"
write_random 24 "$referral_runtime_secret/referral-runtime-password"
write_random 24 "$membership_db_owner_secret/membership-db-password"
write_random 24 "$membership_runtime_secret/membership-runtime-password"
mkdir -p "$role_policy_reader_secret"
write_random 24 "$role_policy_reader_secret/password"
chown 70:70 "$role_policy_reader_secret/password"
write_random 32 "$frontend_secrets/auth-secret"
if [ "${ACCOUNT_DATA_SERVICES_ENABLED:-}" = 1 ]; then
  test -z "${ACCOUNT_IMAGE_AGENT_TRIAL:-}" && test -z "${ACCOUNT_ISSUE36_LOCAL_TRIAL:-}" || { echo 'data services profile excludes isolated legacy trial profiles' >&2; exit 1; }
  mkdir -p /data-services-runtime-secret
  write_random 24 /data-services-runtime-secret/password
  touch "$state/.data-services-enabled"
  chmod 600 "$state/.data-services-enabled"
fi
mkdir -p "$image_db_owner_secret" "$image_runtime_secret" "$image_worker_secret" "$acquisition_db_owner_secret" "$acquisition_runtime_secret" "$image_minio_secret"
mkdir -p "$product_agent_db_owner_secret" "$image_audit_reader_secret" "$product_audit_reader_secret"
write_random 24 "$image_db_owner_secret/image-db-password"
write_random 24 "$product_agent_db_owner_secret/product-agent-db-password"
write_random 24 "$image_audit_reader_secret/password"
write_random 24 "$product_audit_reader_secret/password"
write_random 24 "$image_runtime_secret/image-runtime-password"
write_random 24 "$image_worker_secret/image-worker-password"
write_random 24 "$acquisition_db_owner_secret/acquisition-db-password"
write_random 24 "$acquisition_runtime_secret/acquisition-runtime-password"
# postgres:17.2-alpine runs its init hooks as UID/GID 70. Keep the files 0600;
# root schema installers can also read them, serving containers never mount them.
chown 70:70 "$source_db_owner_secret/source-db-password" "$source_runtime_secret/source-runtime-password" \
  "$commercial_runtime_secret/money-owner-password" \
  "$commercial_db_owner_secret/commercial-db-password" "$commercial_runtime_secret/commercial-reader-password" \
  "$commercial_runtime_secret/commercial-owner-password" "$referral_db_owner_secret/referral-db-password" \
  "$referral_runtime_secret/referral-runtime-password" "$membership_db_owner_secret/membership-db-password" \
  "$membership_runtime_secret/membership-runtime-password" "$image_db_owner_secret/image-db-password" \
  "$product_agent_db_owner_secret/product-agent-db-password" "$image_audit_reader_secret/password" "$product_audit_reader_secret/password" \
  "$image_runtime_secret/image-runtime-password" "$image_worker_secret/image-worker-password" \
  "$acquisition_db_owner_secret/acquisition-db-password" "$acquisition_runtime_secret/acquisition-runtime-password"
if [ "${ACCOUNT_IMAGE_AGENT_TRIAL:-}" = ISOLATED_TRIAL_ONLY ]; then
  write_random 24 "$image_minio_secret/minio-root-password"
fi
{ printf 'Local!'; openssl rand -hex 18; } > "$tofu_inputs/operator-password.tmp"
chmod 600 "$tofu_inputs/operator-password.tmp"
mv "$tofu_inputs/operator-password.tmp" "$tofu_inputs/operator-password"
{ printf 'Local!'; openssl rand -hex 18; } > "$tofu_inputs/viewer-password.tmp"
chmod 600 "$tofu_inputs/viewer-password.tmp"
mv "$tofu_inputs/viewer-password.tmp" "$tofu_inputs/viewer-password"
{ printf 'Local!'; openssl rand -hex 18; } > "$tofu_inputs/insufficient-password.tmp"
chmod 600 "$tofu_inputs/insufficient-password.tmp"
mv "$tofu_inputs/insufficient-password.tmp" "$tofu_inputs/insufficient-password"

identity_db_password=$(cat "$identity_db_secret/identity-db-password")
cat > "$zitadel_api_secrets/runtime-config.yaml.tmp" <<EOF
Database:
  postgres:
    DSN: "postgresql://postgres:${identity_db_password}@127.0.0.1:5432/zitadel?sslmode=disable"
EOF
chmod 600 "$zitadel_api_secrets/runtime-config.yaml.tmp"
mv "$zitadel_api_secrets/runtime-config.yaml.tmp" "$zitadel_api_secrets/runtime-config.yaml"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cat > "$work/openssl.cnf" <<'EOF'
[req]
distinguished_name = distinguished_name
prompt = no
req_extensions = req_extensions
[distinguished_name]
CN = localhost
[req_extensions]
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF
openssl req -x509 -newkey rsa:2048 -nodes -days 365 -sha256 -subj '/CN=Task Processor Local Account Center CA' -keyout "$work/root-ca.key" -out "$trusted_ca/root-ca.pem"
openssl req -newkey rsa:2048 -nodes -sha256 -config "$work/openssl.cnf" -keyout "$traefik_tls/localhost.key" -out "$work/localhost.csr"
openssl x509 -req -days 365 -sha256 -CA "$trusted_ca/root-ca.pem" -CAkey "$work/root-ca.key" -CAcreateserial -in "$work/localhost.csr" -out "$traefik_tls/localhost.crt" -extensions req_extensions -extfile "$work/openssl.cnf"
chmod 600 "$trusted_ca/root-ca.pem" "$traefik_tls/localhost.key" "$traefik_tls/localhost.crt"
touch "$marker"
chmod 600 "$marker"
