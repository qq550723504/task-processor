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

if ! secret_snapshot="$(kubectl -n "$namespace" get secret "$secret" -o 'jsonpath={.metadata.resourceVersion}{"\n"}{.data.LISTINGKIT_ZITADEL_ALLOWED_USERNAMES}{"\n"}{.data.TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES}{"\n"}{.data.LISTINGKIT_ZITADEL_ALLOWED_ROLES}{"\n"}{.data.TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES}{"\n"}{"END"}')"; then
  printf 'could not read shared Secret data %s\n' "$secret" >&2
  exit 1
fi
snapshot_fields=()
while IFS= read -r snapshot_field; do
  snapshot_fields+=("$snapshot_field")
done <<<"$secret_snapshot"
if (( ${#snapshot_fields[@]} != 6 )); then
  printf 'shared Secret %s returned an invalid cleanup snapshot\n' "$secret" >&2
  exit 1
fi
resource_version="${snapshot_fields[0]}"
legacy_usernames="${snapshot_fields[1]}"
primary_usernames="${snapshot_fields[2]}"
legacy_roles="${snapshot_fields[3]}"
canonical_roles="${snapshot_fields[4]}"
snapshot_end="${snapshot_fields[5]}"
if [[ "$snapshot_end" != END ]]; then
  printf 'shared Secret %s returned an invalid cleanup marker\n' "$secret" >&2
  exit 1
fi
if [[ -z "$resource_version" ]]; then
  printf 'shared Secret %s has no resourceVersion for safe cleanup\n' "$secret" >&2
  exit 1
fi

secret_data_value() {
  case "$1" in
    LISTINGKIT_ZITADEL_ALLOWED_USERNAMES) printf '%s' "$legacy_usernames" ;;
    TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES) printf '%s' "$primary_usernames" ;;
    LISTINGKIT_ZITADEL_ALLOWED_ROLES) printf '%s' "$legacy_roles" ;;
    TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES) printf '%s' "$canonical_roles" ;;
    *) return 1 ;;
  esac
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

restart_if_present() {
  if kubectl -n "$namespace" get "deployment/${deployment}" -o name >/dev/null 2>&1; then
    kubectl -n "$namespace" rollout restart "deployment/${deployment}" >/dev/null
  fi
}

if ((${#patch_parts[@]} == 0)); then
  restart_if_present
  printf 'shared Secret %s has no deprecated ListingKit identity keys\n' "$secret"
  exit 0
fi

escaped_resource_version="$(json_escape "$resource_version")"
patch_parts=("{\"op\":\"test\",\"path\":\"/metadata/resourceVersion\",\"value\":\"${escaped_resource_version}\"}" "${patch_parts[@]}")
patch="[$(IFS=,; printf '%s' "${patch_parts[*]}")]"

kubectl -n "$namespace" patch secret "$secret" --type=json -p "$patch" >/dev/null
restart_if_present
printf 'removed deprecated ListingKit identity keys from shared Secret %s\n' "$secret"
