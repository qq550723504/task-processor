#!/bin/sh
set -eu
# Keep this entrypoint materialized as LF on existing Windows worktrees.

state=/state
trusted_ca=/trusted-ca
traefik_tls=/traefik-tls
identity_db_secret=/identity-db-secret
source_db_owner_secret=/source-db-owner-secret
source_runtime_secret=/source-runtime-secret
commercial_db_owner_secret=/commercial-db-owner-secret
commercial_runtime_secret=/commercial-runtime-secret
referral_db_owner_secret=/referral-db-owner-secret
referral_runtime_secret=/referral-runtime-secret
membership_db_owner_secret=/membership-db-owner-secret
membership_runtime_secret=/membership-runtime-secret
zitadel_api_secrets=/zitadel-api-secrets
tofu_inputs=/tofu-inputs
frontend_secrets=/frontend-secrets
marker="$state/.bootstrap-complete"

if [ -f "$marker" ]; then
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
  exit 0
fi

umask 077
mkdir -p "$state" "$trusted_ca" "$traefik_tls" "$identity_db_secret" "$source_db_owner_secret" "$source_runtime_secret" \
  "$commercial_db_owner_secret" "$commercial_runtime_secret" "$referral_db_owner_secret" "$referral_runtime_secret" \
  "$membership_db_owner_secret" "$membership_runtime_secret" "$zitadel_api_secrets" "$tofu_inputs" "$frontend_secrets"

write_random() {
  openssl rand -hex "$1" | tr -d '\n' > "$2.tmp"
  chmod 600 "$2.tmp"
  mv "$2.tmp" "$2"
}

write_random 16 "$zitadel_api_secrets/zitadel-masterkey"
write_random 24 "$identity_db_secret/identity-db-password"
write_random 24 "$source_db_owner_secret/source-db-password"
write_random 24 "$source_runtime_secret/source-runtime-password"
write_random 24 "$commercial_db_owner_secret/commercial-db-password"
write_random 24 "$commercial_runtime_secret/commercial-reader-password"
write_random 24 "$referral_db_owner_secret/referral-db-password"
write_random 24 "$referral_runtime_secret/referral-runtime-password"
write_random 24 "$membership_db_owner_secret/membership-db-password"
write_random 24 "$membership_runtime_secret/membership-runtime-password"
write_random 32 "$frontend_secrets/auth-secret"
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
