import { pickActiveSheinStudioGroup } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import type {
  SheinStudioDraft,
  SheinStudioGroupedWorkspace,
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

  return {
    id: batch.id,
    source: "batch",
    isRecoverableDraft: false,
    title: batch.name?.trim() || primaryProductName,
    primaryProductName,
    productCount,
    promptPreview,
    storeSummary: buildStoreSummaryFromAssignments(storeAssignments),
    designCount: group?.designs.length ?? batch.designs.length,
    createdTaskCount: group?.createdTasks.length ?? batch.createdTasks.length,
    updatedAt: group?.updatedAt ?? batch.updatedAt,
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
