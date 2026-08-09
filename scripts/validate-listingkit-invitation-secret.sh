#!/usr/bin/env bash
set -euo pipefail

namespace="${1:?usage: validate-listingkit-invitation-secret.sh <namespace>}"
secret="listingkit-member-invitation-secret"

if ! secret_json="$(kubectl -n "$namespace" get secret "$secret" -o json 2>&1)"; then
  if grep -q "NotFound" <<<"$secret_json"; then
    echo "::error::Missing required ListingKit invitation Secret: $secret"
  else
    echo "::error::Could not inspect required ListingKit invitation Secret: $secret"
  fi
  exit 1
fi

for key in \
  TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN \
  TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID; do
  if ! jq -e --arg key "$key" '.data[$key] != null and .data[$key] != ""' <<<"$secret_json" >/dev/null; then
    echo "::error::Missing required ListingKit invitation Secret key: $key"
    exit 1
  fi
done
