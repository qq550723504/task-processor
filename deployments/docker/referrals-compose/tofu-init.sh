#!/bin/sh
set -eu

private=/private
state=/state
source=/terraform-source
terraform="$state/config"
bootstrap_pat=${BOOTSTRAP_PAT_FILE:?BOOTSTRAP_PAT_FILE is required}

umask 077
test -f "$bootstrap_pat"
test -f "$private/ca/root-ca.pem"
test -f "$private/secrets/operator-password"

if [ -f "$private/.terraform-complete" ]; then
  exit 0
fi
if [ -f "$private/.terraform-started" ]; then
  echo "local OpenTofu initialization is incomplete; recreate this Compose project's named volumes with the documented destroy command" >&2
  exit 1
fi
touch "$private/.terraform-started"
chmod 600 "$private/.terraform-started"

apk add --no-cache ca-certificates curl
export SSL_CERT_FILE="$private/ca/root-ca.pem"

if [ ! -f "$state/.config-copied" ]; then
  mkdir -p "$terraform"
  cp -R "$source/." "$terraform/"
  touch "$state/.config-copied"
fi

until curl --fail --silent --show-error --cacert "$private/ca/root-ca.pem" \
  https://localhost:18443/.well-known/openid-configuration >/dev/null; do
  sleep 2
done

mkdir -p "$state/main" "$state/probe"
cd "$terraform"
tofu init -input=false
tofu apply -input=false -auto-approve \
  -var="bootstrap_pat=$(tr -d '\r\n' < "$bootstrap_pat")" \
  -var="operator_password=$(tr -d '\r\n' < "$private/secrets/operator-password")" \
  -state="$state/main/terraform.tfstate"

write_output() {
  name="$1"
  target="$2"
  tofu output -state="$state/main/terraform.tfstate" -raw "$name" > "$target.tmp"
  chmod 600 "$target.tmp"
  mv "$target.tmp" "$target"
}

mkdir -p "$private/runtime"
write_output provider_pat "$private/runtime/provider-machine.pat"
write_output api_client_id "$private/runtime/api-client-id"
write_output api_client_secret "$private/runtime/api-client-secret"
write_output oidc_client_id "$private/runtime/oidc-client-id"
write_output oidc_client_secret "$private/runtime/oidc-client-secret"
write_output signup_org_id "$private/runtime/signup-org-id"
write_output project_id "$private/runtime/project-id"

cd "$terraform/probe"
tofu init -input=false
tofu apply -input=false -auto-approve \
  -var="provider_pat=$(tr -d '\r\n' < "$private/runtime/provider-machine.pat")" \
  -var="signup_org_id=$(tr -d '\r\n' < "$private/runtime/signup-org-id")" \
  -var="probe_password=$(tr -d '\r\n' < "$private/secrets/operator-password")" \
  -state="$state/probe/terraform.tfstate"
tofu destroy -input=false -auto-approve \
  -var="provider_pat=$(tr -d '\r\n' < "$private/runtime/provider-machine.pat")" \
  -var="signup_org_id=$(tr -d '\r\n' < "$private/runtime/signup-org-id")" \
  -var="probe_password=$(tr -d '\r\n' < "$private/secrets/operator-password")" \
  -state="$state/probe/terraform.tfstate"

mv "$private/.terraform-started" "$private/.terraform-complete"
