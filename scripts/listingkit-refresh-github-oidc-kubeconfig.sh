#!/usr/bin/env bash

set -euo pipefail

usage() {
  printf 'Usage: %s --kubeconfig PATH --cluster-server URL --cluster-ca-b64 BASE64 --audience VALUE --subject VALUE --repository OWNER/REPO --environment NAME --workflow-ref OWNER/REPO/PATH@REF\n' "$0" >&2
}

kubeconfig=""
cluster_server=""
cluster_ca_b64=""
audience=""
subject=""
repository=""
environment=""
workflow_ref=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --kubeconfig) kubeconfig="${2:-}"; shift 2 ;;
    --cluster-server) cluster_server="${2:-}"; shift 2 ;;
    --cluster-ca-b64) cluster_ca_b64="${2:-}"; shift 2 ;;
    --audience) audience="${2:-}"; shift 2 ;;
    --subject) subject="${2:-}"; shift 2 ;;
    --repository) repository="${2:-}"; shift 2 ;;
    --environment) environment="${2:-}"; shift 2 ;;
    --workflow-ref) workflow_ref="${2:-}"; shift 2 ;;
    *) usage; exit 2 ;;
  esac
done

for required in kubeconfig cluster_server cluster_ca_b64 audience subject repository environment workflow_ref; do
  [[ -n "${!required}" ]] || { usage; exit 2; }
done
for command in curl jq mktemp node; do
  command -v "$command" >/dev/null 2>&1 || { printf 'OIDC refresh helper requires %s\n' "$command" >&2; exit 1; }
done
[[ -n "${ACTIONS_ID_TOKEN_REQUEST_URL:-}" ]] || { printf 'GitHub OIDC request URL is unavailable\n' >&2; exit 1; }
[[ -n "${ACTIONS_ID_TOKEN_REQUEST_TOKEN:-}" ]] || { printf 'GitHub OIDC request token is unavailable\n' >&2; exit 1; }

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
token_file="$(mktemp "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/listingkit-oidc-token.XXXXXX")"
trap 'rm -f "$token_file"' EXIT
chmod 600 "$token_file"
separator='?'
[[ "$ACTIONS_ID_TOKEN_REQUEST_URL" == *'?'* ]] && separator='&'
encoded_audience="$(node -e 'process.stdout.write(encodeURIComponent(process.argv[1]))' "$audience")"
curl --fail --silent --show-error --retry 2 \
  -H "Authorization: bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
  "${ACTIONS_ID_TOKEN_REQUEST_URL}${separator}audience=${encoded_audience}" |
  jq -er '.value | select(type == "string" and length > 0)' >"$token_file"

bash "$script_dir/listingkit-configure-github-oidc-kubeconfig.sh" \
  --token-file "$token_file" \
  --kubeconfig "$kubeconfig" \
  --cluster-server "$cluster_server" \
  --cluster-ca-b64 "$cluster_ca_b64" \
  --issuer https://token.actions.githubusercontent.com \
  --audience "$audience" \
  --subject "$subject" \
  --repository "$repository" \
  --environment "$environment" \
  --workflow-ref "$workflow_ref"
