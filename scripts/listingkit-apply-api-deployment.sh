#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/listingkit-immutable-image.sh
source "$script_dir/lib/listingkit-immutable-image.sh"

usage() {
  printf 'Usage: %s --manifest PATH --namespace NAMESPACE --image IMMUTABLE_IMAGE [--container NAME] [--deployment NAME --enforce-env-from-configmap NAME]\n' "$0" >&2
}

MANIFEST=""
K8S_NAMESPACE=""
IMAGE=""
CONTAINER="product-listing-api"
DEPLOYMENT=""
ENV_FROM_CONFIGMAP=""

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
    --container)
      [[ "$#" -ge 2 ]] || { usage; exit 2; }
      CONTAINER="$2"
      shift 2
      ;;
    --deployment)
      [[ "$#" -ge 2 ]] || { usage; exit 2; }
      DEPLOYMENT="$2"
      shift 2
      ;;
    --enforce-env-from-configmap)
      [[ "$#" -ge 2 ]] || { usage; exit 2; }
      ENV_FROM_CONFIGMAP="$2"
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$MANIFEST" || -z "$K8S_NAMESPACE" ]]; then
  usage
  exit 2
fi

if [[ ! -f "$MANIFEST" ]]; then
  printf 'ListingKit Deployment manifest not found: %s\n' "$MANIFEST" >&2
  exit 2
fi

if ! listingkit_is_immutable_image "$IMAGE"; then
  printf 'ListingKit Deployment requires a valid immutable image, got: %s\n' "$IMAGE" >&2
  exit 2
fi

if [[ -n "$DEPLOYMENT" || -n "$ENV_FROM_CONFIGMAP" ]] && [[ -z "$DEPLOYMENT" || -z "$ENV_FROM_CONFIGMAP" ]]; then
  printf 'ListingKit Deployment envFrom enforcement requires both --deployment and --enforce-env-from-configmap\n' >&2
  exit 2
fi

if [[ ! "$CONTAINER" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] ||
  { [[ -n "$DEPLOYMENT" ]] && [[ ! "$DEPLOYMENT" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; } ||
  { [[ -n "$ENV_FROM_CONFIGMAP" ]] && [[ ! "$ENV_FROM_CONFIGMAP" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; }; then
  printf 'ListingKit Deployment container and Kubernetes resource names must be DNS labels\n' >&2
  exit 2
fi

rendered_manifest="$(mktemp "${TMPDIR:-/tmp}/listingkit-deployment.XXXXXX.yaml")"
trap 'rm -f "$rendered_manifest"' EXIT

if ! awk -v image="$IMAGE" -v container="$CONTAINER" '
  $0 ~ "^[[:space:]]*- name:[[:space:]]*" container "[[:space:]]*$" {
    in_target_container = 1
    print
    next
  }
  in_target_container && /^[[:space:]]*image:[[:space:]]*/ {
    sub(/image:[[:space:]].*$/, "image: " image)
    replacements++
    in_target_container = 0
    print
    next
  }
  { print }
  END { if (replacements != 1) exit 42 }
' "$MANIFEST" >"$rendered_manifest"; then
  printf 'ListingKit Deployment manifest must contain exactly one %s container image: %s\n' "$CONTAINER" "$MANIFEST" >&2
  exit 2
fi

if [[ -n "$ENV_FROM_CONFIGMAP" ]]; then
  env_from_patch="$(printf '{"spec":{"template":{"spec":{"containers":[{"name":"%s","envFrom":[{"configMapRef":{"name":"%s"}}]}]}}}}' "$CONTAINER" "$ENV_FROM_CONFIGMAP")"
  kubectl -n "$K8S_NAMESPACE" patch deployment "$DEPLOYMENT" --type=strategic --patch "$env_from_patch"
fi

kubectl -n "$K8S_NAMESPACE" apply -f "$rendered_manifest"
