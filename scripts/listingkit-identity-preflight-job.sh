#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/listingkit-immutable-image.sh
source "$script_dir/lib/listingkit-immutable-image.sh"

usage() {
  printf 'Usage: %s --manifest PATH --namespace NAMESPACE --image API_CANDIDATE_DIGEST --runner-image PREFLIGHT_RUNNER_DIGEST\n' "$0" >&2
}

MANIFEST=""
K8S_NAMESPACE=""
IMAGE=""
RUNNER_IMAGE=""

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --manifest)
      [[ "$#" -ge 2 ]] || { usage; exit 2; }
      MANIFEST="$2"
      shift 2
      ;;
    --namespace)
      [[ "$#" -ge 2 ]] || { usage; exit 2; }
      K8S_NAMESPACE="$2"
      shift 2
      ;;
    --image)
      [[ "$#" -ge 2 ]] || { usage; exit 2; }
      IMAGE="$2"
      shift 2
      ;;
    --runner-image)
      [[ "$#" -ge 2 ]] || { usage; exit 2; }
      RUNNER_IMAGE="$2"
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$MANIFEST" || -z "$K8S_NAMESPACE" || -z "$IMAGE" || -z "$RUNNER_IMAGE" ]]; then
  usage
  exit 2
fi

if [[ ! -f "$MANIFEST" ]]; then
  printf 'Identity preflight Job manifest not found: %s\n' "$MANIFEST" >&2
  exit 2
fi

if ! listingkit_is_immutable_image "$IMAGE"; then
  printf 'Identity preflight requires a valid immutable image, got: %s\n' "$IMAGE" >&2
  exit 2
fi

if ! listingkit_is_immutable_image "$RUNNER_IMAGE"; then
  printf 'Identity preflight requires a valid digest-pinned runner image, got: %s\n' "$RUNNER_IMAGE" >&2
  exit 2
fi

if ! grep -q 'REPLACE_WITH_API_CANDIDATE_IMAGE' "$MANIFEST" || ! grep -q 'REPLACE_WITH_PREFLIGHT_RUNNER_IMAGE' "$MANIFEST"; then
  printf 'Identity preflight Job manifest is missing a required image placeholder: %s\n' "$MANIFEST" >&2
  exit 2
fi

rendered_manifest="$(mktemp "${TMPDIR:-/tmp}/listingkit-identity-preflight.XXXXXX.yaml")"
trap 'rm -f "$rendered_manifest"' EXIT

sed \
  -e "s|REPLACE_WITH_API_CANDIDATE_IMAGE|$IMAGE|g" \
  -e "s|REPLACE_WITH_PREFLIGHT_RUNNER_IMAGE|$RUNNER_IMAGE|g" \
  "$MANIFEST" >"$rendered_manifest"

job_name="$(kubectl create -n "$K8S_NAMESPACE" -f "$rendered_manifest" -o 'jsonpath={.metadata.name}')"
if [[ -z "$job_name" ]]; then
  printf 'kubectl created the identity preflight Job without returning its name\n' >&2
  exit 1
fi

wait_deadline=$((SECONDS + 900))
while :; do
  if job_status="$(kubectl -n "$K8S_NAMESPACE" get "job/$job_name" -o 'jsonpath={.status.succeeded},{.status.failed}' 2>/dev/null)"; then
    succeeded_count="${job_status%,*}"
    failed_count="${job_status#*,}"
    if [[ "$succeeded_count" =~ ^[1-9][0-9]*$ ]]; then
      kubectl -n "$K8S_NAMESPACE" logs "job/$job_name"
      exit 0
    fi
    if [[ "$failed_count" =~ ^[1-9][0-9]*$ ]]; then
      kubectl -n "$K8S_NAMESPACE" logs "job/$job_name" || true
      kubectl -n "$K8S_NAMESPACE" describe "job/$job_name" || true
      exit 1
    fi
  fi

  if (( SECONDS >= wait_deadline )); then
    kubectl -n "$K8S_NAMESPACE" logs "job/$job_name" || true
    kubectl -n "$K8S_NAMESPACE" describe "job/$job_name" || true
    exit 1
  fi
  sleep 5
done
