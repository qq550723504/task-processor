#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflow="$repo_root/.github/workflows/listingkit-deploy.yml"
runner_manifest="$repo_root/deployments/kubernetes/listingkit-workbench/release-authority/listingkit-release-gate-runners.yaml"
worker_manifest="$repo_root/deployments/kubernetes/listingkit-workbench/base/image-agent-temporal-worker-v3-deployment.yaml"

[[ -f "$workflow" && -f "$runner_manifest" && -f "$worker_manifest" ]] || {
  printf 'image-agent canary order guard inputs are missing\n' >&2
  exit 1
}

canary_line="$(grep -n -- '--deployment image-agent-temporal-v3-canary-runner' "$workflow" | cut -d: -f1)"
production_apply_line="$(grep -n -- '--manifest .workflow-tools/deployments/kubernetes/listingkit-workbench/base/image-agent-temporal-worker-v3-deployment.yaml' "$workflow" | cut -d: -f1)"

[[ -n "$canary_line" && -n "$production_apply_line" && "$canary_line" -lt "$production_apply_line" ]] || {
  printf 'v3 production worker rollout must follow the isolated canary gate\n' >&2
  exit 1
}
grep -q '"image-agent-manual-v3-canary"' "$runner_manifest" || {
  printf 'v3 canary runner must use the isolated image-agent-manual-v3-canary queue\n' >&2
  exit 1
}
if grep -q -- '-canary-task-queue", "image-agent-manual-v3"' "$runner_manifest"; then
  printf 'v3 canary runner must not share the production worker queue\n' >&2
  exit 1
fi
grep -q '"image-agent-manual-v3"' "$worker_manifest" || {
  printf 'v3 production worker queue contract is missing\n' >&2
  exit 1
}

printf '%s\n' 'listingkit image-agent canary order guard passed'
