#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
driver="$repo_root/scripts/listingkit-schema-migrate-job.sh"
manifest="$repo_root/deployments/kubernetes/listingkit-workbench/jobs/listingkit-schema-migrate-job.yaml"

if [[ ! -x "$driver" ]]; then
  printf 'missing executable schema migration Job driver: %s\n' "$driver" >&2
  exit 1
fi
if [[ ! -f "$manifest" ]]; then
  printf 'missing schema migration Job manifest: %s\n' "$manifest" >&2
  exit 1
fi

printf '%s\n' 'listingkit schema migration Job driver tests passed'
