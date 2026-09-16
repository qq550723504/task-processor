#!/bin/sh
set -eu

private=/private
marker="$private/.bootstrap-complete"

if [ -f "$marker" ]; then
  for path in \
    "$private/ca/root-ca.pem" \
    "$private/ca/localhost.crt" \
    "$private/ca/localhost.key" \
    "$private/secrets/identity-db-password" \
    "$private/secrets/source-db-password" \
    "$private/secrets/source-runtime-password" \
    "$private/secrets/commercial-db-password" \
    "$private/secrets/commercial-reader-password" \
    "$private/secrets/referral-db-password" \
    "$private/secrets/referral-runtime-password" \
    "$private/secrets/zitadel-masterkey" \
    "$private/zitadel/runtime-config.yaml" \
    "$private/secrets/auth-secret" \
    "$private/secrets/operator-password"; do
    test -f "$path"
  done
  exit 0
fi

umask 077
mkdir -p "$private/ca" "$private/secrets" "$private/zitadel"

write_random() {
  openssl rand -hex "$1" | tr -d '\n' > "$2.tmp"
  chmod 600 "$2.tmp"
  mv "$2.tmp" "$2"
}

write_random 16 "$private/secrets/zitadel-masterkey"
write_random 24 "$private/secrets/identity-db-password"
write_random 24 "$private/secrets/source-db-password"
write_random 24 "$private/secrets/source-runtime-password"
write_random 24 "$private/secrets/commercial-db-password"
write_random 24 "$private/secrets/commercial-reader-password"
write_random 24 "$private/secrets/referral-db-password"
write_random 24 "$private/secrets/referral-runtime-password"
write_random 32 "$private/secrets/auth-secret"
{
  printf 'Local!'
  openssl rand -hex 18
} > "$private/secrets/operator-password.tmp"
chmod 600 "$private/secrets/operator-password.tmp"
mv "$private/secrets/operator-password.tmp" "$private/secrets/operator-password"

identity_db_password=$(cat "$private/secrets/identity-db-password")
cat > "$private/zitadel/runtime-config.yaml.tmp" <<EOF
Database:
  postgres:
    DSN: "postgresql://postgres:${identity_db_password}@127.0.0.1:5432/zitadel?sslmode=disable"
EOF
chmod 600 "$private/zitadel/runtime-config.yaml.tmp"
mv "$private/zitadel/runtime-config.yaml.tmp" "$private/zitadel/runtime-config.yaml"

cat > "$private/ca/openssl.cnf.tmp" <<'EOF'
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
openssl req -x509 -newkey rsa:2048 -nodes -days 365 -sha256 \
  -subj '/CN=Task Processor Local Referrals CA' \
  -keyout "$private/ca/root-ca.key" -out "$private/ca/root-ca.pem"
openssl req -newkey rsa:2048 -nodes -sha256 -config "$private/ca/openssl.cnf.tmp" \
  -keyout "$private/ca/localhost.key" -out "$private/ca/localhost.csr"
openssl x509 -req -days 365 -sha256 -CA "$private/ca/root-ca.pem" -CAkey "$private/ca/root-ca.key" \
  -CAcreateserial -in "$private/ca/localhost.csr" -out "$private/ca/localhost.crt" \
  -extensions req_extensions -extfile "$private/ca/openssl.cnf.tmp"
rm -f "$private/ca/openssl.cnf.tmp" "$private/ca/localhost.csr" "$private/ca/root-ca.srl"
chmod 600 "$private/ca/root-ca.key" "$private/ca/root-ca.pem" "$private/ca/localhost.key" "$private/ca/localhost.crt"
touch "$marker"
chmod 600 "$marker"
