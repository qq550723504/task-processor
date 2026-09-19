#!/bin/sh
set -eu
state=/state; source=/terraform-source; config="$state/config"; ca=/private/ca; inputs=/tofu-inputs; runtime=/runtime; frontend=/frontend
bootstrap_pat=${BOOTSTRAP_PAT_FILE:?BOOTSTRAP_PAT_FILE is required}
umask 077
test -f "$bootstrap_pat"; test -f "$ca/root-ca.pem"; test -f "$inputs/operator-password"
if [ -f "$state/.terraform-complete" ]; then exit 0; fi
if [ -f "$state/.terraform-started" ]; then echo 'local OpenTofu initialization is incomplete; do not retry this project' >&2; exit 1; fi
touch "$state/.terraform-started"; chmod 600 "$state/.terraform-started"
apk add --no-cache ca-certificates curl; export SSL_CERT_FILE="$ca/root-ca.pem"
mkdir -p "$config"; cp -R "$source/." "$config/"
until curl --fail --silent --show-error --cacert "$ca/root-ca.pem" https://localhost:18443/.well-known/openid-configuration >/dev/null; do sleep 2; done
cd "$config"; tofu init -input=false
tofu apply -input=false -auto-approve -var="bootstrap_pat=$(tr -d '\r\n' < "$bootstrap_pat")" -var="operator_password=$(tr -d '\r\n' < "$inputs/operator-password")" -state="$state/terraform.tfstate"
write_output(){ tofu output -state="$state/terraform.tfstate" -raw "$1" > "$2.tmp"; chmod 600 "$2.tmp"; mv "$2.tmp" "$2"; }
mkdir -p "$runtime" "$frontend"
write_output api_client_id "$runtime/api-client-id"; write_output api_client_secret "$runtime/api-client-secret"; write_output project_id "$runtime/project-id"
write_output oidc_client_id "$frontend/oidc-client-id"; write_output oidc_client_secret "$frontend/oidc-client-secret"; write_output project_id "$frontend/project-id"
mv "$state/.terraform-started" "$state/.terraform-complete"
