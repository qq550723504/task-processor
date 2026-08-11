#!/usr/bin/env bash
set -euo pipefail

namespace="${1:?usage: listingkit-clean-legacy-identity-secret.sh <namespace> [secret]}"
secret="${2:-listingkit-workbench-secret}"

if ! secret_json="$(kubectl -n "$namespace" get secret "$secret" -o json 2>&1)"; then
  printf 'could not inspect required shared Secret %s\n' "$secret" >&2
  exit 1
fi

patch_parts=()
for key in \
  LISTINGKIT_ZITADEL_ALLOWED_USERNAMES \
  LISTINGKIT_ZITADEL_ALLOWED_ROLES; do
  if grep -Eq "\"${key}\"[[:space:]]*:" <<<"$secret_json"; then
    patch_parts+=("{\"op\":\"remove\",\"path\":\"/data/${key}\"}")
  fi
done

patch="[$(IFS=,; printf '%s' "${patch_parts[*]}")]"

if [[ "$patch" == "[]" ]]; then
  printf 'shared Secret %s has no deprecated ListingKit identity keys\n' "$secret"
  exit 0
fi

kubectl -n "$namespace" patch secret "$secret" --type=json -p "$patch" >/dev/null
printf 'removed deprecated ListingKit identity keys from shared Secret %s\n' "$secret"
