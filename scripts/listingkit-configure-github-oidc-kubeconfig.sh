#!/usr/bin/env bash

set -euo pipefail

usage() {
  printf 'Usage: %s --token-file PATH --kubeconfig PATH --cluster-server URL --cluster-ca-b64 BASE64 --issuer URL --audience VALUE --subject VALUE --repository OWNER/REPO --environment NAME --workflow-path PATH\n' "$0" >&2
}

token_file=""
kubeconfig=""
cluster_server=""
cluster_ca_b64=""
issuer=""
audience=""
subject=""
repository=""
environment=""
workflow_path=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --token-file) token_file="${2:-}"; shift 2 ;;
    --kubeconfig) kubeconfig="${2:-}"; shift 2 ;;
    --cluster-server) cluster_server="${2:-}"; shift 2 ;;
    --cluster-ca-b64) cluster_ca_b64="${2:-}"; shift 2 ;;
    --issuer) issuer="${2:-}"; shift 2 ;;
    --audience) audience="${2:-}"; shift 2 ;;
    --subject) subject="${2:-}"; shift 2 ;;
    --repository) repository="${2:-}"; shift 2 ;;
    --environment) environment="${2:-}"; shift 2 ;;
    --workflow-path) workflow_path="${2:-}"; shift 2 ;;
    *) usage; exit 2 ;;
  esac
done

for required in token_file kubeconfig cluster_server cluster_ca_b64 issuer audience subject repository environment workflow_path; do
  if [[ -z "${!required}" ]]; then
    usage
    exit 2
  fi
done
if [[ ! -f "$token_file" ]]; then
  printf 'OIDC token file is missing\n' >&2
  exit 1
fi
if [[ "$cluster_server" != https://* ]]; then
  printf 'Kubernetes cluster server must use HTTPS\n' >&2
  exit 1
fi

# This local claim check is fail-fast defense in depth only. It deliberately
# does not authenticate the JWT signature; kube-apiserver performs OIDC
# discovery, signature verification, issuer/audience validation, and RBAC.
node - "$token_file" "$issuer" "$audience" "$subject" "$repository" "$environment" "$workflow_path" <<'NODE'
const fs = require('fs');
const [tokenPath, issuer, audience, subject, repository, environment, workflowPath] = process.argv.slice(2);
const token = fs.readFileSync(tokenPath, 'utf8').trim();
const parts = token.split('.');
if (parts.length !== 3 || !parts.every(Boolean)) throw new Error('OIDC token is not a compact JWT');
let claims;
try {
  claims = JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8'));
} catch (_) {
  throw new Error('OIDC token payload is not valid base64url JSON');
}
const audiences = Array.isArray(claims.aud) ? claims.aud : [claims.aud];
const workflowPrefix = `${repository}/${workflowPath}@`;
const now = Math.floor(Date.now() / 1000);
const exact = claims.iss === issuer && audiences.includes(audience) &&
  claims.sub === subject && claims.repository === repository &&
  claims.environment === environment && typeof claims.workflow_ref === 'string' &&
  claims.workflow_ref.startsWith(workflowPrefix);
const bounded = Number.isInteger(claims.nbf) && Number.isInteger(claims.exp) &&
  claims.nbf <= now + 60 && claims.exp > now && claims.exp - claims.nbf <= 900;
if (!exact || !bounded) throw new Error('OIDC release claims do not match the approved short-lived identity');
NODE

node - "$cluster_ca_b64" <<'NODE'
const value = process.argv[2];
if (!/^[A-Za-z0-9+/]+={0,2}$/.test(value) || Buffer.from(value, 'base64').length === 0) {
  throw new Error('Kubernetes cluster CA is not valid non-empty base64');
}
NODE

token="$(<"$token_file")"
umask 077
mkdir -p "$(dirname "$kubeconfig")"
temporary="${kubeconfig}.tmp.$$"
trap 'rm -f "$temporary"' EXIT
cat >"$temporary" <<EOF
apiVersion: v1
kind: Config
clusters:
  - name: listingkit-production
    cluster:
      server: ${cluster_server}
      certificate-authority-data: ${cluster_ca_b64}
users:
  - name: github-release
    user:
      token: ${token}
contexts:
  - name: listingkit-production
    context:
      cluster: listingkit-production
      user: github-release
      namespace: task-processor
current-context: listingkit-production
EOF
chmod 600 "$temporary"
mv "$temporary" "$kubeconfig"
trap - EXIT
