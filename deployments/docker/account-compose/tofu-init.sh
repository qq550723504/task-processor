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
test -f "$bootstrap_pat"; test -f "$trusted_ca/root-ca.pem"; test -f "$tofu_inputs/operator-password"; test -f "$tofu_inputs/viewer-password"; test -f "$tofu_inputs/insufficient-password"

write_output() {
  tofu output -state="$state/terraform.tfstate" -raw "$1" > "$2.tmp"
  chmod 600 "$2.tmp"
  mv "$2.tmp" "$2"
}

ensure_jq() {
  if command -v jq >/dev/null 2>&1; then return; fi
  if ! apk add --no-cache jq >/dev/null 2>&1; then
    echo 'could not prepare retained OpenTofu state reader' >&2
    return 1
  fi
}

extract_bootstrap_user_id() {
  ensure_jq || return 1
  if ! jq -er '
    [ .resources[]?
      | select(.mode == "managed" and .type == "zitadel_human_user" and .name == "operator" and ((.module // "") == ""))
      | .instances[]?
      | select((.index_key? == null) and (.deposed_key? == null))
      | .attributes.id
    ] as $ids
    | if ($ids | length) != 1 then
        error("expected exactly one valid bootstrap operator ID")
      else
        $ids[0] as $id
        | if ($id | type) != "string" then
            error("expected exactly one valid bootstrap operator ID")
          elif ($id | length) < 1 or ($id | length) > 256 then
            error("expected exactly one valid bootstrap operator ID")
          elif ($id | test("^[A-Za-z0-9][A-Za-z0-9._:-]*$")) then
            $id
          else
            error("expected exactly one valid bootstrap operator ID")
          end
      end
  ' "$1" > "$2.tmp" 2>/dev/null; then
    rm -f "$2.tmp"
    echo 'could not recover bootstrap user identity from retained state' >&2
    return 1
  fi
  if ! chmod 600 "$2.tmp" || ! mv "$2.tmp" "$2"; then
    rm -f "$2.tmp"
    echo 'could not materialize recovered bootstrap user identity' >&2
    return 1
  fi
}

if [ -f "$state/.terraform-complete" ]; then
  mkdir -p "$runtime"
  if [ ! -s "$runtime/bootstrap-user-id" ]; then
    extract_bootstrap_user_id "$state/terraform.tfstate" "$runtime/bootstrap-user-id"
  fi
  exit 0
fi
if [ -f "$state/.terraform-started" ]; then echo 'local OpenTofu initialization is incomplete; recreate this Compose project' >&2; exit 1; fi
touch "$state/.terraform-started"
chmod 600 "$state/.terraform-started"

apk add --no-cache ca-certificates curl jq
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
  -var="viewer_password=$(tr -d '\r\n' < "$tofu_inputs/viewer-password")" \
  -var="insufficient_password=$(tr -d '\r\n' < "$tofu_inputs/insufficient-password")" \
  -var="identity_port=$identity_port" \
  -var="application_port=$application_port" \
  -state="$state/terraform.tfstate"

write_output provider_pat "$runtime/provider-machine.pat"
write_output membership_read_pat "$runtime/membership-read.pat"
write_output membership_write_pat "$runtime/membership-write.pat"
write_output api_client_id "$runtime/api-client-id"
write_output api_client_secret "$runtime/api-client-secret"
write_output signup_org_id "$runtime/signup-org-id"
write_output project_id "$runtime/project-id"
write_output bootstrap_user_id "$runtime/bootstrap-user-id"
write_output viewer_user_id "$runtime/viewer-user-id"
write_output insufficient_user_id "$runtime/insufficient-user-id"
write_output oidc_client_id "$frontend/oidc-client-id"
write_output oidc_client_secret "$frontend/oidc-client-secret"
write_output project_id "$frontend/project-id"

mv "$state/.terraform-started" "$state/.terraform-complete"
