#!/usr/bin/env bash

set -euo pipefail

usage() {
  printf 'Usage: %s --manifest PATH --namespace NAMESPACE --image-tag IMMUTABLE_TAG\n' "$0" >&2
}

MANIFEST=""
K8S_NAMESPACE=""
IMAGE_TAG=""

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
    --image-tag)
      [[ "$#" -ge 2 ]] || { usage; exit 2; }
      IMAGE_TAG="$2"
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$MANIFEST" || -z "$K8S_NAMESPACE" || -z "$IMAGE_TAG" ]]; then
  usage
  exit 2
fi

if [[ ! -f "$MANIFEST" ]]; then
  printf 'Identity preflight Job manifest not found: %s\n' "$MANIFEST" >&2
  exit 2
fi

if [[ "$IMAGE_TAG" == "latest" || ! "$IMAGE_TAG" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]; then
  printf 'Identity preflight requires a valid immutable image tag, got: %s\n' "$IMAGE_TAG" >&2
  exit 2
fi

if ! grep -q 'REPLACE_WITH_DEPLOYED_TAG' "$MANIFEST"; then
  printf 'Identity preflight Job manifest is missing the image tag placeholder: %s\n' "$MANIFEST" >&2
  exit 2
fi

rendered_manifest="$(mktemp "${TMPDIR:-/tmp}/listingkit-identity-preflight.XXXXXX.yaml")"
trap 'rm -f "$rendered_manifest"' EXIT

sed "s/REPLACE_WITH_DEPLOYED_TAG/$IMAGE_TAG/g" "$MANIFEST" >"$rendered_manifest"

job_name="$(kubectl create -n "$K8S_NAMESPACE" -f "$rendered_manifest" -o 'jsonpath={.metadata.name}')"
if [[ -z "$job_name" ]]; then
  printf 'kubectl created the identity preflight Job without returning its name\n' >&2
  exit 1
fi

if ! kubectl -n "$K8S_NAMESPACE" wait --for=condition=complete "job/$job_name" --timeout=15m; then
  kubectl -n "$K8S_NAMESPACE" logs "job/$job_name" || true
  kubectl -n "$K8S_NAMESPACE" describe "job/$job_name" || true
  exit 1
fi

kubectl -n "$K8S_NAMESPACE" logs "job/$job_name"
