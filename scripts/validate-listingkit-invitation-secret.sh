#!/usr/bin/env bash
set -euo pipefail

namespace="${1:?usage: validate-listingkit-invitation-secret.sh <namespace>}"
secret="listingkit-member-invitation-secret"

if ! secret_status="$(kubectl -n "$namespace" get secret "$secret" -o name 2>&1)"; then
  if grep -q "NotFound" <<<"$secret_status"; then
    echo "::error::Missing required ListingKit invitation Secret: $secret"
  else
    echo "::error::Could not inspect required ListingKit invitation Secret: $secret"
  fi
  exit 1
fi

for key in \
  TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN \
  TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID; do
  if ! value="$(kubectl -n "$namespace" get secret "$secret" -o "jsonpath={.data.$key}" 2>&1)"; then
    echo "::error::Could not inspect required ListingKit invitation Secret key: $key"
    exit 1
  fi
  if [[ -z "$value" ]]; then
    echo "::error::Missing required ListingKit invitation Secret key: $key"
    exit 1
  fi
done
