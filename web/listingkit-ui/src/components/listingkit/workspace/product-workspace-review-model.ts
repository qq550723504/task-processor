import {
  normalizeSheinWorkspaceActionKey,
  type SheinWorkspaceActionKey,
} from "@/components/listingkit/shein/shein-workspace-actions";
import { extractTaskReviewReasons } from "@/components/listingkit/tasks/task-review-reasons";
import type { ProductWorkspaceAttentionSeverity } from "@/components/listingkit/workspace/product-workspace-model";
import type { CanonicalProduct, ListingKitTaskResult } from "@/lib/types/listingkit";

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

const SHEIN_EXPLICIT_ACTION_CODES = new Set([
  "store_login",
  "store",
  "shein_online_auth",
  "category",
  "category_review",
  "attributes",
  "attribute",
  "attribute_review",
  "sale_attributes",
  "sale_attribute",
  "variants",
  "sku",
  "images",
  "image",
  "preview_product",
  "final_images",
  "pod_platform",
  "pricing",
  "price",
  "inventory",
  "variant_mapping",
  "required_attribute",
  "shein_category_template_freshness",
  "shein_attribute_template_freshness",
  "shein_sale_attribute_template_freshness",
  "shein_sale_attribute_freshness",
]);

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
    const actionKey = resolveIssueActionKey(selectedPlatform, [issue.code], issue.stage);
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
  const reasons = [
    ...extractTaskReviewReasons(task),
    ...canonicalProductReviewReasons(task?.result?.canonical_product),
  ].filter((reason, index, values) => values.indexOf(reason) === index);

  return reasons.map((reason, index) => {
    return {
      id: `fallback-review-${index + 1}`,
      severity: isMandatoryFallbackReview(task) ? "blocking" : "warning",
      title: reason,
    } satisfies ProductWorkspaceReviewIssue;
  });
}

function isReviewRequiredTaskStatus(status?: string) {
  return (
    status === "needs_review" ||
    status === "review_ready" ||
    status === "failed" ||
    status === "error"
  );
}

function isMandatoryFallbackReview(task?: ListingKitTaskResult | null) {
  return (
    isReviewRequiredTaskStatus(task?.status) ||
    Boolean(task?.result?.canonical_product?.needs_review)
  );
}

function canonicalProductReviewReasons(
  product?: CanonicalProduct | null,
) {
  if (!product) {
    return [];
  }

  const fieldReasons = Object.entries(product.field_traces ?? {})
    .filter(([, trace]) => trace.needs_review)
    .map(([field, trace]) => trace.review_reason?.trim() || `${field}需要确认`);

  const missingRequiredFieldReasons = product.needs_review
    ? [
        product.title?.trim() ? undefined : "商品标题缺失",
        product.description?.trim() ? undefined : "商品描述缺失",
      ].filter((reason): reason is string => Boolean(reason))
    : [];

  if (fieldReasons.length > 0 || missingRequiredFieldReasons.length > 0) {
    return [...fieldReasons, ...missingRequiredFieldReasons];
  }
  return product.needs_review ? ["商品资料需要确认"] : [];
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
  stage?: string,
): SheinWorkspaceActionKey | undefined {
  if (selectedPlatform !== "shein") {
    return undefined;
  }

  const normalizedStage = stage?.trim().toLowerCase();
  const isSheinStage =
    normalizedStage === "shein" ||
    normalizedStage === "shein_review" ||
    normalizedStage?.startsWith("shein_") === true ||
    normalizedStage?.endsWith(":shein") === true;
  if (
    !isSheinStage &&
    !candidates.some((candidate) =>
      SHEIN_EXPLICIT_ACTION_CODES.has(candidate?.trim().toLowerCase() ?? ""),
    )
  ) {
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
