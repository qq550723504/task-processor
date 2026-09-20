#!/bin/sh
set -eu
state=/state; ca=/trusted-ca; tls=/traefik-tls; identity=/identity-db-secret
source_owner=/source-db-owner-secret; source_runtime=/source-runtime-secret
commercial_owner=/commercial-db-owner-secret; commercial_runtime=/commercial-runtime-secret
product_owner=/product-db-owner-secret; product_runtime=/product-runtime-secret
zitadel=/zitadel-api-secrets; inputs=/tofu-inputs; frontend=/frontend-secrets
marker="$state/.bootstrap-complete"
if [ -f "$marker" ]; then
  for path in "$ca/root-ca.pem" "$tls/localhost.crt" "$tls/localhost.key" "$identity/identity-db-password" "$source_owner/source-db-password" "$source_runtime/source-runtime-password" "$commercial_owner/commercial-db-password" "$commercial_runtime/commercial-reader-password" "$product_owner/product-db-password" "$product_runtime/product-runtime-password" "$zitadel/zitadel-masterkey" "$zitadel/runtime-config.yaml" "$inputs/operator-password" "$frontend/auth-secret"; do test -f "$path"; done
  exit 0
fi
umask 077
mkdir -p "$state" "$ca" "$tls" "$identity" "$source_owner" "$source_runtime" "$commercial_owner" "$commercial_runtime" "$product_owner" "$product_runtime" "$zitadel" "$inputs" "$frontend"
write_random(){ openssl rand -hex "$1" | tr -d '\n' > "$2.tmp"; chmod 600 "$2.tmp"; mv "$2.tmp" "$2"; }
write_random 16 "$zitadel/zitadel-masterkey"; write_random 24 "$identity/identity-db-password"
write_random 24 "$source_owner/source-db-password"; write_random 24 "$source_runtime/source-runtime-password"
write_random 24 "$commercial_owner/commercial-db-password"; write_random 24 "$commercial_runtime/commercial-reader-password"
write_random 24 "$product_owner/product-db-password"; write_random 24 "$product_runtime/product-runtime-password"
write_random 32 "$frontend/auth-secret"
{ printf 'Local!'; openssl rand -hex 18; } > "$inputs/operator-password.tmp"; chmod 600 "$inputs/operator-password.tmp"; mv "$inputs/operator-password.tmp" "$inputs/operator-password"
identity_password=$(cat "$identity/identity-db-password")
printf 'Database:\n  postgres:\n    DSN: "postgresql://postgres:%s@127.0.0.1:5432/zitadel?sslmode=disable"\n' "$identity_password" > "$zitadel/runtime-config.yaml.tmp"; chmod 600 "$zitadel/runtime-config.yaml.tmp"; mv "$zitadel/runtime-config.yaml.tmp" "$zitadel/runtime-config.yaml"
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
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
DNS.2 = acquisition.home.arpa
IP.1 = 127.0.0.1
IP.2 = ::1
EOF
openssl req -x509 -newkey rsa:2048 -nodes -days 365 -sha256 -subj '/CN=Task Processor Local Acquisition CA' -keyout "$work/root-ca.key" -out "$ca/root-ca.pem"
openssl req -newkey rsa:2048 -nodes -sha256 -config "$work/openssl.cnf" -keyout "$tls/localhost.key" -out "$work/localhost.csr"
openssl x509 -req -days 365 -sha256 -CA "$ca/root-ca.pem" -CAkey "$work/root-ca.key" -CAcreateserial -in "$work/localhost.csr" -out "$tls/localhost.crt" -extensions req_extensions -extfile "$work/openssl.cnf"
chmod 600 "$ca/root-ca.pem" "$tls/localhost.key" "$tls/localhost.crt"; touch "$marker"; chmod 600 "$marker"
