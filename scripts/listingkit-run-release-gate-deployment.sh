#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$script_dir/lib/listingkit-immutable-image.sh"

usage() {
  printf 'Usage: %s --namespace NAMESPACE --deployment NAME --container NAME --image DIGEST_IMAGE --timeout-seconds N\n' "$0" >&2
}

namespace=""
deployment=""
container=""
image=""
timeout_seconds=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace) namespace="${2:-}"; shift 2 ;;
    --deployment) deployment="${2:-}"; shift 2 ;;
    --container) container="${2:-}"; shift 2 ;;
    --image) image="${2:-}"; shift 2 ;;
    --timeout-seconds) timeout_seconds="${2:-}"; shift 2 ;;
    *) usage; exit 2 ;;
  esac
done

if [[ -z "$namespace" || -z "$deployment" || -z "$container" || -z "$image" || -z "$timeout_seconds" ]]; then
  usage
  exit 2
fi
for value in "$namespace" "$deployment" "$container"; do
  if [[ ! "$value" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
    printf 'release-gate Kubernetes names must be DNS labels\n' >&2
    exit 2
  fi
done
if [[ ! "$timeout_seconds" =~ ^[1-9][0-9]*$ ]] || (( timeout_seconds > 1800 )); then
  printf 'release-gate timeout must be between 1 and 1800 seconds\n' >&2
  exit 2
fi
if ! listingkit_is_immutable_image "$image"; then
  printf 'release-gate runner requires a digest-pinned image\n' >&2
  exit 2
fi
if [[ ! "${GITHUB_RUN_ID:-}" =~ ^[1-9][0-9]*$ || ! "${GITHUB_RUN_ATTEMPT:-}" =~ ^[1-9][0-9]*$ ]]; then
  printf 'release-gate runner requires GitHub run identity\n' >&2
  exit 1
fi

cleanup() {
  kubectl -n "$namespace" scale "deployment/$deployment" --replicas=0 >/dev/null 2>&1 || true
}
trap cleanup EXIT

kubectl -n "$namespace" scale "deployment/$deployment" --replicas=0 >/dev/null
kubectl -n "$namespace" set image "deployment/$deployment" "$container=$image" >/dev/null
patch="{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"listingkit.io/release-run-id\":\"${GITHUB_RUN_ID}\",\"listingkit.io/release-run-attempt\":\"${GITHUB_RUN_ATTEMPT}\",\"listingkit.io/release-image\":\"${image}\"}}}}}"
kubectl -n "$namespace" patch "deployment/$deployment" --type=merge --patch "$patch" >/dev/null
kubectl -n "$namespace" scale "deployment/$deployment" --replicas=1 >/dev/null

deadline=$((SECONDS + timeout_seconds))
while (( SECONDS < deadline )); do
  status="$(kubectl -n "$namespace" get "deployment/$deployment" -o 'jsonpath={.metadata.generation}{" "}{.status.observedGeneration}{" "}{.spec.replicas}{" "}{.status.updatedReplicas}{" "}{.status.availableReplicas}{" "}{.status.unavailableReplicas}' 2>/dev/null || true)"
  read -r generation observed desired updated available unavailable <<<"$status"
  if [[ "$generation" =~ ^[0-9]+$ && "$observed" =~ ^[0-9]+$ ]] &&
    (( observed >= generation )) && [[ "$desired" == "1" && "$updated" == "1" && "$available" == "1" ]] &&
    [[ -z "$unavailable" || "$unavailable" == "0" ]]; then
    printf 'release gate %s completed with image %s\n' "$deployment" "$image"
    exit 0
  fi
  sleep 5
done

kubectl -n "$namespace" describe "deployment/$deployment" >&2 || true
printf 'release gate %s did not complete within %s seconds\n' "$deployment" "$timeout_seconds" >&2
exit 1
