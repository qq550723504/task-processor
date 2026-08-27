#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
usage: verify-listingkit-api-release-attestation.sh \
  --attestation PATH --run-json PATH --run-id ID \
  --repository OWNER/REPO --api-repository IMAGE_REPOSITORY
USAGE
}

fail() {
  printf 'release attestation verification failed: %s\n' "$1" >&2
  exit 1
}

attestation=""
run_json_path=""
run_id=""
repository=""
api_repository=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --attestation)
      [[ $# -ge 2 ]] || fail "--attestation requires a path"
      attestation="$2"
      shift 2
      ;;
    --run-json)
      [[ $# -ge 2 ]] || fail "--run-json requires a path"
      run_json_path="$2"
      shift 2
      ;;
    --run-id)
      [[ $# -ge 2 ]] || fail "--run-id requires an ID"
      run_id="$2"
      shift 2
      ;;
    --repository)
      [[ $# -ge 2 ]] || fail "--repository requires OWNER/REPO"
      repository="$2"
      shift 2
      ;;
    --api-repository)
      [[ $# -ge 2 ]] || fail "--api-repository requires an image repository"
      api_repository="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      fail "unknown argument $1"
      ;;
  esac
done

[[ -n "$attestation" && -f "$attestation" ]] || fail "attestation is missing or expired"
[[ -n "$run_json_path" && -f "$run_json_path" ]] || fail "workflow run metadata is missing"
[[ "$run_id" =~ ^[1-9][0-9]*$ ]] || fail "run ID must be a positive integer"
[[ "$repository" =~ ^[^/[:space:]]+/[^/[:space:]]+$ ]] || fail "repository must be OWNER/REPO"
[[ -n "$api_repository" && "$api_repository" != *@* ]] || fail "API repository must not contain a digest"
for command in jq gh date; do
  command -v "$command" >/dev/null 2>&1 || fail "$command is required"
done

actual_run_id="$(jq -er '.id | select(type == "number")' "$run_json_path")"
actual_repository="$(jq -er '.repository.full_name | select(type == "string")' "$run_json_path")"
actual_workflow_name="$(jq -er '.name | select(type == "string")' "$run_json_path")"
actual_workflow_path="$(jq -er '.path | select(type == "string")' "$run_json_path")"
actual_workflow_path="${actual_workflow_path%%@*}"
actual_conclusion="$(jq -er '.conclusion | select(type == "string")' "$run_json_path")"
workflow_head_sha="$(jq -er '.head_sha | select(type == "string")' "$run_json_path")"

[[ "$actual_run_id" == "$run_id" ]] || fail "workflow run ID does not match"
[[ "$actual_repository" == "$repository" ]] || fail "workflow run repository does not match"
[[ "$actual_workflow_name" == "ListingKit API Deploy" ]] || fail "workflow name does not match"
[[ "$actual_workflow_path" == ".github/workflows/listingkit-deploy.yml" ]] || fail "workflow path does not match"
[[ "$actual_conclusion" == "success" ]] || fail "workflow conclusion is not success"
[[ "$workflow_head_sha" =~ ^[0-9a-f]{40}$ ]] || fail "workflow head SHA is malformed"

if ! jq -e '
  type == "object" and
  keys == ["api_candidate_image","api_workflow_run_id","expires_at","gate_version","issued_at","repository","source_sha","workflow_name","workflow_path"]
' "$attestation" >/dev/null; then
  fail "attestation schema is malformed"
fi

gate_version="$(jq -er '.gate_version | select(type == "string")' "$attestation")"
attested_repository="$(jq -er '.repository | select(type == "string")' "$attestation")"
attested_workflow_name="$(jq -er '.workflow_name | select(type == "string")' "$attestation")"
attested_workflow_path="$(jq -er '.workflow_path | select(type == "string")' "$attestation")"
source_sha="$(jq -er '.source_sha | select(type == "string")' "$attestation")"
api_candidate_image="$(jq -er '.api_candidate_image | select(type == "string")' "$attestation")"
attested_run_id="$(jq -er '.api_workflow_run_id | select(type == "number")' "$attestation")"
issued_at="$(jq -er '.issued_at | select(type == "string")' "$attestation")"
expires_at="$(jq -er '.expires_at | select(type == "string")' "$attestation")"

[[ "$gate_version" == "listingkit-api-release-gate/v1" ]] || fail "gate schema version does not match"
[[ "$attested_repository" == "$repository" ]] || fail "attested repository does not match"
[[ "$attested_workflow_name" == "ListingKit API Deploy" ]] || fail "attested workflow name does not match"
[[ "$attested_workflow_path" == ".github/workflows/listingkit-deploy.yml" ]] || fail "attested workflow path does not match"
[[ "$attested_run_id" == "$run_id" ]] || fail "attested workflow run ID does not match"
[[ "$source_sha" =~ ^[0-9a-f]{40}$ ]] || fail "attested source is not an exact lowercase commit SHA"

resolved_source_sha="$(gh api --method GET "repos/${repository}/commits/${source_sha}" --jq .sha)"
[[ "$resolved_source_sha" == "$source_sha" ]] || fail "attested source does not resolve to the exact repository commit"

candidate_prefix="${api_repository}@sha256:"
[[ "$api_candidate_image" == "$candidate_prefix"* ]] || fail "API candidate repository is not the expected repository"
candidate_digest="${api_candidate_image#"$candidate_prefix"}"
[[ "$candidate_digest" =~ ^[0-9a-f]{64}$ ]] || fail "API candidate is not digest-pinned"

issued_epoch="$(date -u -d "$issued_at" +%s)" || fail "issued_at is malformed"
expires_epoch="$(date -u -d "$expires_at" +%s)" || fail "expires_at is malformed"
now_epoch="$(date -u +%s)"
(( issued_epoch <= now_epoch )) || fail "attestation is not valid yet"
(( expires_epoch > issued_epoch )) || fail "attestation validity interval is invalid"
(( expires_epoch > now_epoch )) || fail "attestation is expired"

printf '%s\n' "$source_sha"
