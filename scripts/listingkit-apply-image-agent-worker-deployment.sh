#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

container_is_explicit=false
for argument in "$@"; do
  if [[ "$argument" == "--container" ]]; then
    container_is_explicit=true
    break
  fi
done

if [[ "$container_is_explicit" != "true" ]]; then
  printf 'Image-agent workload apply requires an explicit --container name\n' >&2
  exit 2
fi

exec "$script_dir/listingkit-apply-api-deployment.sh" "$@"
