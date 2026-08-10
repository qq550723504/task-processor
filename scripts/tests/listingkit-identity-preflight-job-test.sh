#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
driver="$repo_root/scripts/listingkit-identity-preflight-job.sh"
manifest="$repo_root/deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml"

if [[ ! -x "$driver" ]]; then
  printf 'missing executable release-gate driver: %s\n' "$driver" >&2
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
    */listingkit-identity-preflight.*.yaml)
      normalized+=("<rendered-manifest>")
      ;;
    *)
      normalized+=("$argument")
      ;;
  esac
done
(IFS='|'; printf '%s\n' "${normalized[*]}") >>"$FAKE_KUBECTL_LOG"

if [[ "$1" == "create" ]]; then
  if [[ -n "${FAKE_RENDERED_PATH_LOG:-}" ]]; then
    printf '%s\n' "$5" >"$FAKE_RENDERED_PATH_LOG"
  fi
  cp "$5" "$FAKE_RENDERED_MANIFEST"
  printf '%s' 'listingkit-identity-preflight-test-abc12'
  exit 0
fi

if [[ "${1:-}" == "-n" && "${3:-}" == "wait" ]]; then
  exit "${FAKE_KUBECTL_WAIT_STATUS:-0}"
fi

if [[ "${1:-}" == "-n" && "${3:-}" == "logs" ]]; then
  printf '%s\n' 'identity preflight logs'
  exit 0
fi

if [[ "${1:-}" == "-n" && "${3:-}" == "describe" ]]; then
  printf '%s\n' 'identity preflight description'
  exit 0
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
    printf 'unexpected command log in %s\nexpected:\n%s\nactual:\n' "$actual_file" "$expected" >&2
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

run_success_case() {
  local command_log="$test_root/success.commands"
  local rendered_manifest="$test_root/success.rendered.yaml"
  local rendered_path_log="$test_root/success.rendered-path"
  local output

  output="$({
    PATH="$fake_bin:$PATH" \
      FAKE_KUBECTL_LOG="$command_log" \
      FAKE_RENDERED_MANIFEST="$rendered_manifest" \
      FAKE_RENDERED_PATH_LOG="$rendered_path_log" \
      FAKE_KUBECTL_WAIT_STATUS=0 \
      "$driver" \
        --manifest "$manifest" \
        --namespace test-namespace \
        --image docker.io/alternate-registry/task-processor-product-listing-api:release-20260810-abc123
  } 2>&1)"

  local expected_commands
  expected_commands=$'create|-n|test-namespace|-f|<rendered-manifest>|-o|jsonpath={.metadata.name}\n-n|test-namespace|wait|--for=condition=complete|job/listingkit-identity-preflight-test-abc12|--timeout=15m\n-n|test-namespace|logs|job/listingkit-identity-preflight-test-abc12'
  assert_file_equals "$expected_commands" "$command_log"
  assert_contains "$(cat "$rendered_manifest")" 'docker.io/alternate-registry/task-processor-product-listing-api:release-20260810-abc123'
  assert_not_contains "$(cat "$rendered_manifest")" 'REPLACE_WITH_DEPLOYED_IMAGE'
  assert_contains "$output" 'identity preflight logs'
  if [[ -e "$(cat "$rendered_path_log")" ]]; then
    printf 'expected temporary rendered manifest to be removed: %s\n' "$(cat "$rendered_path_log")" >&2
    exit 1
  fi
}

run_failure_case() {
  local command_log="$test_root/failure.commands"
  local rendered_manifest="$test_root/failure.rendered.yaml"
  local output
  local status

  set +e
  output="$({
    PATH="$fake_bin:$PATH" \
      FAKE_KUBECTL_LOG="$command_log" \
      FAKE_RENDERED_MANIFEST="$rendered_manifest" \
      FAKE_KUBECTL_WAIT_STATUS=1 \
      "$driver" \
        --manifest "$manifest" \
        --namespace test-namespace \
        --image docker.io/alternate-registry/task-processor-product-listing-api:release-20260810-abc123
  } 2>&1)"
  status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    printf 'expected failed or timed-out Job to exit non-zero\n' >&2
    exit 1
  fi

  local expected_commands
  expected_commands=$'create|-n|test-namespace|-f|<rendered-manifest>|-o|jsonpath={.metadata.name}\n-n|test-namespace|wait|--for=condition=complete|job/listingkit-identity-preflight-test-abc12|--timeout=15m\n-n|test-namespace|logs|job/listingkit-identity-preflight-test-abc12\n-n|test-namespace|describe|job/listingkit-identity-preflight-test-abc12'
  assert_file_equals "$expected_commands" "$command_log"
  assert_contains "$output" 'identity preflight logs'
  assert_contains "$output" 'identity preflight description'
  assert_not_contains "$(cat "$command_log")" 'set|image'
}

run_mutable_image_case() {
  local command_log="$test_root/mutable-image.commands"
  local rendered_manifest="$test_root/mutable-image.rendered.yaml"
  local status

  set +e
  PATH="$fake_bin:$PATH" \
    FAKE_KUBECTL_LOG="$command_log" \
    FAKE_RENDERED_MANIFEST="$rendered_manifest" \
    FAKE_KUBECTL_WAIT_STATUS=0 \
    "$driver" \
      --manifest "$manifest" \
      --namespace test-namespace \
      --image docker.io/alternate-registry/task-processor-product-listing-api:latest >/dev/null 2>&1
  status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    printf 'expected mutable latest image to exit non-zero\n' >&2
    exit 1
  fi
  if [[ -s "$command_log" ]]; then
    printf 'expected mutable latest image to be rejected before kubectl, got:\n' >&2
    cat "$command_log" >&2
    exit 1
  fi
}

run_success_case
run_failure_case
run_mutable_image_case
printf '%s\n' 'listingkit identity preflight Job driver tests passed'
