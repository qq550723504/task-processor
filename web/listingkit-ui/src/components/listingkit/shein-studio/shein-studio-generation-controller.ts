import { useMemo } from "react";

import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  buildGroupedSDSBaselineHandoff,
  getSDSBaselineReasonMessage,
} from "@/lib/shein-studio/sds-baseline-ui";
import type { SDSBaselineReadiness } from "@/lib/types/sds-baseline";
import type {
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioGeneratedDesign,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

type BatchGenerationContext = {
  ensureBatch: () => Promise<SheinStudioSavedBatch | null>;
  startGenerationRun: (savedBatch: SheinStudioSavedBatch) => Promise<void>;
};

type BatchRunStarter = (
  batchIds: string[],
  mode: "generate",
) => Promise<{ run: { id: string } }>;

type BuildDraftInputOverrides = Partial<{
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  generationJobs: SheinStudioGenerationJob[];
  generationError: string;
  generationJobId: string;
}>;

type BaselineWarmupFeedback = {
  action: {
    intent: "focus_generate" | "open_sds_login" | "warm_baseline";
    label: string;
  } | null;
  message: string;
};

type BatchGenerationContextParams = {
  activeBatchId?: string;
  buildDraftInput: (overrides?: BuildDraftInputOverrides) => SheinStudioSaveInput;
  createdTasks?: SheinStudioCreatedTask[];
  currentGenerationJobId?: string;
  designs?: SheinStudioGeneratedDesign[];
  enabled: boolean;
  generationError?: string;
  generationJobs?: SheinStudioGenerationJob[];
  getHydratedBatch: (
    batchId: string,
  ) => Promise<SheinStudioWorkbenchHydratedBatch | null>;
  initialBatchId?: string;
  saveBatch: (
    input: SheinStudioSaveInput,
    options?: { makeActive?: boolean },
  ) => Promise<SheinStudioSavedBatch | null>;
  selectedIds?: string[];
  setActiveBatchId: (batchId: string) => void;
  setActiveBatchRunId: (runId: string) => void;
  setActiveSavedBatchId: (batchId: string) => void;
  setBatchRunError: (message: string) => void;
  setSavedBatches: (
    updater: (current: SheinStudioSavedBatch[]) => SheinStudioSavedBatch[],
  ) => void;
  startBatchRun: BatchRunStarter;
  upsertSavedBatch?: (
    current: SheinStudioSavedBatch[],
    savedBatch: SheinStudioSavedBatch,
  ) => SheinStudioSavedBatch[];
};

export function projectBaselineWarmupFeedback(
  readiness: Pick<SDSBaselineReadiness, "reason" | "reasonCode" | "status">,
): BaselineWarmupFeedback {
  const handoff = buildGroupedSDSBaselineHandoff({
    status: readiness.status,
    reason: readiness.reason,
    reasonCode: readiness.reasonCode,
  });
  return {
    action:
      handoff?.action && handoff.actionLabel
        ? {
            intent: handoff.action,
            label: handoff.actionLabel,
          }
        : null,
    message:
      readiness.status === "ready"
        ? "这款 SDS 商品的 baseline 已通过校验，现在可以继续加入 grouped 批量上品。"
        : readiness.status === "baseline_cached" &&
            !readiness.reason?.trim() &&
            !readiness.reasonCode?.trim()
          ? "这款 SDS 商品已经完成 baseline 缓存，当前没有更多校验结果。可以继续使用，必要时再手动复查。"
          : readiness.reason ||
            getSDSBaselineReasonMessage(readiness.reasonCode) ||
            "baseline 预热与校验已发起，请稍后再试。",
  };
}

function defaultUpsertSavedBatch(
  current: SheinStudioSavedBatch[],
  savedBatch: SheinStudioSavedBatch,
) {
  return [savedBatch, ...current.filter((batch) => batch.id !== savedBatch.id)].sort(
    (left, right) => right.updatedAt.localeCompare(left.updatedAt),
  );
}

export function useSheinStudioBatchGenerationContext({
  activeBatchId,
  buildDraftInput,
  createdTasks = [],
  currentGenerationJobId = "",
  designs = [],
  enabled,
  generationError = "",
  generationJobs = [],
  getHydratedBatch,
  initialBatchId,
  saveBatch,
  selectedIds = [],
  setActiveBatchId,
  setActiveBatchRunId,
  setActiveSavedBatchId,
  setBatchRunError,
  setSavedBatches,
  startBatchRun,
  upsertSavedBatch = defaultUpsertSavedBatch,
}: BatchGenerationContextParams): {
  batchGenerationContext?: BatchGenerationContext;
} {
  const batchGenerationContext = useMemo<BatchGenerationContext | undefined>(() => {
    if (!enabled) {
      return undefined;
    }

    return {
      ensureBatch: async () => {
        const currentBatchId = activeBatchId || initialBatchId || "";
        const latestHydratedBatch =
          currentBatchId && initialBatchId
            ? await getHydratedBatch(currentBatchId).catch(() => null)
            : null;
        const saved = await saveBatch(
          {
            ...buildDraftInput({
              designs,
              selectedIds,
              createdTasks,
              generationJobs,
              generationError,
              generationJobId: currentGenerationJobId,
            }),
            ...(currentBatchId ? { id: currentBatchId } : {}),
            updatedAt:
              latestHydratedBatch?.detail.batch.draftUpdatedAt ||
              latestHydratedBatch?.savedBatch.draftUpdatedAt ||
              latestHydratedBatch?.savedBatch.updatedAt ||
              buildDraftInput().updatedAt,
          },
          currentBatchId ? { makeActive: false } : undefined,
        );
        if (!saved) {
          return null;
        }
        setActiveBatchId(saved.id);
        setActiveSavedBatchId(saved.id);
        setSavedBatches((current) => upsertSavedBatch(current, saved));
        return saved;
      },
      startGenerationRun: async (savedBatch) => {
        setBatchRunError("");
        const response = await startBatchRun([savedBatch.id], "generate");
        setActiveBatchRunId(response.run.id);
      },
    };
  }, [
    activeBatchId,
    buildDraftInput,
    createdTasks,
    currentGenerationJobId,
    designs,
    enabled,
    generationError,
    generationJobs,
    getHydratedBatch,
    initialBatchId,
    saveBatch,
    selectedIds,
    setActiveBatchId,
    setActiveBatchRunId,
    setActiveSavedBatchId,
    setBatchRunError,
    setSavedBatches,
    startBatchRun,
    upsertSavedBatch,
  ]);

  return {
    batchGenerationContext,
  };
}
