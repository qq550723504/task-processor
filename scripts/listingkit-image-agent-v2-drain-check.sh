#!/usr/bin/env bash

set -euo pipefail

fail() {
  printf 'image-agent v2 drain check failed: %s\n' "$1" >&2
  exit 1
}

usage() {
  cat >&2 <<'EOF'
usage: listingkit-image-agent-v2-drain-check.sh \
  --expected-run-id RUN_ID \
  --expected-run-attempt RUN_ATTEMPT \
  --expected-api-image REPOSITORY@sha256:DIGEST \
  --namespace NAMESPACE
EOF
  exit 2
}

required_temporal_cli="1.8.1"
v2_queue="image-agent-manual"
v3_queue="image-agent-manual-v3"
sample_count=3
convergence_interval_seconds=300
run_id_annotation="listingkit.sh/api-release-run-id"
run_attempt_annotation="listingkit.sh/api-release-run-attempt"
image_annotation="listingkit.sh/api-release-image"
routing_annotation="listingkit.sh/image-agent-routing-contract"
routing_contract="image-agent-v3-new-starts-v1"

expected_run_id=""
expected_run_attempt=""
expected_api_image=""
namespace=""
while (($# > 0)); do
  case "$1" in
    --expected-run-id)
      (($# >= 2)) || usage
      expected_run_id="$2"
      shift 2
      ;;
    --expected-run-attempt)
      (($# >= 2)) || usage
      expected_run_attempt="$2"
      shift 2
      ;;
    --expected-api-image)
      (($# >= 2)) || usage
      expected_api_image="$2"
      shift 2
      ;;
    --namespace)
      (($# >= 2)) || usage
      namespace="$2"
      shift 2
      ;;
    *) usage ;;
  esac
done

[[ "$expected_run_id" =~ ^[1-9][0-9]*$ ]] || fail "expected run ID must be a positive integer"
[[ "$expected_run_attempt" =~ ^[1-9][0-9]*$ ]] || fail "expected run attempt must be a positive integer"
[[ "$expected_api_image" =~ ^[A-Za-z0-9][A-Za-z0-9._:/-]*@sha256:[0-9a-f]{64}$ ]] || \
  fail "expected API image must be an immutable lowercase sha256 reference"
[[ ${#namespace} -le 63 && "$namespace" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] || \
  fail "namespace must be a valid DNS label"

for required_variable in TEMPORAL_ADDRESS TEMPORAL_NAMESPACE PGHOST PGPORT PGDATABASE PGUSER; do
  [[ -n "${!required_variable:-}" ]] || fail "${required_variable} is required"
done
[[ "$PGPORT" =~ ^[1-9][0-9]{0,4}$ ]] || fail "PGPORT must be a positive port"

for required_command in temporal jq mktemp kubectl psql sleep sort cmp wc chmod; do
  command -v "$required_command" >/dev/null 2>&1 || fail "$required_command is required"
done

temporal_version="$(temporal --version)" || fail "Temporal CLI version check failed"
grep -Eq "(^|[[:space:]])v?${required_temporal_cli}([[:space:]]|$)" \
  <<<"$temporal_version" || fail "Temporal CLI ${required_temporal_cli} is required"

umask 077
work_dir="$(mktemp -d)" || fail "could not create temporary evidence directory"
trap 'rm -rf -- "$work_dir"' EXIT
chmod 700 "$work_dir" || fail "could not protect temporary evidence directory"

api_ready_pod_count=0
db_nonterminal_run_count=0
temporal_parent_identity_count=0
open_v2_parent_count=0
open_v2_child_count=0
pending_v2_child_count=0
pending_v2_activity_count=0
pending_v2_activity_attempt_sum=0
description_index=0
evidence_dir=""
temporal_parent_ids=""

collect_api_evidence() {
  local deployment_json="$evidence_dir/api-deployment.json"
  local pods_json="$evidence_dir/api-pods.json"
  local deployment_ready_count

  if ! kubectl -n "$namespace" get deployment product-listing-api -o json >"$deployment_json"; then
    fail "could not query the serving API Deployment"
  fi
  if ! kubectl -n "$namespace" get pods -l app=product-listing-api -o json >"$pods_json"; then
    fail "could not query serving API Pods"
  fi

  if ! jq -e \
    --arg image "$expected_api_image" \
    --arg run_id "$expected_run_id" \
    --arg run_attempt "$expected_run_attempt" \
    --arg run_id_key "$run_id_annotation" \
    --arg run_attempt_key "$run_attempt_annotation" \
    --arg image_key "$image_annotation" \
    --arg routing_key "$routing_annotation" \
    --arg routing_contract "$routing_contract" '
      # listingkit-api-deployment-shape
      def exact_annotations:
        .[$run_id_key] == $run_id and
        .[$run_attempt_key] == $run_attempt and
        .[$image_key] == $image and
        .[$routing_key] == $routing_contract;
      type == "object" and
      .metadata.name == "product-listing-api" and
      (.metadata.annotations | type == "object" and exact_annotations) and
      (.spec.template.metadata.annotations | type == "object" and exact_annotations) and
      (.metadata.generation | type == "number" and . >= 1 and floor == .) and
      (.spec.replicas | type == "number" and . >= 1 and floor == .) and
      .status.observedGeneration == .metadata.generation and
      .status.readyReplicas == .spec.replicas and
      .status.updatedReplicas == .spec.replicas and
      .status.availableReplicas == .spec.replicas and
      ([.spec.template.spec.containers[] | select(.name == "product-listing-api")] | length == 1) and
      all(.spec.template.spec.containers[] | select(.name == "product-listing-api"); .image == $image)
    ' "$deployment_json" >/dev/null; then
    fail "serving API Deployment identity or readiness is invalid"
  fi
  if ! deployment_ready_count="$(jq -er '
    # listingkit-api-deployment-ready-count
    .status.readyReplicas
  ' "$deployment_json")"; then
    fail "serving API Deployment ready count is invalid"
  fi

  if ! jq -e \
    --arg image "$expected_api_image" \
    --arg run_id "$expected_run_id" \
    --arg run_attempt "$expected_run_attempt" \
    --arg run_id_key "$run_id_annotation" \
    --arg run_attempt_key "$run_attempt_annotation" \
    --arg image_key "$image_annotation" \
    --arg routing_key "$routing_annotation" \
    --arg routing_contract "$routing_contract" '
      # listingkit-api-pods-shape
      def exact_annotations:
        .[$run_id_key] == $run_id and
        .[$run_attempt_key] == $run_attempt and
        .[$image_key] == $image and
        .[$routing_key] == $routing_contract;
      type == "object" and
      (.items | type == "array" and length > 0) and
      all(.items[];
        .metadata.deletionTimestamp == null and
        (.metadata.annotations | type == "object" and exact_annotations) and
        .status.phase == "Running" and
        any(.status.conditions[]?; .type == "Ready" and .status == "True") and
        ([.spec.containers[] | select(.name == "product-listing-api")] | length == 1) and
        all(.spec.containers[] | select(.name == "product-listing-api"); .image == $image) and
        ([.status.containerStatuses[] | select(.name == "product-listing-api")] | length == 1) and
        all(.status.containerStatuses[] | select(.name == "product-listing-api");
          .ready == true and (.state.running | type == "object")))
    ' "$pods_json" >/dev/null; then
    fail "serving API Pod identity, image, or readiness is invalid"
  fi
  if ! api_ready_pod_count="$(jq -er '
    # listingkit-api-pods-count
    .items | length
  ' "$pods_json")"; then
    fail "serving API Pod count is invalid"
  fi
  [[ "$api_ready_pod_count" =~ ^[1-9][0-9]*$ && "$api_ready_pod_count" == "$deployment_ready_count" ]] || \
    fail "serving API Deployment and Pod readiness counts disagree"
}

collect_database_inventory() {
  local db_tsv="$evidence_dir/database-runs.tsv"
  local db_ids_unsorted="$evidence_dir/database-workflow-ids.unsorted"
  local db_ids="$evidence_dir/database-workflow-ids"
  local tenant_id owner_user_id run_id extra value raw_count unique_count
  local sql="SELECT tenant_id, owner_user_id, id FROM image_agent_v2_runs WHERE status NOT IN ('completed', 'failed', 'cancelled') ORDER BY tenant_id, owner_user_id, id"

  if ! psql --no-psqlrc --set=ON_ERROR_STOP=1 --tuples-only --no-align \
    --field-separator=$'\t' --command "$sql" >"$db_tsv"; then
    fail "PostgreSQL non-terminal v2 run query failed"
  fi
  : >"$db_ids_unsorted"
  raw_count=0
  while IFS=$'\t' read -r tenant_id owner_user_id run_id extra || \
    [[ -n "${tenant_id}${owner_user_id}${run_id}${extra}" ]]; do
    [[ -n "${tenant_id}${owner_user_id}${run_id}${extra}" ]] || continue
    [[ -z "$extra" && -n "$tenant_id" && -n "$owner_user_id" && -n "$run_id" ]] || \
      fail "PostgreSQL returned malformed v2 run identity output"
    for value in "$tenant_id" "$owner_user_id" "$run_id"; do
      [[ "$value" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || \
        fail "PostgreSQL returned malformed v2 run identity output"
    done
    printf 'image-agent:%s:%s:%s\n' "$tenant_id" "$owner_user_id" "$run_id" >>"$db_ids_unsorted"
    raw_count=$((raw_count + 1))
  done <"$db_tsv"
  sort -u "$db_ids_unsorted" >"$db_ids" || fail "could not normalize PostgreSQL identities"
  unique_count="$(wc -l <"$db_ids")"
  unique_count="${unique_count//[[:space:]]/}"
  [[ "$unique_count" =~ ^(0|[1-9][0-9]*)$ && "$unique_count" == "$raw_count" ]] || \
    fail "PostgreSQL returned duplicate or malformed v2 run identities"
  db_nonterminal_run_count="$raw_count"
}

collect_open_executions() {
  local workflow_type="$1"
  local query="WorkflowType = '${workflow_type}' AND ExecutionStatus = 'Running'"
  local list_json="$evidence_dir/${workflow_type}.json"
  local count_json="$evidence_dir/${workflow_type}.count.json"
  local list_tsv="$evidence_dir/${workflow_type}.tsv"
  local authoritative_count list_count

  if ! temporal workflow list --address "$TEMPORAL_ADDRESS" --namespace "$TEMPORAL_NAMESPACE" \
    --output json --query "$query" >"$list_json"; then
    fail "Temporal list failed for ${workflow_type}"
  fi
  if ! temporal workflow count --address "$TEMPORAL_ADDRESS" --namespace "$TEMPORAL_NAMESPACE" \
    --output json --query "$query" >"$count_json"; then
    fail "Temporal count failed for ${workflow_type}"
  fi

  if ! authoritative_count="$(jq -er '
    # listingkit-count
    if type == "object" and length == 0 then "0"
    elif type == "object" and (keys | sort) == ["count"] and
      (.count | type == "string" and test("^[1-9][0-9]*$")) then .count
    else error("invalid Temporal count response") end
  ' "$count_json")"; then
    fail "Temporal count JSON is malformed for ${workflow_type}"
  fi
  [[ "$authoritative_count" =~ ^(0|[1-9][0-9]*)$ ]] || \
    fail "Temporal count is not a non-negative integer for ${workflow_type}"

  if [[ ! -s "$list_json" ]]; then
    [[ "$authoritative_count" == "0" ]] || fail "Temporal count/list evidence disagrees for ${workflow_type}"
    : >"$list_tsv"
    return 0
  fi
  grep -q '[^[:space:]]' "$list_json" || fail "Temporal list output is whitespace-only for ${workflow_type}"
  jq -e -s '
    # listingkit-list-shape
    length > 0 and all(.[]; type == "object" and
      (.execution.workflowId | type == "string" and length > 0) and
      (.execution.runId | type == "string" and length > 0) and
      (.type.name | type == "string" and length > 0))
  ' "$list_json" >/dev/null || fail "Temporal list JSON is malformed for ${workflow_type}"
  list_count="$(jq -er -s '
    # listingkit-list-count
    length
  ' "$list_json")" || fail "Temporal list count could not be normalized for ${workflow_type}"
  [[ "$list_count" =~ ^(0|[1-9][0-9]*)$ && "$list_count" == "$authoritative_count" ]] || \
    fail "Temporal count/list evidence disagrees for ${workflow_type}"
  jq -r -s '
    # listingkit-list-rows
    .[] | [.execution.workflowId, .execution.runId, .type.name] | @tsv
  ' "$list_json" >"$list_tsv" || fail "Temporal list JSON could not be normalized for ${workflow_type}"
}

is_v2_activity() {
  case "$1" in
    imageagent.execute_slot | imageagent.execute_slot.v2 | imageagent.persist_slot_result | \
      imageagent.persist_slot_result.v2 | imageagent.persist_run_state | imageagent.persist_run_state.v2 | \
      imageagent.persist_plan_revision | imageagent.persist_plan_revision.v2 | imageagent.persist_pending_command | \
      imageagent.persist_pending_command.v2 | imageagent.publish_approved | imageagent.publish_approved.v2)
      return 0 ;;
    *) return 1 ;;
  esac
}

describe_execution() {
  local expected_type="$1" workflow_id="$2" temporal_run_id="$3" listed_type="$4"
  local description_json="$evidence_dir/description-${description_index}.json"
  local children_tsv="$evidence_dir/children-${description_index}.tsv"
  local activities_tsv="$evidence_dir/activities-${description_index}.tsv"
  local task_queue child_workflow_id child_run_id child_type activity_name attempt
  description_index=$((description_index + 1))

  [[ "$listed_type" == "$expected_type" ]] || fail "Temporal list returned an unexpected workflow type"
  [[ -n "$workflow_id" && -n "$temporal_run_id" ]] || fail "Temporal list returned a missing workflow identity"
  temporal workflow describe --address "$TEMPORAL_ADDRESS" --namespace "$TEMPORAL_NAMESPACE" \
    --workflow-id "$workflow_id" --run-id "$temporal_run_id" --output json >"$description_json" || \
    fail "Temporal describe failed"
  jq -e '
    # listingkit-describe-shape
    def task_queue:
      if (.workflowExecutionInfo.taskQueue | type) == "string" then .workflowExecutionInfo.taskQueue
      elif (.workflowExecutionInfo.taskQueue.name | type) == "string" then .workflowExecutionInfo.taskQueue.name
      elif (.executionConfig.taskQueue.name | type) == "string" then .executionConfig.taskQueue.name
      else null end;
    type == "object" and (task_queue | type == "string" and length > 0) and
    ((.pendingChildren == null) or (.pendingChildren | type == "array")) and
    all((.pendingChildren // [])[]; (.workflowId | type == "string" and length > 0) and
      (.runId | type == "string" and length > 0) and
      ((.workflowTypeName == "ImageSlotWorkflow") or (.workflowType.name == "ImageSlotWorkflow") or
       (.workflowTypeName == "ImageSlotWorkflowV3") or (.workflowType.name == "ImageSlotWorkflowV3"))) and
    ((.pendingActivities == null) or (.pendingActivities | type == "array")) and
    all((.pendingActivities // [])[]; (.activityType.name | type == "string" and length > 0) and
      (.attempt | type == "number" and . >= 1 and floor == .))
  ' "$description_json" >/dev/null || fail "Temporal describe JSON is malformed"
  task_queue="$(jq -er '
    # listingkit-queue
    if (.workflowExecutionInfo.taskQueue | type) == "string" then .workflowExecutionInfo.taskQueue
    elif (.workflowExecutionInfo.taskQueue.name | type) == "string" then .workflowExecutionInfo.taskQueue.name
    else .executionConfig.taskQueue.name end
  ' "$description_json")" || fail "Temporal describe queue is missing"
  [[ "$task_queue" == "$v2_queue" || "$task_queue" == "$v3_queue" ]] || \
    fail "Temporal image-agent execution uses an unexpected task queue"
  jq -r '
    # listingkit-child-rows
    (.pendingChildren // [])[] | [.workflowId, .runId, (.workflowTypeName // .workflowType.name)] | @tsv
  ' "$description_json" >"$children_tsv" || fail "Temporal pending child evidence could not be normalized"
  jq -r '
    # listingkit-activity-rows
    (.pendingActivities // [])[] | [.activityType.name, (.attempt | tostring)] | @tsv
  ' "$description_json" >"$activities_tsv" || fail "Temporal pending activity evidence could not be normalized"

  [[ "$expected_type" != "ImageAgentWorkflow" ]] || printf '%s\n' "$workflow_id" >>"$temporal_parent_ids"
  [[ "$task_queue" == "$v2_queue" ]] || return 0
  case "$expected_type" in
    ImageAgentWorkflow) open_v2_parent_count=$((open_v2_parent_count + 1)) ;;
    ImageSlotWorkflow) open_v2_child_count=$((open_v2_child_count + 1)) ;;
    *) fail "unsupported workflow type" ;;
  esac
  while IFS=$'\t' read -r child_workflow_id child_run_id child_type; do
    [[ -n "$child_workflow_id" ]] || continue
    [[ "$expected_type" == "ImageAgentWorkflow" && "$child_type" == "ImageSlotWorkflow" ]] || \
      fail "unexpected pending child on the v2 queue"
    pending_v2_child_count=$((pending_v2_child_count + 1))
  done <"$children_tsv"
  while IFS=$'\t' read -r activity_name attempt; do
    [[ -n "$activity_name" ]] || continue
    is_v2_activity "$activity_name" || fail "unexpected pending activity on the v2 queue"
    [[ "$attempt" =~ ^[1-9][0-9]*$ ]] || fail "pending activity attempt is malformed"
    pending_v2_activity_count=$((pending_v2_activity_count + 1))
    pending_v2_activity_attempt_sum=$((pending_v2_activity_attempt_sum + attempt))
  done <"$activities_tsv"
}

collect_temporal_inventory() {
  local workflow_type workflow_id temporal_run_id listed_type raw_count unique_count
  local temporal_parent_ids_sorted="$evidence_dir/temporal-parent-workflow-ids"
  temporal_parent_ids="$evidence_dir/temporal-parent-workflow-ids.unsorted"
  : >"$temporal_parent_ids"
  open_v2_parent_count=0; open_v2_child_count=0; pending_v2_child_count=0
  pending_v2_activity_count=0; pending_v2_activity_attempt_sum=0; description_index=0
  collect_open_executions "ImageAgentWorkflow"
  collect_open_executions "ImageSlotWorkflow"
  for workflow_type in ImageAgentWorkflow ImageSlotWorkflow; do
    while IFS=$'\t' read -r workflow_id temporal_run_id listed_type; do
      [[ -n "$workflow_id" ]] || continue
      describe_execution "$workflow_type" "$workflow_id" "$temporal_run_id" "$listed_type"
    done <"$evidence_dir/${workflow_type}.tsv"
  done
  sort -u "$temporal_parent_ids" >"$temporal_parent_ids_sorted" || fail "could not normalize Temporal parent identities"
  raw_count="$(wc -l <"$temporal_parent_ids")"; raw_count="${raw_count//[[:space:]]/}"
  unique_count="$(wc -l <"$temporal_parent_ids_sorted")"; unique_count="${unique_count//[[:space:]]/}"
  [[ "$raw_count" =~ ^(0|[1-9][0-9]*)$ && "$raw_count" == "$unique_count" ]] || \
    fail "Temporal returned duplicate parent workflow identities"
  temporal_parent_identity_count="$unique_count"
}

print_sample_summary() {
  printf 'sample=%d\n' "$1"
  printf 'api_ready_pod_count=%d\n' "$api_ready_pod_count"
  printf 'db_nonterminal_run_count=%d\n' "$db_nonterminal_run_count"
  printf 'temporal_parent_identity_count=%d\n' "$temporal_parent_identity_count"
  printf 'open_v2_parent_count=%d\n' "$open_v2_parent_count"
  printf 'open_v2_child_count=%d\n' "$open_v2_child_count"
  printf 'pending_v2_child_count=%d\n' "$pending_v2_child_count"
  printf 'pending_v2_activity_count=%d\n' "$pending_v2_activity_count"
  printf 'pending_v2_activity_attempt_sum=%d\n' "$pending_v2_activity_attempt_sum"
}

for ((sample = 1; sample <= sample_count; sample++)); do
  evidence_dir="$work_dir/sample-${sample}"
  mkdir -m 700 "$evidence_dir" || fail "could not create private sample evidence directory"
  collect_api_evidence
  collect_database_inventory
  collect_temporal_inventory
  cmp -s "$evidence_dir/database-workflow-ids" "$evidence_dir/temporal-parent-workflow-ids" || \
    fail "authoritative PostgreSQL and Temporal parent identities disagree"
  print_sample_summary "$sample"
  if ((open_v2_parent_count != 0 || open_v2_child_count != 0 || \
    pending_v2_child_count != 0 || pending_v2_activity_count != 0)); then
    exit 1
  fi
  ((sample >= sample_count)) || sleep "$convergence_interval_seconds" || fail "fixed convergence wait failed"
done

printf 'stable_sample_count=%d\n' "$sample_count"
printf 'convergence_interval_seconds=%d\n' "$convergence_interval_seconds"
printf 'first_to_final_window_seconds=%d\n' "$((convergence_interval_seconds * (sample_count - 1)))"
