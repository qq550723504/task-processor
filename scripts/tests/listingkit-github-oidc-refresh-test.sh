#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
helper="$repo_root/scripts/listingkit-refresh-github-oidc-kubeconfig.sh"

if [[ ! -f "$helper" ]]; then
  printf 'missing checked-in GitHub OIDC refresh helper: %s\n' "$helper" >&2
  exit 1
fi

printf '%s\n' 'GitHub OIDC refresh helper test passed'
