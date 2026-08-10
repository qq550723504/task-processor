#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$script_dir/listingkit-apply-api-deployment.sh" \
  --container listingkit-ui \
  --deployment listingkit-ui \
  --enforce-env-from-configmap listingkit-workbench-config \
  "$@"
