import {
  normalizeSheinFreshnessActionKey,
  normalizeSheinWorkspaceActionKey,
  normalizeSheinReadinessItemWorkspaceActionKey,
  sheinWorkspaceTargetIdForKey,
  type SheinFreshnessActionKey,
  type SheinWorkspaceActionKey,
} from "@/components/listingkit/shein/shein-workspace-actions";
import {
  extractTaskReviewReasons,
  inferSheinReviewActionKey,
} from "@/components/listingkit/tasks/task-review-reasons";
import type { ProductWorkspaceAttentionSeverity } from "@/components/listingkit/workspace/product-workspace-model";
import type {
  CanonicalFieldTrace,
  CanonicalProduct,
  ListingKitTaskResult,
  SheinReadinessItem,
  SheinSubmitReadiness,
} from "@/lib/types/listingkit";

export type ProductWorkspaceReviewIssue = {
  id: string;
  severity: Exclude<ProductWorkspaceAttentionSeverity, "passed">;
  title: string;
  description?: string;
  suggestion?: string;
  evidence?: string;
  confidence?: number;
  actionKey?: SheinWorkspaceActionKey | SheinFreshnessActionKey;
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
  readiness?: SheinSubmitReadiness | null,
): ProductWorkspaceReviewIssue[] {
  const relevantWorkflowIssues = (task?.result?.workflow_issues ?? []).filter(
    (issue) =>
      issue.severity === "blocking" ||
      issue.severity === "warning" ||
      issue.severity === "review",
  );
  const issueCodeOccurrences = new Map<string, number>();
  const workflowIssues = relevantWorkflowIssues.map((issue, index) => {
    const actionKey = resolveIssueActionKey(
      selectedPlatform,
      [issue.code, issue.message, issue.detail],
      issue.stage,
    );
    const id = workflowIssueID(issue.code, index, issueCodeOccurrences);
    return {
      id,
      severity: issue.severity === "blocking" ? "blocking" : "warning",
      title: issue.message || issue.code || "需要确认",
      ...(issue.detail ? { description: issue.detail } : {}),
      ...(actionKey ? { actionKey } : {}),
    } satisfies ProductWorkspaceReviewIssue;
  });

  const readinessIssues = buildReadinessIssues(readiness);
  const fallbackIssues = buildFallbackReviewIssues(task, selectedPlatform);
  const existingTitles = new Set(
    [...workflowIssues, ...readinessIssues].map((issue) => issue.title.trim()),
  );
  if (workflowIssues.length === 0 && readinessIssues.length === 0) {
    return fallbackIssues;
  }

  return [
    ...workflowIssues,
    ...readinessIssues,
    ...fallbackIssues.filter((issue) => !existingTitles.has(issue.title.trim())),
  ];
}

function buildReadinessIssues(readiness?: SheinSubmitReadiness | null) {
  const issues = [
    ...mapReadinessItems(readiness?.blocking_items, "blocking"),
    ...mapReadinessItems(readiness?.warning_items, "warning"),
  ];
  const seenTargets = new Set<string>();
  return issues.filter((issue) => {
    const freshnessActionKey = normalizeSheinFreshnessActionKey(issue.actionKey);
    const workspaceActionKey = normalizeSheinWorkspaceActionKey(issue.actionKey);
    const identity = freshnessActionKey
      ? `freshness:${freshnessActionKey}`
      : workspaceActionKey
      ? `target:${sheinWorkspaceTargetIdForKey(workspaceActionKey)}`
      : `issue:${issue.severity}:${issue.title}`;
    if (seenTargets.has(identity)) {
      return false;
    }
    seenTargets.add(identity);
    return true;
  });
}

function mapReadinessItems(
  items: SheinReadinessItem[] | undefined,
  severity: ProductWorkspaceReviewIssue["severity"],
) {
  return (items ?? []).map((item, index) => {
    const actionKey = readinessItemActionKey(item);
    const title = item.label?.trim() || item.message?.trim() || item.key?.trim() || "需要处理";
    const description = item.message?.trim() || item.reason?.summary?.trim();
    return {
      id: `readiness-${severity}-${index + 1}`,
      severity,
      title,
      ...(description ? { description } : {}),
      ...(actionKey ? { actionKey } : {}),
    } satisfies ProductWorkspaceReviewIssue;
  });
}

function buildFallbackReviewIssues(
  task: ListingKitTaskResult | null | undefined,
  selectedPlatform?: string,
) {
  const reasons = [
    ...extractTaskReviewReasons(task),
    ...canonicalProductReviewReasons(task?.result?.canonical_product),
  ].filter((reason, index, values) => values.indexOf(reason) === index);

  return reasons.map((reason, index) => {
    const actionKey =
      selectedPlatform?.trim().toLowerCase() === "shein"
        ? inferSheinReviewActionKey(reason)
        : undefined;
    return {
      id: `fallback-review-${index + 1}`,
      severity: isMandatoryFallbackReview(task) ? "blocking" : "warning",
      title: reason,
      ...(actionKey ? { actionKey } : {}),
    } satisfies ProductWorkspaceReviewIssue;
  });
}

function readinessItemActionKey(
  item: SheinReadinessItem,
): SheinWorkspaceActionKey | SheinFreshnessActionKey | false {
  const freshnessActionKey = [
    item.taxonomy?.repair_target,
    item.suggested_action,
    item.taxonomy?.blocker_key,
    item.key,
  ]
    .map((key) => normalizeSheinFreshnessActionKey(key))
    .find(
      (key): key is Exclude<SheinFreshnessActionKey, "shein_online_auth"> =>
        Boolean(key) && key !== "shein_online_auth",
    );
  return freshnessActionKey || normalizeSheinReadinessItemWorkspaceActionKey(item);
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
  const nestedTraceReasons = [
    ...Object.entries(product.attributes ?? {}).map(([name, attribute]) =>
      reviewReasonForTrace(attribute.trace, `属性 ${name}需要确认`),
    ),
    ...(product.variants ?? []).flatMap((variant, index) => {
      const variantLabel = variant.sku?.trim() || `变体 ${index + 1}`;
      return [
        reviewReasonForTrace(variant.trace, `变体 ${variantLabel}需要确认`),
        ...Object.entries(variant.attributes ?? {}).map(([name, attribute]) =>
          reviewReasonForTrace(
            attribute.trace,
            `变体属性 ${variantLabel}/${name}需要确认`,
          ),
        ),
      ];
    }),
  ].filter((reason): reason is string => Boolean(reason));

  const missingRequiredFieldReasons = product.needs_review
    ? [
        product.title?.trim() ? undefined : "商品标题缺失",
        product.description?.trim() ? undefined : "商品描述缺失",
      ].filter((reason): reason is string => Boolean(reason))
    : [];

  if (
    fieldReasons.length > 0 ||
    nestedTraceReasons.length > 0 ||
    missingRequiredFieldReasons.length > 0
  ) {
    return [...fieldReasons, ...nestedTraceReasons, ...missingRequiredFieldReasons];
  }
  return product.needs_review ? ["商品资料需要确认"] : [];
}

function reviewReasonForTrace(
  trace: CanonicalFieldTrace | undefined,
  fallback: string,
) {
  if (!trace?.needs_review) {
    return undefined;
  }
  return trace.review_reason?.trim() || fallback;
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
  const normalizedStage = stage?.trim().toLowerCase();
  const isSheinStage =
    normalizedStage === "shein" ||
    normalizedStage === "shein_review" ||
    normalizedStage?.startsWith("shein_") === true ||
    normalizedStage?.endsWith(":shein") === true;
  const isSheinCode = candidates.some((candidate) =>
    candidate?.trim().toLowerCase().startsWith("shein_")
  );
  if (selectedPlatform?.trim().toLowerCase() !== "shein" && !isSheinStage && !isSheinCode) {
    return undefined;
  }
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
