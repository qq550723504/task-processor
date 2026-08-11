#!/usr/bin/env bash
set -euo pipefail

namespace="${1:?usage: listingkit-clean-legacy-identity-secret.sh <namespace> [secret]}"
secret="${2:-listingkit-workbench-secret}"

if ! secret_json="$(kubectl -n "$namespace" get secret "$secret" -o json 2>&1)"; then
  printf 'could not inspect required shared Secret %s\n' "$secret" >&2
  exit 1
fi

has_key() {
  grep -Eq "\"$1\"[[:space:]]*:" <<<"$secret_json"
}

patch_parts=()
for key in \
  LISTINGKIT_ZITADEL_ALLOWED_USERNAMES \
  TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES; do
  if has_key "$key"; then
    patch_parts+=("{\"op\":\"remove\",\"path\":\"/data/${key}\"}")
  fi
done

legacy_roles_key=LISTINGKIT_ZITADEL_ALLOWED_ROLES
canonical_roles_key=TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES
if has_key "$legacy_roles_key"; then
  if has_key "$canonical_roles_key"; then
    patch_parts+=("{\"op\":\"remove\",\"path\":\"/data/${legacy_roles_key}\"}")
  else
    if ! legacy_roles_value="$(kubectl -n "$namespace" get secret "$secret" -o "jsonpath={.data.${legacy_roles_key}}" 2>&1)"; then
      printf 'could not read deprecated ListingKit roles from shared Secret %s\n' "$secret" >&2
      exit 1
    fi
    patch_parts+=("{\"op\":\"add\",\"path\":\"/data/${canonical_roles_key}\",\"value\":\"${legacy_roles_value}\"}")
    patch_parts+=("{\"op\":\"remove\",\"path\":\"/data/${legacy_roles_key}\"}")
  fi
fi

patch="[$(IFS=,; printf '%s' "${patch_parts[*]}")]"

if [[ "$patch" == "[]" ]]; then
  printf 'shared Secret %s has no deprecated ListingKit identity keys\n' "$secret"
  exit 0
fi

kubectl -n "$namespace" patch secret "$secret" --type=json -p "$patch" >/dev/null
printf 'removed deprecated ListingKit identity keys from shared Secret %s\n' "$secret"
