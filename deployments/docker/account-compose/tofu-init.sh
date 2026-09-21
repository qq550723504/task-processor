#!/bin/sh
set -eu
# Keep this entrypoint materialized as LF on existing Windows worktrees.

state=/state
terraform_source=/terraform-source
trusted_ca=/private/ca
tofu_inputs=/tofu-inputs
runtime=/runtime
frontend=/frontend
bootstrap_pat=${BOOTSTRAP_PAT_FILE:?BOOTSTRAP_PAT_FILE is required}
identity_port=${ACCOUNT_IDENTITY_PORT:?ACCOUNT_IDENTITY_PORT is required}
application_port=${ACCOUNT_APPLICATION_PORT:?ACCOUNT_APPLICATION_PORT is required}

umask 077
test -f "$bootstrap_pat"; test -f "$trusted_ca/root-ca.pem"; test -f "$tofu_inputs/operator-password"
if [ -f "$state/.terraform-complete" ]; then exit 0; fi
if [ -f "$state/.terraform-started" ]; then echo 'local OpenTofu initialization is incomplete; recreate this Compose project' >&2; exit 1; fi
touch "$state/.terraform-started"
chmod 600 "$state/.terraform-started"

apk add --no-cache ca-certificates curl
export SSL_CERT_FILE="$trusted_ca/root-ca.pem"
mkdir -p "$state/config" "$runtime" "$frontend"
cp -R "$terraform_source/." "$state/config/"
# This container shares the runtime-network namespace with Traefik.  The
# host-specific port is only for callers outside the Compose project; inside
# the namespace Traefik listens on its fixed identity entrypoint.
until curl --fail --silent --show-error --cacert "$trusted_ca/root-ca.pem" "https://localhost:${identity_port}/.well-known/openid-configuration" >/dev/null; do sleep 2; done
cd "$state/config"
tofu init -input=false
tofu apply -input=false -auto-approve \
  -var="bootstrap_pat=$(tr -d '\r\n' < "$bootstrap_pat")" \
  -var="operator_password=$(tr -d '\r\n' < "$tofu_inputs/operator-password")" \
  -var="identity_port=$identity_port" \
  -var="application_port=$application_port" \
  -state="$state/terraform.tfstate"

write_output() {
  tofu output -state="$state/terraform.tfstate" -raw "$1" > "$2.tmp"
  chmod 600 "$2.tmp"
  mv "$2.tmp" "$2"
}
write_output provider_pat "$runtime/provider-machine.pat"
write_output membership_read_pat "$runtime/membership-read.pat"
write_output membership_write_pat "$runtime/membership-write.pat"
write_output api_client_id "$runtime/api-client-id"
write_output api_client_secret "$runtime/api-client-secret"
write_output signup_org_id "$runtime/signup-org-id"
write_output project_id "$runtime/project-id"
write_output oidc_client_id "$frontend/oidc-client-id"
write_output oidc_client_secret "$frontend/oidc-client-secret"
write_output project_id "$frontend/project-id"

mv "$state/.terraform-started" "$state/.terraform-complete"
