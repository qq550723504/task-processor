#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
driver="$repo_root/scripts/listingkit-clean-legacy-identity-secret.sh"

if [[ ! -x "$driver" ]]; then
  printf 'missing executable legacy identity Secret cleanup driver: %s\n' "$driver" >&2
  exit 1
fi

temp_dir="$(mktemp -d)"
trap 'rm -rf "$temp_dir"' EXIT
cat > "$temp_dir/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${FAKE_KUBECTL_LOG:?}"
if [[ "$*" == *"get secret listingkit-workbench-secret -o json"* ]]; then
  if [[ "${FAKE_KUBECTL_MODE:-normal}" == "annotation-only" ]]; then
    printf '%s\n' '{"data":{"TASK_PROCESSOR_DATABASE_HOST":"c2VydmljZQ=="},"metadata":{"resourceVersion":"rv-123","annotations":{"LISTINGKIT_ZITADEL_ALLOWED_USERNAMES":"annotation-only"}}}'
  else
    printf '%s\n' '{"data":{"LISTINGKIT_ZITADEL_ALLOWED_USERNAMES":"bGVnYWN5","TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES":"cHJpbWFyeQ==","LISTINGKIT_ZITADEL_ALLOWED_ROLES":"bGVnYWN5","TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES":"","TASK_PROCESSOR_DATABASE_HOST":"c2VydmljZQ=="},"metadata":{"resourceVersion":"rv-123","annotations":{"LISTINGKIT_ZITADEL_ALLOWED_USERNAMES":"annotation-only"}}}'
  fi
  exit 0
fi
if [[ "$*" == *"patch secret listingkit-workbench-secret"* ]]; then
  printf '%s\n' "$*" > "${FAKE_KUBECTL_PATCH:?}"
  exit 0
fi
if [[ "$*" == *"rollout restart deployment/product-listing-api"* ]]; then
  : > "${FAKE_KUBECTL_ROLLOUT:?}"
  exit 0
fi
printf 'unexpected fake kubectl call: %s\n' "$*" >&2
exit 1
EOF
chmod +x "$temp_dir/kubectl"
cat > "$temp_dir/jq" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
input="$(cat)"
if [[ "$*" == *"metadata.resourceVersion"* ]]; then
  printf '%s\n' 'rv-123'
  exit 0
fi
key="${4:-}"
data="${input#*\"data\":{}"
data="${data%%\},\"metadata\"*}"
if [[ "$*" == *"!= null"* ]]; then
  [[ "$data" == *"\"${key}\":"* ]]
  exit $?
fi
if [[ "$*" == *"// empty"* ]]; then
  case "$key" in
    LISTINGKIT_ZITADEL_ALLOWED_ROLES) printf '%s\n' 'bGVnYWN5' ;;
    TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES) printf '\n' ;;
    *) exit 1 ;;
  esac
  exit 0
fi
printf 'unexpected fake jq call: %s\n' "$*" >&2
exit 1
EOF
chmod +x "$temp_dir/jq"
export PATH="$temp_dir:$PATH"
export FAKE_KUBECTL_LOG="$temp_dir/kubectl.log"
export FAKE_KUBECTL_PATCH="$temp_dir/kubectl.patch"
export FAKE_KUBECTL_ROLLOUT="$temp_dir/kubectl.rollout"

"$driver" task-processor listingkit-workbench-secret
patch_call="$(cat "$FAKE_KUBECTL_PATCH")"
[[ "$patch_call" == *'/metadata/resourceVersion'* ]]
[[ "$patch_call" == *'rv-123'* ]]
[[ "$patch_call" == *'/data/LISTINGKIT_ZITADEL_ALLOWED_USERNAMES'* ]]
[[ "$patch_call" == *'/data/TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES'* ]]
[[ "$patch_call" == *'/data/LISTINGKIT_ZITADEL_ALLOWED_ROLES'* ]]
[[ "$patch_call" == *'/data/TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES'* ]]
[[ "$patch_call" == *'bGVnYWN5'* ]]
[[ -e "$FAKE_KUBECTL_ROLLOUT" ]]
patch_line="$(grep -n 'patch secret listingkit-workbench-secret' "$FAKE_KUBECTL_LOG" | cut -d: -f1)"
rollout_line="$(grep -n 'rollout restart deployment/product-listing-api' "$FAKE_KUBECTL_LOG" | cut -d: -f1)"
(( patch_line < rollout_line ))

rm -f "$FAKE_KUBECTL_PATCH"
rm -f "$FAKE_KUBECTL_ROLLOUT"
export FAKE_KUBECTL_MODE=annotation-only
"$driver" task-processor listingkit-workbench-secret
[[ ! -e "$FAKE_KUBECTL_PATCH" ]]
[[ ! -e "$FAKE_KUBECTL_ROLLOUT" ]]

printf '%s\n' 'listingkit legacy identity Secret cleanup tests passed'
