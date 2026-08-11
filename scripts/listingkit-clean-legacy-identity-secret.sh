#!/usr/bin/env bash
set -euo pipefail

namespace="${1:?usage: listingkit-clean-legacy-identity-secret.sh <namespace> [secret] [deployment]}"
secret="${2:-listingkit-workbench-secret}"
deployment="${3:-product-listing-api}"

if [[ ! "$deployment" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
  printf 'deployment name must be a DNS label: %s\n' "$deployment" >&2
  exit 2
fi

if ! kubectl -n "$namespace" get secret "$secret" -o name >/dev/null 2>&1; then
  printf 'could not inspect required shared Secret %s\n' "$secret" >&2
  exit 1
fi

secret_data_value() {
  kubectl -n "$namespace" get secret "$secret" -o "jsonpath={.data.${1}}"
}

resource_version_value() {
  kubectl -n "$namespace" get secret "$secret" -o 'jsonpath={.metadata.resourceVersion}'
}

has_key() {
  [[ -n "$(secret_data_value "$1")" ]]
}

has_usable_roles() {
  local encoded="$1"
  local decoded
  if ! decoded="$(printf '%s' "$encoded" | base64 --decode 2>/dev/null)"; then
    return 1
  fi
  [[ -n "$(printf '%s' "$decoded" | tr ',' '\n' | sed 's/[[:space:]]//g' | grep -m1 -E '.+')" ]]
}

json_escape() {
  local escaped="$1"
  escaped="${escaped//\\/\\\\}"
  escaped="${escaped//\"/\\\"}"
  escaped="${escaped//$'\r'/\\r}"
  escaped="${escaped//$'\n'/\\n}"
  printf '%s' "$escaped"
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
  canonical_roles_value="$(secret_data_value "$canonical_roles_key")"
  if [[ -n "$canonical_roles_value" ]] && has_usable_roles "$canonical_roles_value"; then
    patch_parts+=("{\"op\":\"remove\",\"path\":\"/data/${legacy_roles_key}\"}")
  else
    legacy_roles_value="$(secret_data_value "$legacy_roles_key")"
    if [[ -z "$legacy_roles_value" ]]; then
      printf 'deprecated ListingKit roles key has no readable value in shared Secret %s\n' "$secret" >&2
      exit 1
    fi
    patch_parts+=("{\"op\":\"add\",\"path\":\"/data/${canonical_roles_key}\",\"value\":\"${legacy_roles_value}\"}")
    patch_parts+=("{\"op\":\"remove\",\"path\":\"/data/${legacy_roles_key}\"}")
  fi
fi

if ((${#patch_parts[@]} == 0)); then
  kubectl -n "$namespace" rollout restart "deployment/${deployment}" >/dev/null
  printf 'shared Secret %s has no deprecated ListingKit identity keys\n' "$secret"
  exit 0
fi

resource_version="$(resource_version_value)"
if [[ -z "$resource_version" ]]; then
  printf 'shared Secret %s has no resourceVersion for safe cleanup\n' "$secret" >&2
  exit 1
fi
escaped_resource_version="$(json_escape "$resource_version")"
patch_parts=("{\"op\":\"test\",\"path\":\"/metadata/resourceVersion\",\"value\":\"${escaped_resource_version}\"}" "${patch_parts[@]}")
patch="[$(IFS=,; printf '%s' "${patch_parts[*]}")]"

kubectl -n "$namespace" patch secret "$secret" --type=json -p "$patch" >/dev/null
kubectl -n "$namespace" rollout restart "deployment/${deployment}" >/dev/null
printf 'removed deprecated ListingKit identity keys from shared Secret %s\n' "$secret"
