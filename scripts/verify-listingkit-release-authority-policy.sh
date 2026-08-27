#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
docker_repo_root="$repo_root"
if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
  docker_repo_root="$(cd "$repo_root" && pwd -W)"
fi
conftest_image="openpolicyagent/conftest@sha256:5fd81e332d7e4bc01daf3ef35371800a9a9720a30c0c37a78de0c5fbe4b6d622"
policy_dir="policy/listingkit-release-authority"
authority_dir="deployments/kubernetes/listingkit-workbench/release-authority"

run_conftest() {
  MSYS_NO_PATHCONV=1 docker run --rm \
    --volume "${docker_repo_root}:/project" \
    --workdir /project \
    "$conftest_image" "$@"
}

run_conftest test --combine \
  --namespace listingkit_release_authority \
  --policy "$policy_dir" \
  "$authority_dir/release-policy.yaml" \
  "$authority_dir/kubernetes-authentication-config.example.yaml" \
  "$authority_dir/listingkit-api-release-role.yaml" \
  "$authority_dir/listingkit-api-release-rolebinding.yaml" \
  "$authority_dir/listingkit-ui-release-role.yaml" \
  "$authority_dir/listingkit-ui-release-rolebinding.yaml" \
  "$authority_dir/kustomization.yaml" \
  "$authority_dir/listingkit-release-gate-runners.yaml" \
  .github/workflows/listingkit-deploy.yml \
  .github/workflows/listingkit-ui-deploy.yml

for fixture in "$repo_root"/policy/listingkit-release-authority/fixtures/negative/*.yaml; do
  relative="${fixture#"$repo_root"/}"
  if run_conftest test --namespace listingkit_release_fixture --policy "$policy_dir" "$relative" >/dev/null 2>&1; then
    printf 'negative release-authority fixture unexpectedly passed: %s\n' "$relative" >&2
    exit 1
  fi
done

printf 'ListingKit release-authority policy and all negative fixtures passed\n'
