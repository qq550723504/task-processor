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
  printf '%s\n' '{"data":{"LISTINGKIT_ZITADEL_ALLOWED_USERNAMES":"bGVnYWN5","LISTINGKIT_ZITADEL_ALLOWED_ROLES":"bGVnYWN5","TASK_PROCESSOR_DATABASE_HOST":"c2VydmljZQ=="}}'
  exit 0
fi
if [[ "$*" == *"patch secret listingkit-workbench-secret"* ]]; then
  printf '%s\n' "$*" > "${FAKE_KUBECTL_PATCH:?}"
  exit 0
fi
printf 'unexpected fake kubectl call: %s\n' "$*" >&2
exit 1
EOF
chmod +x "$temp_dir/kubectl"
export PATH="$temp_dir:$PATH"
export FAKE_KUBECTL_LOG="$temp_dir/kubectl.log"
export FAKE_KUBECTL_PATCH="$temp_dir/kubectl.patch"

"$driver" task-processor listingkit-workbench-secret
patch_call="$(cat "$FAKE_KUBECTL_PATCH")"
[[ "$patch_call" == *'/data/LISTINGKIT_ZITADEL_ALLOWED_USERNAMES'* ]]
[[ "$patch_call" == *'/data/LISTINGKIT_ZITADEL_ALLOWED_ROLES'* ]]
[[ "$patch_call" != *'bGVnYWN5'* ]]

printf '%s\n' 'listingkit legacy identity Secret cleanup tests passed'
