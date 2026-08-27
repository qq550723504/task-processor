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
  const relevantWorkflowIssues = (task?.result?.workflow_issues ?? []).filter(
    (issue) =>
      issue.severity === "blocking" ||
      issue.severity === "warning" ||
      issue.severity === "review",
  );
  const issueCodeOccurrences = new Map<string, number>();
  const workflowIssues = relevantWorkflowIssues.map((issue, index) => {
    const actionKey = resolveIssueActionKey(selectedPlatform, [issue.code]);
    const id = workflowIssueID(issue.code, index, issueCodeOccurrences);
    return {
      id,
      severity:
        issue.severity === "blocking" || issue.severity === "review"
          ? "blocking"
          : "warning",
      title: issue.message || issue.code || "需要确认",
      ...(issue.detail ? { description: issue.detail } : {}),
      ...(actionKey ? { actionKey } : {}),
    } satisfies ProductWorkspaceReviewIssue;
  });

  const hasAuthoritativeWorkflowIssue = relevantWorkflowIssues.some(
    (issue) => issue.severity === "blocking" || issue.severity === "review",
  );
  if (hasAuthoritativeWorkflowIssue) {
    return workflowIssues;
  }

  const fallbackIssues = buildFallbackReviewIssues(task);
  if (workflowIssues.length === 0) {
    return fallbackIssues;
  }

  const workflowTitles = new Set(workflowIssues.map((issue) => issue.title.trim()));
  return [
    ...workflowIssues,
    ...fallbackIssues.filter((issue) => !workflowTitles.has(issue.title.trim())),
  ];
}

function buildFallbackReviewIssues(
  task: ListingKitTaskResult | null | undefined,
) {
  return extractTaskReviewReasons(task).map((reason, index) => {
    return {
      id: `fallback-review-${index + 1}`,
      severity: isReviewRequiredTaskStatus(task?.status) ? "blocking" : "warning",
      title: reason,
    } satisfies ProductWorkspaceReviewIssue;
  });
}

function isReviewRequiredTaskStatus(status?: string) {
  return status === "needs_review" || status === "review_ready";
}

function workflowIssueID(
  code: string | undefined,
  index: number,
  occurrences: Map<string, number>,
) {
  if (!code) {
    return `workflow-issue-${index + 1}`;
  }

  const occurrence = (occurrences.get(code) ?? 0) + 1;
  occurrences.set(code, occurrence);
  return occurrence === 1 ? code : `${code}-${occurrence}`;
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
