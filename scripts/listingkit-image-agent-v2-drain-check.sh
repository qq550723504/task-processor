#!/usr/bin/env bash

set -euo pipefail

fail() {
  printf 'image-agent v2 drain check failed: %s\n' "$1" >&2
  exit 1
}

required_temporal_cli="1.8.1"
v2_queue="image-agent-manual"

: "${TEMPORAL_ADDRESS:?set the target Temporal address}"
: "${TEMPORAL_NAMESPACE:?set the target Temporal namespace}"
for required_command in temporal jq mktemp; do
  command -v "$required_command" >/dev/null 2>&1 || fail "$required_command is required"
done

temporal_version="$(temporal --version)" || fail "Temporal CLI version check failed"
grep -Eq "(^|[[:space:]])v?${required_temporal_cli}([[:space:]]|$)" \
  <<<"$temporal_version" || fail "Temporal CLI ${required_temporal_cli} is required"

work_dir="$(mktemp -d)" || fail "could not create temporary evidence directory"
trap 'rm -rf -- "$work_dir"' EXIT

collect_open_executions() {
  local workflow_type="$1"
  local list_json="$work_dir/${workflow_type}.json"
  local list_tsv="$work_dir/${workflow_type}.tsv"

  if ! temporal workflow list \
    --address "$TEMPORAL_ADDRESS" \
    --namespace "$TEMPORAL_NAMESPACE" \
    --output json \
    --query "WorkflowType = '${workflow_type}' AND ExecutionStatus = 'Running'" \
    >"$list_json"; then
    fail "Temporal list failed for ${workflow_type}"
  fi

  if ! jq -e -s '
    # listingkit-list-shape
    def records: .[] | if type == "array" then .[] else . end;
    . as $documents |
    ($documents | length) > 0 and
    all($documents[]; type == "array" or type == "object") and
    ([records] | all(.[];
      type == "object" and
      (.execution.workflowId | type == "string" and length > 0) and
      (.execution.runId | type == "string" and length > 0) and
      (.type.name | type == "string" and length > 0)))
  ' "$list_json" >/dev/null; then
    fail "Temporal list JSON is malformed for ${workflow_type}"
  fi

  if ! jq -r -s '
    # listingkit-list-rows
    def records: .[] | if type == "array" then .[] else . end;
    records | [.execution.workflowId, .execution.runId, .type.name] | @tsv
  ' "$list_json" >"$list_tsv"; then
    fail "Temporal list JSON could not be normalized for ${workflow_type}"
  fi
}

collect_open_executions "ImageAgentWorkflow"
collect_open_executions "ImageSlotWorkflow"

open_v2_parent_count=0
open_v2_child_count=0
pending_v2_child_count=0
pending_v2_activity_count=0
pending_v2_activity_attempt_sum=0
description_index=0

is_v2_activity() {
  case "$1" in
    imageagent.execute_slot | imageagent.execute_slot.v2 | \
      imageagent.persist_slot_result | imageagent.persist_slot_result.v2 | \
      imageagent.persist_run_state | imageagent.persist_run_state.v2 | \
      imageagent.persist_plan_revision | imageagent.persist_plan_revision.v2 | \
      imageagent.persist_pending_command | imageagent.persist_pending_command.v2 | \
      imageagent.publish_approved | imageagent.publish_approved.v2)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

describe_execution() {
  local expected_type="$1"
  local workflow_id="$2"
  local temporal_run_id="$3"
  local listed_type="$4"
  local description_json="$work_dir/description-${description_index}.json"
  local children_tsv="$work_dir/children-${description_index}.tsv"
  local activities_tsv="$work_dir/activities-${description_index}.tsv"
  local task_queue child_workflow_id child_run_id child_type activity_name attempt
  description_index=$((description_index + 1))

  [[ "$listed_type" == "$expected_type" ]] || fail "Temporal list returned an unexpected workflow type"
  [[ -n "$workflow_id" && -n "$temporal_run_id" ]] || fail "Temporal list returned a missing workflow identity"
  if ! temporal workflow describe \
    --address "$TEMPORAL_ADDRESS" \
    --namespace "$TEMPORAL_NAMESPACE" \
    --workflow-id "$workflow_id" \
    --run-id "$temporal_run_id" \
    --output json \
    >"$description_json"; then
    fail "Temporal describe failed"
  fi

  if ! jq -e '
    # listingkit-describe-shape
    def task_queue:
      if (.workflowExecutionInfo.taskQueue | type) == "string" then .workflowExecutionInfo.taskQueue
      elif (.workflowExecutionInfo.taskQueue.name | type) == "string" then .workflowExecutionInfo.taskQueue.name
      elif (.executionConfig.taskQueue.name | type) == "string" then .executionConfig.taskQueue.name
      else null end;
    type == "object" and
    (task_queue | type == "string" and length > 0) and
    ((.pendingChildren == null) or (.pendingChildren | type == "array")) and
    all((.pendingChildren // [])[];
      (.workflowId | type == "string" and length > 0) and
      (.runId | type == "string" and length > 0) and
      ((.workflowTypeName == "ImageSlotWorkflow") or (.workflowType.name == "ImageSlotWorkflow"))) and
    ((.pendingActivities == null) or (.pendingActivities | type == "array")) and
    all((.pendingActivities // [])[];
      (.activityType.name | type == "string" and length > 0) and
      (.attempt | type == "number" and . >= 1 and floor == .))
  ' "$description_json" >/dev/null; then
    fail "Temporal describe JSON is malformed"
  fi

  task_queue="$(jq -er '
    # listingkit-queue
    if (.workflowExecutionInfo.taskQueue | type) == "string" then .workflowExecutionInfo.taskQueue
    elif (.workflowExecutionInfo.taskQueue.name | type) == "string" then .workflowExecutionInfo.taskQueue.name
    else .executionConfig.taskQueue.name end
  ' "$description_json")" || fail "Temporal describe queue is missing"
  if ! jq -r '
    # listingkit-child-rows
    (.pendingChildren // [])[] |
    [.workflowId, .runId, (.workflowTypeName // .workflowType.name)] | @tsv
  ' "$description_json" >"$children_tsv"; then
    fail "Temporal pending child evidence could not be normalized"
  fi
  if ! jq -r '
    # listingkit-activity-rows
    (.pendingActivities // [])[] |
    [.activityType.name, (.attempt | tostring)] | @tsv
  ' "$description_json" >"$activities_tsv"; then
    fail "Temporal pending activity evidence could not be normalized"
  fi

  if [[ "$task_queue" != "$v2_queue" ]]; then
    return 0
  fi

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

for workflow_type in ImageAgentWorkflow ImageSlotWorkflow; do
  while IFS=$'\t' read -r workflow_id temporal_run_id listed_type; do
    [[ -n "$workflow_id" ]] || continue
    describe_execution "$workflow_type" "$workflow_id" "$temporal_run_id" "$listed_type"
  done <"$work_dir/${workflow_type}.tsv"
done

printf 'open_v2_parent_count=%d\n' "$open_v2_parent_count"
printf 'open_v2_child_count=%d\n' "$open_v2_child_count"
printf 'pending_v2_child_count=%d\n' "$pending_v2_child_count"
printf 'pending_v2_activity_count=%d\n' "$pending_v2_activity_count"
printf 'pending_v2_activity_attempt_sum=%d\n' "$pending_v2_activity_attempt_sum"

if ((open_v2_parent_count != 0 || open_v2_child_count != 0 || \
  pending_v2_child_count != 0 || pending_v2_activity_count != 0)); then
  exit 1
fi
