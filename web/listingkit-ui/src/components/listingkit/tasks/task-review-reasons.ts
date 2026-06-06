import type { ListingKitTaskResult } from "@/lib/types/listingkit";
import { buildSheinGeneralReviewHref } from "@/components/listingkit/shein/shein-workspace-actions";

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

type SheinReviewActionKey = "category" | "attributes" | "sale_attributes";

export type TaskReviewActionLink = {
  key: SheinReviewActionKey;
  label: string;
  href: string;
};

function inferSheinReviewActionKeys(reasons: string[]) {
  const keys = new Set<SheinReviewActionKey>();

  for (const reason of reasons) {
    const normalized = reason.toLowerCase();

    if (
      reason.includes("销售属性") ||
      reason.includes("变体规格") ||
      normalized.includes("sale attribute")
    ) {
      keys.add("sale_attributes");
    }

    if (
      reason.includes("类目") ||
      normalized.includes("category_id") ||
      normalized.includes("category path")
    ) {
      keys.add("category");
    }

    if (
      reason.includes("普通属性") ||
      reason.includes("属性模板") ||
      normalized.includes("attribute_id")
    ) {
      keys.add("attributes");
    }
  }

  return keys;
}

export function buildTaskReviewActionLinks(
  taskId: string,
  task?: ListingKitTaskResult | null,
): TaskReviewActionLink[] {
  const reasons = extractTaskReviewReasons(task);
  const actionKeys = inferSheinReviewActionKeys(reasons);
  const orderedKeys: SheinReviewActionKey[] = [
    "category",
    "attributes",
    "sale_attributes",
  ];

  return orderedKeys
    .filter((key) => actionKeys.has(key))
    .map((key) => {
      switch (key) {
        case "category":
          return {
            key,
            label: "去确认类目",
            href: buildSheinGeneralReviewHref(taskId, "shein-category-review-card"),
          };
        case "attributes":
          return {
            key,
            label: "去确认普通属性",
            href: buildSheinGeneralReviewHref(taskId, "shein-attribute-review-card"),
          };
        case "sale_attributes":
          return {
            key,
            label: "去确认销售属性",
            href: buildSheinGeneralReviewHref(taskId, "shein-sale-attribute-review-card"),
          };
      }
    });
}
