#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
driver="$repo_root/scripts/listingkit-apply-api-deployment.sh"
manifest="$repo_root/deployments/kubernetes/listingkit-workbench/base/product-listing-api-deployment.yaml"

if [[ ! -x "$driver" ]]; then
  printf 'missing executable immutable deployment driver: %s\n' "$driver" >&2
  exit 1
fi

test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT

fake_bin="$test_root/bin"
mkdir -p "$fake_bin"

cat >"$fake_bin/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

normalized=()
for argument in "$@"; do
  case "$argument" in
    */listingkit-api-deployment.*.yaml)
      normalized+=("<rendered-manifest>")
      ;;
    *)
      normalized+=("$argument")
      ;;
  esac
done
(IFS='|'; printf '%s\n' "${normalized[*]}") >>"$FAKE_KUBECTL_LOG"

if [[ "${1:-}" == "-n" && "${3:-}" == "apply" && "${4:-}" == "-f" ]]; then
  printf '%s\n' "$5" >"$FAKE_RENDERED_PATH_LOG"
  cp "$5" "$FAKE_RENDERED_MANIFEST"
  exit "${FAKE_KUBECTL_APPLY_STATUS:-0}"
fi

printf 'unexpected kubectl invocation:' >&2
printf ' %q' "$@" >&2
printf '\n' >&2
exit 97
EOF
chmod +x "$fake_bin/kubectl"

assert_file_equals() {
  local expected="$1"
  local actual_file="$2"

  if [[ "$(cat "$actual_file")" != "$expected" ]]; then
    printf 'unexpected file content in %s\nexpected:\n%s\nactual:\n' "$actual_file" "$expected" >&2
    cat "$actual_file" >&2
    exit 1
  fi
}

assert_contains() {
  local haystack="$1"
  local needle="$2"

  if [[ "$haystack" != *"$needle"* ]]; then
    printf 'expected output to contain %q, got:\n%s\n' "$needle" "$haystack" >&2
    exit 1
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"

  if [[ "$haystack" == *"$needle"* ]]; then
    printf 'expected output not to contain %q, got:\n%s\n' "$needle" "$haystack" >&2
    exit 1
  fi
}

assert_temporary_manifest_removed() {
  local rendered_path_log="$1"
  local rendered_path

  rendered_path="$(cat "$rendered_path_log")"
  if [[ -e "$rendered_path" ]]; then
    printf 'expected temporary rendered manifest to be removed: %s\n' "$rendered_path" >&2
    exit 1
  fi
}

run_success_case() {
  local command_log="$test_root/success.commands"
  local rendered_manifest="$test_root/success.rendered.yaml"
  local rendered_path_log="$test_root/success.rendered-path"
  local immutable_image='docker.io/xuwei190/task-processor-product-listing-api:release-20260810-abc123'

  PATH="$fake_bin:$PATH" \
    FAKE_KUBECTL_LOG="$command_log" \
    FAKE_RENDERED_MANIFEST="$rendered_manifest" \
    FAKE_RENDERED_PATH_LOG="$rendered_path_log" \
    FAKE_KUBECTL_APPLY_STATUS=0 \
    "$driver" \
      --manifest "$manifest" \
      --namespace test-namespace \
      --image "$immutable_image"

  assert_file_equals '-n|test-namespace|apply|-f|<rendered-manifest>' "$command_log"
  assert_contains "$(cat "$rendered_manifest")" "image: $immutable_image"
  assert_not_contains "$(cat "$rendered_manifest")" 'task-processor-product-listing-api:latest'
  assert_temporary_manifest_removed "$rendered_path_log"
}

run_apply_failure_case() {
  local command_log="$test_root/apply-failure.commands"
  local rendered_manifest="$test_root/apply-failure.rendered.yaml"
  local rendered_path_log="$test_root/apply-failure.rendered-path"
  local status

  set +e
  PATH="$fake_bin:$PATH" \
    FAKE_KUBECTL_LOG="$command_log" \
    FAKE_RENDERED_MANIFEST="$rendered_manifest" \
    FAKE_RENDERED_PATH_LOG="$rendered_path_log" \
    FAKE_KUBECTL_APPLY_STATUS=1 \
    "$driver" \
      --manifest "$manifest" \
      --namespace test-namespace \
      --image docker.io/xuwei190/task-processor-product-listing-api:release-20260810-abc123 \
      >/dev/null 2>&1
  status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    printf 'expected failed apply to exit non-zero\n' >&2
    exit 1
  fi
  assert_file_equals '-n|test-namespace|apply|-f|<rendered-manifest>' "$command_log"
  assert_temporary_manifest_removed "$rendered_path_log"
}

run_invalid_image_case() {
  local image="$1"
  local label="$2"
  local command_log="$test_root/$label.commands"
  local status
  local output

  set +e
  output="$({
    PATH="$fake_bin:$PATH" \
      FAKE_KUBECTL_LOG="$command_log" \
      FAKE_RENDERED_MANIFEST="$test_root/$label.rendered.yaml" \
      FAKE_RENDERED_PATH_LOG="$test_root/$label.rendered-path" \
      FAKE_KUBECTL_APPLY_STATUS=0 \
      "$driver" \
        --manifest "$manifest" \
        --namespace test-namespace \
        --image "$image"
  } 2>&1)"
  status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    printf 'expected invalid image %q to exit non-zero\n' "$image" >&2
    exit 1
  fi
  assert_contains "$output" 'immutable image'
  if [[ -s "$command_log" ]]; then
    printf 'expected invalid image %q to be rejected before kubectl, got:\n' "$image" >&2
    cat "$command_log" >&2
    exit 1
  fi
}

run_success_case
run_apply_failure_case
run_invalid_image_case '' empty-image
run_invalid_image_case 'docker.io/xuwei190/task-processor-product-listing-api:latest' latest-image
printf '%s\n' 'ListingKit immutable API deployment driver tests passed'
