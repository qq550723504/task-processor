#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/listingkit-immutable-image.sh
source "$script_dir/lib/listingkit-immutable-image.sh"

usage() {
  printf 'Usage: %s --manifest PATH --namespace NAMESPACE --image IMMUTABLE_IMAGE\n' "$0" >&2
}

MANIFEST=""
K8S_NAMESPACE=""
IMAGE=""

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
  printf 'ListingKit API Deployment manifest not found: %s\n' "$MANIFEST" >&2
  exit 2
fi

if ! listingkit_is_immutable_image "$IMAGE"; then
  printf 'ListingKit API deployment requires a valid immutable image, got: %s\n' "$IMAGE" >&2
  exit 2
fi

rendered_manifest="$(mktemp "${TMPDIR:-/tmp}/listingkit-api-deployment.XXXXXX.yaml")"
trap 'rm -f "$rendered_manifest"' EXIT

if ! awk -v image="$IMAGE" '
  /^[[:space:]]*- name:[[:space:]]*product-listing-api[[:space:]]*$/ {
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
  printf 'ListingKit API Deployment manifest must contain exactly one product-listing-api container image: %s\n' "$MANIFEST" >&2
  exit 2
fi

kubectl -n "$K8S_NAMESPACE" apply -f "$rendered_manifest"
