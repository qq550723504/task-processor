#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$script_dir/lib/listingkit-immutable-image.sh"

usage() {
  printf 'Usage: %s --namespace NAMESPACE --manifest PATH --deployment NAME --image DIGEST_IMAGE --timeout-seconds N\n' "$0" >&2
}

namespace=""
manifest=""
deployment=""
image=""
timeout_seconds=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace) namespace="${2:-}"; shift 2 ;;
    --manifest) manifest="${2:-}"; shift 2 ;;
    --deployment) deployment="${2:-}"; shift 2 ;;
    --image) image="${2:-}"; shift 2 ;;
    --timeout-seconds) timeout_seconds="${2:-}"; shift 2 ;;
    *) usage; exit 2 ;;
  esac
done

if [[ -z "$namespace" || -z "$manifest" || -z "$deployment" || -z "$image" || -z "$timeout_seconds" ]]; then
  usage
  exit 2
fi
for value in "$namespace" "$deployment"; do
  if [[ ! "$value" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
    printf 'release-gate Kubernetes names must be DNS labels\n' >&2
    exit 2
  fi
done
if [[ ! -f "$manifest" || ! -r "$manifest" ]]; then
  printf 'release-gate runner manifest must be a readable file\n' >&2
  exit 2
fi
if [[ ! "$timeout_seconds" =~ ^[1-9][0-9]*$ ]] || (( timeout_seconds > 1800 )); then
  printf 'release-gate timeout must be between 1 and 1800 seconds\n' >&2
  exit 2
fi
if ! listingkit_is_immutable_image "$image"; then
  printf 'release-gate runner requires a digest-pinned image\n' >&2
  exit 2
fi
for command_name in kubectl jq cmp mktemp; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf 'release-gate runner requires %s\n' "$command_name" >&2
    exit 1
  fi
done

hold_image="registry.k8s.io/pause@sha256:ee6521f290b2168b6e0935a181d4cff9be1ac3f505666ef0e3c98fae8199917a"
temporary_dir="$(mktemp -d)"
selected_manifest="$temporary_dir/selected.json"
image_patch="$temporary_dir/image-patch.json"
expected_deployment="$temporary_dir/expected.json"
expected_canonical="$temporary_dir/expected-canonical.json"
live_deployment="$temporary_dir/live.json"
live_canonical="$temporary_dir/live-canonical.json"
pods_json="$temporary_dir/pods.json"

cleanup() {
  local status=$?
  trap - EXIT
  kubectl -n "$namespace" scale "deployment/$deployment" --replicas=0 >/dev/null 2>&1 || true
  rm -rf "$temporary_dir"
  exit "$status"
}
trap cleanup EXIT

if ! kubectl -n "$namespace" create --dry-run=client --validate=false -f "$manifest" -o json |
  jq -e -s \
    --arg namespace "$namespace" \
    --arg deployment "$deployment" \
    --arg hold_image "$hold_image" '
      # listingkit-runner-select-v1
      [
        .[] |
        (if .kind == "List" and (.items | type) == "array" then .items[] else . end) |
        select(
          .apiVersion == "apps/v1" and
          .kind == "Deployment" and
          .metadata.name == $deployment
        )
      ] as $matches |
      if ($matches | length) != 1 then
        error("reviewed runner manifest must contain exactly one named Deployment")
      else
        $matches[0]
      end |
      if (
        ((.metadata.namespace // $namespace) == $namespace) and
        (.spec.replicas == 0) and
        (.spec.selector.matchLabels | type == "object" and length > 0) and
        (.spec.template.spec.automountServiceAccountToken == false) and
        ((.spec.template.spec.initContainers | length) == 1) and
        (.spec.template.spec.initContainers[0].name == "release-gate") and
        ((.spec.template.spec.initContainers[0].command // []) | type == "array" and length > 0) and
        ((.spec.template.spec.containers | length) == 1) and
        (.spec.template.spec.containers[0].name == "hold-after-gate") and
        (.spec.template.spec.containers[0].image == $hold_image)
      ) then . else
        error("reviewed runner Deployment has an invalid one-shot shape")
      end
    ' >"$selected_manifest"; then
  printf 'could not select an exact reviewed release-gate Deployment\n' >&2
  exit 1
fi

if ! kubectl -n "$namespace" get "deployment/$deployment" -o json >"$live_deployment" 2>/dev/null; then
  printf 'preinstalled release-gate Deployment is missing\n' >&2
  exit 1
fi

kubectl -n "$namespace" scale "deployment/$deployment" --replicas=0 >/dev/null
kubectl -n "$namespace" apply -f "$selected_manifest" >/dev/null
jq -n --arg image "$image" '
  # listingkit-runner-image-patch-v1
  [{"op":"replace","path":"/spec/template/spec/initContainers/0/image","value":$image}]
' >"$image_patch"
kubectl -n "$namespace" patch "deployment/$deployment" --type=json --patch-file "$image_patch" >/dev/null
kubectl -n "$namespace" scale "deployment/$deployment" --replicas=1 >/dev/null

jq \
  --arg namespace "$namespace" \
  --arg image "$image" '
    # listingkit-runner-expected-v1
    .metadata.namespace = $namespace |
    .spec.replicas = 1 |
    .spec.template.spec.initContainers[0].image = $image
  ' "$selected_manifest" >"$expected_deployment"

canonical_filter='
  # listingkit-runner-canonical-v1
  def canonical_container:
    {
      name: .name,
      image: .image,
      imagePullPolicy: .imagePullPolicy,
      command: (.command // []),
      args: (.args // []),
      env: (.env // []),
      envFrom: (.envFrom // []),
      volumeMounts: (.volumeMounts // []),
      securityContext: (.securityContext // {}),
      resources: (.resources // {})
    };
  {
    apiVersion: .apiVersion,
    kind: .kind,
    metadata: {
      name: .metadata.name,
      namespace: (.metadata.namespace // $namespace),
      labels: (.metadata.labels // {})
    },
    spec: {
      replicas: .spec.replicas,
      revisionHistoryLimit: (.spec.revisionHistoryLimit // null),
      progressDeadlineSeconds: (.spec.progressDeadlineSeconds // null),
      strategy: .spec.strategy,
      selector: .spec.selector,
      template: {
        metadata: {labels: (.spec.template.metadata.labels // {})},
        spec: {
          automountServiceAccountToken: .spec.template.spec.automountServiceAccountToken,
          serviceAccountName: (.spec.template.spec.serviceAccountName // "default"),
          securityContext: (.spec.template.spec.securityContext // {}),
          imagePullSecrets: (.spec.template.spec.imagePullSecrets // []),
          initContainers: [
            .spec.template.spec.initContainers[]? | canonical_container
          ],
          containers: [
            .spec.template.spec.containers[]? | canonical_container
          ],
          volumes: (.spec.template.spec.volumes // [])
        }
      }
    }
  }
'
jq -S --arg namespace "$namespace" "$canonical_filter" "$expected_deployment" >"$expected_canonical"

selector="$(jq -r '
  # listingkit-runner-selector-v1
  (.spec.selector.matchLabels // {}) |
  to_entries |
  sort_by(.key) |
  map("\(.key)=\(.value)") |
  join(",")
' "$expected_deployment")"
if [[ -z "$selector" ]]; then
  printf 'reviewed release-gate Deployment has no exact Pod selector\n' >&2
  exit 1
fi

deadline=$((SECONDS + timeout_seconds))
while (( SECONDS < deadline )); do
  if ! kubectl -n "$namespace" get "deployment/$deployment" -o json >"$live_deployment" 2>/dev/null; then
    printf 'live release-gate Deployment disappeared\n' >&2
    exit 1
  fi
  jq -S --arg namespace "$namespace" "$canonical_filter" "$live_deployment" >"$live_canonical"
  if ! cmp -s "$expected_canonical" "$live_canonical"; then
    printf 'live release-gate runner contract differs from reviewed manifest\n' >&2
    exit 1
  fi

  if jq -e '
    # listingkit-runner-available-v1
    ((.status.observedGeneration // -1) >= (.metadata.generation // 0)) and
    (.spec.replicas == 1) and
    (.status.updatedReplicas == 1) and
    (.status.availableReplicas == 1) and
    ((.status.unavailableReplicas // 0) == 0)
  ' "$live_deployment" >/dev/null; then
    if kubectl -n "$namespace" get pods -l "$selector" -o json >"$pods_json" 2>/dev/null; then
      init_result="$(jq -r \
        --arg image "$image" \
        --arg hold_image "$hold_image" '
          # listingkit-runner-init-result-v1
          .items as $items |
          if (($items | type) != "array" or ($items | length) != 1) then
            "pending"
          else
            $items[0] as $pod |
            [$pod.spec.initContainers[]? | select(.name == "release-gate")] as $init_specs |
            [$pod.spec.containers[]? | select(.name == "hold-after-gate")] as $hold_specs |
            [$pod.status.initContainerStatuses[]? | select(.name == "release-gate")] as $init_statuses |
            [$pod.status.containerStatuses[]? | select(.name == "hold-after-gate")] as $hold_statuses |
            ($init_statuses[0].state.terminated // null) as $terminated |
            if (
              ($pod.metadata.deletionTimestamp // null) == null and
              ($init_specs | length) == 1 and
              $init_specs[0].image == $image and
              ($hold_specs | length) == 1 and
              $hold_specs[0].image == $hold_image and
              ($init_statuses | length) == 1 and
              $terminated != null
            ) then
              if (
                $terminated.exitCode == 0 and
                $terminated.reason == "Completed" and
                ($hold_statuses | length) == 1 and
                $hold_statuses[0].ready == true
              ) then "success" else "failed" end
            else
              "pending"
            end
          end
        ' "$pods_json")"
      case "$init_result" in
        success)
          printf 'release gate %s completed with reviewed contract and image %s\n' "$deployment" "$image"
          exit 0
          ;;
        failed)
          printf 'release-gate init container terminated unsuccessfully\n' >&2
          exit 1
          ;;
      esac
    fi
  fi
  sleep 5
done

kubectl -n "$namespace" describe "deployment/$deployment" >&2 || true
printf 'release gate %s did not complete within %s seconds\n' "$deployment" "$timeout_seconds" >&2
exit 1
