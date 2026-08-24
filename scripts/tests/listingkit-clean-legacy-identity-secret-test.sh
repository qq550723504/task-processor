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
if [[ "$*" == *"get secret listingkit-workbench-secret -o name"* ]]; then
  printf '%s\n' 'secret/listingkit-workbench-secret'
  exit 0
fi
if [[ "$*" == *"get secret listingkit-workbench-secret -o jsonpath="* ]]; then
  if [[ "$*" == *'metadata.resourceVersion'* && "$*" == *'END'* ]]; then
    if [[ "${FAKE_KUBECTL_MODE:-normal}" == "annotation-only" ]]; then
      printf 'rv:opaque/123\n\n\n\n\nEND\n'
    elif [[ "${FAKE_KUBECTL_MODE:-normal}" == "no-deployment" ]]; then
      printf 'rv:opaque/123\nbGVnYWN5\ncHJpbWFyeQ==\nbGVnYWN5\n\nEND\n'
    elif [[ "${FAKE_KUBECTL_MODE:-normal}" == "canonical-whitespace" ]]; then
      printf 'rv:opaque/123\nbGVnYWN5\ncHJpbWFyeQ==\nbGVnYWN5\nIA==\nEND\n'
    else
      printf 'rv:opaque/123\nbGVnYWN5\ncHJpbWFyeQ==\nbGVnYWN5\n\nEND\n'
    fi
    exit 0
  fi
  case "$*" in
    *'.metadata.resourceVersion}'*) printf '%s\n' 'rv:opaque/123' ;;
    *'.data.LISTINGKIT_ZITADEL_ALLOWED_USERNAMES}'*) [[ "${FAKE_KUBECTL_MODE:-normal}" != "annotation-only" ]] && printf '%s\n' 'bGVnYWN5' ;;
    *'.data.TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES}'*) [[ "${FAKE_KUBECTL_MODE:-normal}" != "annotation-only" ]] && printf '%s\n' 'cHJpbWFyeQ==' ;;
    *'.data.LISTINGKIT_ZITADEL_ALLOWED_ROLES}'*) [[ "${FAKE_KUBECTL_MODE:-normal}" != "annotation-only" ]] && printf '%s\n' 'bGVnYWN5' ;;
    *'.data.TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES}'*)
      if [[ "${FAKE_KUBECTL_MODE:-normal}" == "canonical-whitespace" ]]; then
        printf '%s\n' 'IA=='
      elif [[ "${FAKE_KUBECTL_MODE:-normal}" != "annotation-only" ]]; then
        printf '\n'
      fi
      ;;
  esac
  exit 0
fi
if [[ "$*" == *"patch secret listingkit-workbench-secret"* ]]; then
  printf '%s\n' "$*" > "${FAKE_KUBECTL_PATCH:?}"
  exit 0
fi
if [[ "$*" == *"get deployment/product-listing-api -o name"* ]]; then
  if [[ "${FAKE_KUBECTL_MODE:-normal}" == "no-deployment" ]]; then
    exit 1
  fi
  printf '%s\n' 'deployment.apps/product-listing-api'
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
export PATH="$temp_dir:$PATH"
export FAKE_KUBECTL_LOG="$temp_dir/kubectl.log"
export FAKE_KUBECTL_PATCH="$temp_dir/kubectl.patch"
export FAKE_KUBECTL_ROLLOUT="$temp_dir/kubectl.rollout"

"$driver" task-processor listingkit-workbench-secret
patch_call="$(cat "$FAKE_KUBECTL_PATCH")"
[[ "$patch_call" == *'/metadata/resourceVersion'* ]]
[[ "$patch_call" == *'rv:opaque/123'* ]]
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
[[ -e "$FAKE_KUBECTL_ROLLOUT" ]]

rm -f "$FAKE_KUBECTL_PATCH" "$FAKE_KUBECTL_ROLLOUT"
export FAKE_KUBECTL_MODE=canonical-whitespace
"$driver" task-processor listingkit-workbench-secret
patch_call="$(cat "$FAKE_KUBECTL_PATCH")"
[[ "$patch_call" == *'/data/TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES'* ]]
[[ "$patch_call" == *'bGVnYWN5'* ]]
[[ -e "$FAKE_KUBECTL_ROLLOUT" ]]

rm -f "$FAKE_KUBECTL_PATCH" "$FAKE_KUBECTL_ROLLOUT"
export FAKE_KUBECTL_MODE=no-deployment
"$driver" task-processor listingkit-workbench-secret
[[ -e "$FAKE_KUBECTL_PATCH" ]]
[[ ! -e "$FAKE_KUBECTL_ROLLOUT" ]]

printf '%s\n' 'listingkit legacy identity Secret cleanup tests passed'
