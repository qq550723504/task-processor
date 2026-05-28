import { pickActiveSheinStudioGroup } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { buildRecentBatchBaselineAlert } from "@/lib/shein-studio/sds-baseline-ui";
import type {
  SheinStudioDraft,
  SheinStudioGroupedWorkspace,
  SheinStudioRecentBatchAlert,
  SheinStudioRecentBatchResult,
  SheinStudioRecentBatchSummary,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

function buildStoreSummaryFromAssignments(assignments: string[]) {
  const normalized = Array.from(
    new Set(assignments.map((item) => item.trim()).filter(Boolean)),
  );
  if (normalized.length === 0) {
    return "跟随当前店铺";
  }
  if (normalized.length === 1) {
    return normalized[0];
  }
  return "跨店铺分发";
}

function pickSummaryGroup(groups?: SheinStudioGroupedWorkspace[]) {
  return groups?.length ? pickActiveSheinStudioGroup(groups) : null;
}

function buildGroupedSelectionAlerts(
  groupedSelections: {
    eligible?: boolean;
    eligibilityReason?: string;
    baselineStatus?: string;
    baselineReason?: string;
    baselineReasonCode?: string;
}[],
) {
  const alerts: SheinStudioRecentBatchAlert[] = [];
  const baselineIssue = groupedSelections.find(
    (item) => item.baselineStatus && item.baselineStatus !== "ready",
  );
  const baselineAlert = baselineIssue
    ? buildRecentBatchBaselineAlert({
        status: baselineIssue.baselineStatus,
        reason: baselineIssue.baselineReason,
        reasonCode: baselineIssue.baselineReasonCode,
      })
    : null;
  if (baselineAlert) {
    alerts.push(baselineAlert);
  }
  if (groupedSelections.some((item) => item.eligible === false)) {
    const reason = groupedSelections.find((item) => item.eligible === false)
      ?.eligibilityReason;
    alerts.push({
      tone: "warning",
      label: "Grouped 商品待处理",
      detail: reason?.trim() || "组内仍有商品暂时不能加入当前批次处理。",
    });
  }
  return alerts;
}

function buildSelectionReviewAlert(input: {
  designCount: number;
  selectedIds: string[];
}) {
  if (input.designCount > 0 && input.selectedIds.length === 0) {
    return {
      tone: "warning",
      label: "待确认款式",
      detail: "这批已经生成设计，但还没有确认要继续创建任务的款式。",
    } satisfies SheinStudioRecentBatchAlert;
  }
  return null;
}

function buildRecentGenerationResult(input: {
  designCount: number;
  generationError?: string;
  sessionStatus?: string;
}): SheinStudioRecentBatchResult | null {
  if (input.sessionStatus === "generating") {
    return {
      tone: "warning",
      label: "最近生成中",
      detail: "当前仍在生成设计。",
    } satisfies SheinStudioRecentBatchResult;
  }
  if (input.generationError?.trim()) {
    return {
      tone: "danger",
      label: "最近生成失败",
      detail: input.generationError.trim(),
    } satisfies SheinStudioRecentBatchResult;
  }
  if (input.designCount > 0) {
    return {
      tone: "success",
      label: "最近生成成功",
      detail: `已生成 ${input.designCount} 张设计。`,
    } satisfies SheinStudioRecentBatchResult;
  }
  return null;
}

function buildRecentTaskResult(input: {
  designCount: number;
  selectedIds: string[];
  createdTaskCount: number;
}): SheinStudioRecentBatchResult | null {
  if (input.createdTaskCount > 0) {
    return {
      tone: "success",
      label: "最近任务已创建",
      detail: `已创建 ${input.createdTaskCount} 个 SHEIN 资料任务。`,
    } satisfies SheinStudioRecentBatchResult;
  }
  if (input.designCount > 0 && input.selectedIds.length > 0) {
    return {
      tone: "warning",
      label: "待创建任务",
      detail: `已确认 ${input.selectedIds.length} 个款式，尚未创建任务。`,
    } satisfies SheinStudioRecentBatchResult;
  }
  return null;
}

function buildRecentResults(input: {
  designCount: number;
  generationError?: string;
  sessionStatus?: string;
  selectedIds: string[];
  createdTaskCount: number;
}) {
  return [
    buildRecentGenerationResult(input),
    buildRecentTaskResult(input),
  ].filter((item): item is SheinStudioRecentBatchResult => item != null);
}

function buildPersistedBatchAlerts(batch: SheinStudioSavedBatch) {
  const group = pickSummaryGroup(batch.groups);
  const groupedSelections = group?.groupedSelections ?? batch.groupedSelections ?? [];
  const alerts = buildGroupedSelectionAlerts(groupedSelections);
  const reviewAlert = buildSelectionReviewAlert({
    designCount: group?.designs.length ?? batch.designs.length,
    selectedIds: group?.selectedIds ?? batch.selectedIds,
  });
  if (reviewAlert) {
    alerts.push(reviewAlert);
  }
  return alerts;
}

function buildRecoverableDraftAlerts(
  draft: SheinStudioDraft,
  group: SheinStudioGroupedWorkspace,
) {
  const alerts: SheinStudioRecentBatchAlert[] = [
    {
      tone: "warning",
      label: "未保存草稿",
      detail: "当前恢复的是本地草稿，建议尽快保存成批次以免后续丢失。",
    },
    ...buildGroupedSelectionAlerts(group.groupedSelections),
  ];
  if (draft.generationError?.trim()) {
    alerts.push({
      tone: "danger",
      label: "生成失败",
      detail: draft.generationError.trim(),
    });
  }
  const reviewAlert = buildSelectionReviewAlert({
    designCount: group.designs.length,
    selectedIds: group.selectedIds,
  });
  if (reviewAlert) {
    alerts.push(reviewAlert);
  }
  return alerts;
}

function buildPersistedBatchSummary(
  batch: SheinStudioSavedBatch,
): SheinStudioRecentBatchSummary {
  const group = pickSummaryGroup(batch.groups);
  const primaryProductName =
    group?.primarySelection.productName ??
    batch.selection?.productName ??
    "未命名 SDS 商品";
  const productCount = group
    ? group.groupedSelections.length + 1
    : (batch.groupedSelections?.length ?? 0) + (batch.selection?.variantId ? 1 : 0);
  const promptPreview =
    group?.currentPrompt?.trim() || batch.prompt.trim() || "暂未填写";
  const storeAssignments = group
    ? [group.sheinStoreId, ...group.groupedSelections.map((item) => item.sheinStoreId)]
    : [
        batch.sheinStoreId,
        ...(batch.groupedSelections ?? []).map((item) => item.sheinStoreId),
      ];
  const designCount = group?.designs.length ?? batch.designs.length;
  const selectedIds = group?.selectedIds ?? batch.selectedIds;
  const createdTaskCount = group?.createdTasks.length ?? batch.createdTasks.length;

  return {
    id: batch.id,
    source: "batch",
    isRecoverableDraft: false,
    title: batch.name?.trim() || primaryProductName,
    primaryProductName,
    productCount,
    promptPreview,
    storeSummary: buildStoreSummaryFromAssignments(storeAssignments),
    designCount,
    createdTaskCount,
    updatedAt: group?.updatedAt ?? batch.updatedAt,
    alerts: buildPersistedBatchAlerts(batch),
    recentResults: buildRecentResults({
      designCount,
      selectedIds,
      createdTaskCount,
    }),
  };
}

function buildRecoverableDraftSummary(
  draft: SheinStudioDraft,
): SheinStudioRecentBatchSummary | null {
  const group = pickSummaryGroup(draft.groups);
  if (!group) {
    return null;
  }
  return {
    id: `local-draft:${group.id}`,
    source: "local_draft",
    isRecoverableDraft: true,
    title: group.name?.trim() || "未保存草稿",
    primaryProductName: group.primarySelection.productName || "未命名 SDS 商品",
    productCount: group.groupedSelections.length + 1,
    promptPreview: group.currentPrompt.trim() || draft.prompt.trim() || "暂未填写",
    storeSummary: buildStoreSummaryFromAssignments([
      group.sheinStoreId,
      ...group.groupedSelections.map((item) => item.sheinStoreId),
    ]),
    designCount: group.designs.length,
    createdTaskCount: group.createdTasks.length,
    updatedAt: group.updatedAt || draft.updatedAt,
    alerts: buildRecoverableDraftAlerts(draft, group),
    recentResults: buildRecentResults({
      designCount: group.designs.length,
      generationError: draft.generationError,
      sessionStatus: draft.sessionStatus,
      selectedIds: group.selectedIds,
      createdTaskCount: group.createdTasks.length,
    }),
  };
}

export function buildRecentBatchSummaries(
  batches: SheinStudioSavedBatch[],
  options?: {
    draft?: SheinStudioDraft | null;
  },
) {
  const summaries = batches.map(buildPersistedBatchSummary);
  const draftSummary = options?.draft
    ? buildRecoverableDraftSummary(options.draft)
    : null;
  const merged = draftSummary ? [draftSummary, ...summaries] : summaries;

  return merged.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
}
