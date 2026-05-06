import type { ListingKitTaskResult } from "@/lib/types/listingkit";

function primaryTaskError(task?: ListingKitTaskResult | null) {
  if (!task) return undefined;
  if (task.error) return task.error;
  return task.result?.child_tasks?.find((child) => child.error)?.error;
}

function workflowReviewReasonMessages(task?: ListingKitTaskResult | null) {
  return (
    task?.result?.workflow_issues
      ?.filter((issue) => issue.severity === "review" || issue.severity === "blocking")
      .map((issue) => issue.message ?? "")
      .filter(Boolean) ?? []
  );
}

function uniqueNormalizedReasons(values: string[]) {
  const seen = new Set<string>();
  return values
    .map(normalizeReasonLine)
    .filter(Boolean)
    .filter((line) => {
      if (seen.has(line)) {
        return false;
      }
      seen.add(line);
      return true;
    });
}

function normalizeReasonLine(value: string) {
  return value.replace(/^\s*(?:[-*•]\s+|\d+\.\s+)/, "").trim();
}

export function extractTaskReviewReasons(task?: ListingKitTaskResult | null) {
  const workflowReasons = uniqueNormalizedReasons(workflowReviewReasonMessages(task));
  if (workflowReasons.length > 0) {
    return workflowReasons;
  }

  const structuredReasons = uniqueNormalizedReasons([
    ...(task?.review_reasons ?? []),
    ...(task?.result?.review_reasons ?? []),
  ]);
  if (structuredReasons.length > 0) {
    return structuredReasons;
  }

  const error = primaryTaskError(task);
  if (!error) {
    return [];
  }

  return uniqueNormalizedReasons(
    error.split(/\r?\n+/).flatMap((line) =>
      line
        .split(/[;；]+/)
        .map((part) => part.trim())
        .filter(Boolean),
    ),
  );
}
