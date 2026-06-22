import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import type {
  SheinStudioBatchQueueMode,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

export type SheinStudioBatchQueueResumeState = {
  batchIds: string[];
  mode: SheinStudioBatchQueueMode;
  startIndex: number;
  total: number;
};

export function getBatchQueueStartState({
  batchIds,
  savedBatches,
  startIndex = 0,
}: {
  batchIds: string[];
  savedBatches: SheinStudioSavedBatch[];
  startIndex?: number;
}) {
  const validBatchIds = batchIds.filter((batchId) =>
    savedBatches.some((item) => item.id === batchId),
  );
  if (validBatchIds.length === 0) {
    return {
      batchIds: [],
      startIndex: 0,
    };
  }
  return {
    batchIds: validBatchIds,
    startIndex: Math.max(0, Math.min(startIndex, validBatchIds.length - 1)),
  };
}

export function resolveNextQueuedBatch({
  batchIds,
  savedBatches,
  startIndex,
}: {
  batchIds: string[];
  savedBatches: SheinStudioSavedBatch[];
  startIndex: number;
}) {
  for (let index = startIndex; index < batchIds.length; index += 1) {
    const batchId = batchIds[index];
    const batch = savedBatches.find((item) => item.id === batchId);
    if (batch) {
      return {
        batch,
        batchId,
        index,
      };
    }
  }
  return null;
}

export function resolveQueuedBatchStep(
  batch: SheinStudioSavedBatch,
  mode: SheinStudioBatchQueueMode,
): SheinStudioStepKey {
  if (batch.createdTasks.length > 0) {
    return "tasks";
  }
  if (batch.designs.length > 0) {
    return "review";
  }
  if (mode === "generate") {
    return "generate";
  }
  return "generate";
}

export function buildBatchQueueResumeState({
  batchIds,
  mode,
  startIndex,
}: {
  batchIds: string[];
  mode: SheinStudioBatchQueueMode | null;
  startIndex: number;
}): SheinStudioBatchQueueResumeState | null {
  if (!mode || batchIds.length === 0) {
    return null;
  }
  return {
    batchIds,
    mode,
    startIndex,
    total: batchIds.length,
  };
}

export function buildBatchQueueCompletionMessage(
  mode: SheinStudioBatchQueueMode,
  batchCount: number,
) {
  const actionLabel = mode === "create_tasks" ? "创建任务处理" : "继续生成处理";
  return batchCount > 0
    ? `已完成这轮${actionLabel}，共处理 ${batchCount} 个已保存批次。首页勾选已保留，可继续调整或再次发起批量处理。`
    : `当前没有可继续的已保存批次。首页勾选已保留，可重新检查后再发起批量处理。`;
}
