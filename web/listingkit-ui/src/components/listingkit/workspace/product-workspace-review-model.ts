import {
  normalizeSheinWorkspaceActionKey,
  type SheinWorkspaceActionKey,
} from "@/components/listingkit/shein/shein-workspace-actions";
import { extractTaskReviewReasons } from "@/components/listingkit/tasks/task-review-reasons";
import type { ProductWorkspaceAttentionSeverity } from "@/components/listingkit/workspace/product-workspace-model";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export type ProductWorkspaceReviewIssue = {
  id: string;
  severity: Exclude<ProductWorkspaceAttentionSeverity, "passed">;
  title: string;
  description?: string;
  suggestion?: string;
  evidence?: string;
  confidence?: number;
  actionKey?: SheinWorkspaceActionKey;
};

export function buildProductWorkspaceReviewIssues(
  task?: ListingKitTaskResult | null,
  selectedPlatform?: string,
): ProductWorkspaceReviewIssue[] {
  const workflowIssues = (task?.result?.workflow_issues ?? [])
    .filter(
      (issue) =>
        issue.severity === "blocking" ||
        issue.severity === "warning" ||
        issue.severity === "review",
    )
    .map((issue, index) => {
      const actionKey = resolveIssueActionKey(selectedPlatform, [
        issue.code,
        issue.message,
        issue.detail,
      ]);
      return {
        id: issue.code || `workflow-issue-${index + 1}`,
        severity: issue.severity === "blocking" ? "blocking" : "warning",
        title: issue.message || issue.code || "需要确认",
        description: issue.detail,
        ...(actionKey ? { actionKey } : {}),
      } satisfies ProductWorkspaceReviewIssue;
    });

  if (workflowIssues.length > 0) {
    return workflowIssues;
  }

  return extractTaskReviewReasons(task).map((reason, index) => {
    const actionKey = resolveIssueActionKey(selectedPlatform, [reason]);
    return {
      id: `fallback-review-${index + 1}`,
      severity: task?.status === "needs_review" ? "blocking" : "warning",
      title: reason,
      ...(actionKey ? { actionKey } : {}),
    } satisfies ProductWorkspaceReviewIssue;
  });
}

function resolveIssueActionKey(
  selectedPlatform: string | undefined,
  candidates: Array<string | undefined>,
): SheinWorkspaceActionKey | undefined {
  if (selectedPlatform !== "shein") {
    return undefined;
  }

  for (const candidate of candidates) {
    const actionKey = normalizeSheinWorkspaceActionKey(candidate);
    if (actionKey) {
      return actionKey;
    }
  }

  return undefined;
}
