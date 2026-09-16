#!/bin/sh
set -eu

state=/state
source=/terraform-source
terraform="$state/config"
trusted_ca=/private/ca
tofu_inputs=/tofu-inputs
runtime=/runtime
frontend=/frontend
bootstrap_pat=${BOOTSTRAP_PAT_FILE:?BOOTSTRAP_PAT_FILE is required}

umask 077
test -f "$bootstrap_pat"
test -f "$trusted_ca/root-ca.pem"
test -f "$tofu_inputs/operator-password"

if [ -f "$state/.terraform-complete" ]; then
  exit 0
fi
if [ -f "$state/.terraform-started" ]; then
  echo "local OpenTofu initialization is incomplete; recreate this Compose project's named volumes with the documented destroy command" >&2
  exit 1
fi
touch "$state/.terraform-started"
chmod 600 "$state/.terraform-started"

apk add --no-cache ca-certificates curl
export SSL_CERT_FILE="$trusted_ca/root-ca.pem"

if [ ! -f "$state/.config-copied" ]; then
  mkdir -p "$terraform"
  cp -R "$source/." "$terraform/"
  touch "$state/.config-copied"
fi

until curl --fail --silent --show-error --cacert "$trusted_ca/root-ca.pem" \
  https://localhost:18443/.well-known/openid-configuration >/dev/null; do
  sleep 2
done

mkdir -p "$state/main" "$state/probe"
cd "$terraform"
tofu init -input=false
tofu apply -input=false -auto-approve \
  -var="bootstrap_pat=$(tr -d '\r\n' < "$bootstrap_pat")" \
  -var="operator_password=$(tr -d '\r\n' < "$tofu_inputs/operator-password")" \
  -state="$state/main/terraform.tfstate"

write_output() {
  name="$1"
  target="$2"
  tofu output -state="$state/main/terraform.tfstate" -raw "$name" > "$target.tmp"
  chmod 600 "$target.tmp"
  mv "$target.tmp" "$target"
}

mkdir -p "$runtime" "$frontend"
write_output provider_pat "$runtime/provider-machine.pat"
write_output api_client_id "$runtime/api-client-id"
write_output api_client_secret "$runtime/api-client-secret"
write_output signup_org_id "$runtime/signup-org-id"
write_output project_id "$runtime/project-id"
write_output oidc_client_id "$frontend/oidc-client-id"
write_output oidc_client_secret "$frontend/oidc-client-secret"
write_output project_id "$frontend/project-id"

cd "$terraform/probe"
tofu init -input=false
tofu apply -input=false -auto-approve \
  -var="provider_pat=$(tr -d '\r\n' < "$runtime/provider-machine.pat")" \
  -var="signup_org_id=$(tr -d '\r\n' < "$runtime/signup-org-id")" \
  -var="probe_password=$(tr -d '\r\n' < "$tofu_inputs/operator-password")" \
  -state="$state/probe/terraform.tfstate"
tofu destroy -input=false -auto-approve \
  -var="provider_pat=$(tr -d '\r\n' < "$runtime/provider-machine.pat")" \
  -var="signup_org_id=$(tr -d '\r\n' < "$runtime/signup-org-id")" \
  -var="probe_password=$(tr -d '\r\n' < "$tofu_inputs/operator-password")" \
  -state="$state/probe/terraform.tfstate"

mv "$state/.terraform-started" "$state/.terraform-complete"
