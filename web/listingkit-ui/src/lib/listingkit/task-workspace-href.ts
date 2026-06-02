import type { ListingKitTaskListItem } from "@/lib/types/listingkit/tasks";

const RESUMABLE_SHEIN_WORKFLOW_STATUSES = new Set([
  "pending_confirmation",
  "ready_to_submit",
  "publish_failed",
  "draft_saved",
]);

export function isResumableSheinTask(task: ListingKitTaskListItem) {
  return (
    task.platforms?.includes("shein") &&
    (task.shein_workflow_status == null ||
      RESUMABLE_SHEIN_WORKFLOW_STATUSES.has(task.shein_workflow_status))
  );
}

export function buildTaskWorkspaceHref(task: ListingKitTaskListItem) {
  const baseHref = `/listing-kits/${task.task_id}/workspace`;
  const platform = isResumableSheinTask(task) ? "shein" : task.platforms?.[0];

  if (!platform) {
    return baseHref;
  }

  return `${baseHref}?platform=${platform}`;
}
