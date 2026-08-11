#!/usr/bin/env bash
set -euo pipefail

namespace="${1:?usage: listingkit-clean-legacy-identity-secret.sh <namespace> [secret] [deployment]}"
secret="${2:-listingkit-workbench-secret}"
deployment="${3:-product-listing-api}"

if [[ ! "$deployment" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
  printf 'deployment name must be a DNS label: %s\n' "$deployment" >&2
  exit 2
fi

if ! secret_json="$(kubectl -n "$namespace" get secret "$secret" -o json 2>&1)"; then
  printf 'could not inspect required shared Secret %s\n' "$secret" >&2
  exit 1
fi

if command -v jq >/dev/null 2>&1; then
  json_has_key() {
    jq -e --arg key "$1" '.data[$key] != null' <<<"$secret_json" >/dev/null
  }
  json_value() {
    jq -r --arg key "$1" '.data[$key] // empty' <<<"$secret_json"
  }
  json_resource_version() {
    jq -r '.metadata.resourceVersion // empty' <<<"$secret_json"
  }
elif command -v python3 >/dev/null 2>&1; then
  json_python="$(command -v python3)"
  json_has_key() {
    printf '%s' "$secret_json" | "$json_python" -c 'import json,sys; key=sys.argv[1]; obj=json.load(sys.stdin); sys.exit(0 if key in obj.get("data", {}) else 1)' "$1"
  }
  json_value() {
    printf '%s' "$secret_json" | "$json_python" -c 'import json,sys; key=sys.argv[1]; value=json.load(sys.stdin).get("data", {}).get(key); print(value if value is not None else "")' "$1"
  }
  json_resource_version() {
    printf '%s' "$secret_json" | "$json_python" -c 'import json,sys; value=json.load(sys.stdin).get("metadata", {}).get("resourceVersion"); print(value if value is not None else "")'
  }
else
  printf 'jq or python3 is required to inspect Secret data safely\n' >&2
  exit 1
fi

has_key() {
  json_has_key "$1"
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
  if has_key "$canonical_roles_key" && [[ -n "$(json_value "$canonical_roles_key")" ]]; then
    patch_parts+=("{\"op\":\"remove\",\"path\":\"/data/${legacy_roles_key}\"}")
  else
    legacy_roles_value="$(json_value "$legacy_roles_key")"
    if [[ -z "$legacy_roles_value" ]]; then
      printf 'deprecated ListingKit roles key has no readable value in shared Secret %s\n' "$secret" >&2
      exit 1
    fi
    patch_parts+=("{\"op\":\"add\",\"path\":\"/data/${canonical_roles_key}\",\"value\":\"${legacy_roles_value}\"}")
    patch_parts+=("{\"op\":\"remove\",\"path\":\"/data/${legacy_roles_key}\"}")
  fi
fi

if ((${#patch_parts[@]} == 0)); then
  printf 'shared Secret %s has no deprecated ListingKit identity keys\n' "$secret"
  exit 0
fi

resource_version="$(json_resource_version)"
if [[ -z "$resource_version" || ! "$resource_version" =~ ^[A-Za-z0-9._-]+$ ]]; then
  printf 'shared Secret %s has no valid resourceVersion for safe cleanup\n' "$secret" >&2
  exit 1
fi
patch_parts=("{\"op\":\"test\",\"path\":\"/metadata/resourceVersion\",\"value\":\"${resource_version}\"}" "${patch_parts[@]}")
patch="[$(IFS=,; printf '%s' "${patch_parts[*]}")]"

kubectl -n "$namespace" patch secret "$secret" --type=json -p "$patch" >/dev/null
kubectl -n "$namespace" rollout restart "deployment/${deployment}" >/dev/null
printf 'removed deprecated ListingKit identity keys from shared Secret %s\n' "$secret"
