#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
driver="$repo_root/scripts/listingkit-schema-migrate-job.sh"
manifest="$repo_root/deployments/kubernetes/listingkit-workbench/jobs/listingkit-schema-migrate-job.yaml"
product_manifest="$repo_root/deployments/kubernetes/listingkit-workbench/jobs/product-listing-api-schema-migrate-job.yaml"
workflow="$repo_root/.github/workflows/listingkit-deploy.yml"

if [[ ! -x "$driver" ]]; then
  printf 'missing executable schema migration Job driver: %s\n' "$driver" >&2
  exit 1
fi
if [[ ! -f "$manifest" ]]; then
  printf 'missing schema migration Job manifest: %s\n' "$manifest" >&2
  exit 1
fi
if [[ ! -f "$product_manifest" ]]; then
  printf 'missing product-listing API schema migration Job manifest: %s\n' "$product_manifest" >&2
  exit 1
fi
grep -q 'image: REPLACE_WITH_API_IMAGE' "$product_manifest" || {
  printf 'product-listing API migration manifest must use the immutable API image placeholder\n' >&2
  exit 1
}
grep -q 'activeDeadlineSeconds: 900' "$product_manifest" || {
  printf 'product-listing API migration manifest must bound execution to the driver wait\n' >&2
  exit 1
}
grep -q 'product-listing-api-schema-migrate-job.yaml' "$workflow" || {
  printf 'deployment workflow does not run the product-listing API migration Job\n' >&2
  exit 1
}

printf '%s\n' 'listingkit schema migration Job driver tests passed'
