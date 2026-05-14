import type { ListingKitTaskListItem } from "@/lib/types/listingkit";

const ACTIONABLE_STATUSES = new Set([
  "pending",
  "processing",
  "needs_review",
  "failed",
]);
const RESUMABLE_SHEIN_WORKFLOW_STATUSES = new Set([
  "pending_confirmation",
  "ready_to_submit",
  "publish_failed",
  "draft_saved",
]);
const RESUMABLE_SHEIN_WORK_QUEUES = new Set([
  "repair_queue",
  "review_queue",
  "submit_ready_queue",
  "draft_queue",
  "submit_failed_queue",
]);

function updatedAtValue(task: ListingKitTaskListItem) {
  const value = Date.parse(task.updated_at ?? task.created_at ?? "");
  return Number.isNaN(value) ? 0 : value;
}

function isActionable(task: ListingKitTaskListItem) {
  return ACTIONABLE_STATUSES.has(task.status ?? "");
}

function isSheinTask(task: ListingKitTaskListItem) {
  return (task.platforms ?? []).includes("shein");
}

function hasResumableSheinWorkflow(task: ListingKitTaskListItem) {
  return RESUMABLE_SHEIN_WORKFLOW_STATUSES.has(task.shein_workflow_status ?? "");
}

function hasResumableSheinQueue(task: ListingKitTaskListItem) {
  return RESUMABLE_SHEIN_WORK_QUEUES.has(task.shein_work_queue ?? "");
}

function hasResumableSheinAction(task: ListingKitTaskListItem) {
  return Boolean(
    task.shein_action_queue ||
      task.shein_status_overview?.primary_action ||
      task.shein_status_overview?.needs_review,
  );
}

function isContinueCandidate(task: ListingKitTaskListItem) {
  return (
    isActionable(task) ||
    (isSheinTask(task) &&
      (hasResumableSheinWorkflow(task) ||
        hasResumableSheinQueue(task) ||
        hasResumableSheinAction(task)))
  );
}

export function sortRecentTasksForHomepage(tasks: ListingKitTaskListItem[]) {
  return [...tasks]
    .sort((left, right) => updatedAtValue(right) - updatedAtValue(left))
    .slice(0, 3);
}

export function pickContinueTask(tasks: ListingKitTaskListItem[]) {
  const sorted = [...tasks].sort(
    (left, right) => updatedAtValue(right) - updatedAtValue(left),
  );

  return (
    sorted.find((task) => isSheinTask(task) && isContinueCandidate(task)) ??
    sorted.find((task) => isContinueCandidate(task)) ??
    sorted[0] ??
    null
  );
}
