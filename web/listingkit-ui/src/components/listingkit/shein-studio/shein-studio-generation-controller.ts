import { useMemo } from "react";

import { upsertRecentSavedBatch } from "@/components/listingkit/shein-studio/shein-studio-recent-batch-controller";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  buildGroupedSDSBaselineHandoff,
  getSDSBaselineReasonMessage,
} from "@/lib/shein-studio/sds-baseline-ui";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  buildGroupedSDSSelectionID,
  type GroupedSDSSelectionEligibility,
  type SDSBaselineReadiness,
  type SDSBaselineReadinessRequest,
  type SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
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

type ActiveSelectionBaseline = {
  baselineKey?: string;
  reason: string;
  reasonCode?: string;
  status: SDSBaselineStatus;
};

type BaselineReadinessEntry = readonly [string, ActiveSelectionBaseline];

type ResolveBaselineReadinessEntriesParams = {
  getReadiness: (
    request: SDSBaselineReadinessRequest,
  ) => Promise<SDSBaselineReadiness>;
  selections: SDSProductVariantSelection[];
};

type BaselineWarmupRunnerParams = {
  activeSelection?: SDSProductVariantSelection;
  baselineStatuses: Record<string, ActiveSelectionBaseline>;
  warmBaseline: (
    selection: SDSProductVariantSelection,
  ) => Promise<SDSBaselineReadiness>;
};

type BaselineWarmupResult = Awaited<ReturnType<typeof runBaselineWarmup>>;

type ApplyBaselineWarmupResultParams = {
  result: BaselineWarmupResult;
  setBaselineStatuses: (
    statuses: Record<string, ActiveSelectionBaseline>,
  ) => void;
  setGenerationWarning: (message: string) => void;
  setGenerationWarningAction: (action: BaselineWarmupFeedback["action"]) => void;
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

export function projectActiveSelectionBaselineState({
  activeGroupedSelectionID,
  baselineStatuses,
  hasActiveSelection,
}: {
  activeGroupedSelectionID: string;
  baselineStatuses: Record<string, ActiveSelectionBaseline>;
  hasActiveSelection: boolean;
}) {
  const resolvedBaseline = activeGroupedSelectionID
    ? baselineStatuses[activeGroupedSelectionID]
    : undefined;
  const baseline = resolvedBaseline ?? {
    status: "missing" as SDSBaselineStatus,
    reasonCode: undefined,
    reason: hasActiveSelection ? "正在检查 baseline 状态..." : "",
  };
  const reason = baseline.reason || getSDSBaselineReasonMessage(baseline.reasonCode);
  const handoff = resolvedBaseline
    ? buildGroupedSDSBaselineHandoff({
        status: resolvedBaseline.status,
        reason: resolvedBaseline.reason,
        reasonCode: resolvedBaseline.reasonCode,
      })
    : null;
  return {
    baseline,
    handoff,
    reason,
    resolvedBaseline,
  };
}

export async function resolveBaselineReadinessEntries({
  getReadiness,
  selections,
}: ResolveBaselineReadinessEntriesParams): Promise<BaselineReadinessEntry[]> {
  return Promise.all(
    selections.map(async (item) => {
      const selectionId = buildGroupedSDSSelectionID(item);
      try {
        const readiness = await getReadiness({
          parentProductId: item.parentProductId,
          prototypeGroupId: item.prototypeGroupId,
          variantId: item.variantId,
          selectedVariantIds: item.selectedVariantIds,
        });
        return [
          selectionId,
          {
            status: readiness.status,
            reason: readiness.reason ?? "",
            reasonCode: readiness.reasonCode,
            baselineKey: readiness.baselineKey,
          },
        ] as const;
      } catch (error) {
        return [
          selectionId,
          {
            status: "failed" as SDSBaselineStatus,
            reasonCode: undefined,
            reason:
              error instanceof Error
                ? error.message
                : "读取 SDS baseline 状态失败。",
          },
        ] as const;
      }
    }),
  );
}

export async function runBaselineWarmup({
  activeSelection,
  baselineStatuses,
  warmBaseline,
}: BaselineWarmupRunnerParams): Promise<
  | {
      baselineStatuses: Record<string, ActiveSelectionBaseline>;
      feedback: BaselineWarmupFeedback;
    }
  | {
      warning: string;
    }
  | null
> {
  if (!activeSelection?.variantId) {
    return null;
  }
  const activeSelectionId = buildGroupedSDSSelectionID(activeSelection);
  try {
    const readiness = await warmBaseline(activeSelection);
    return {
      baselineStatuses: {
        ...baselineStatuses,
        [activeSelectionId]: {
          status: readiness.status,
          reason: readiness.reason ?? "",
          reasonCode: readiness.reasonCode,
          baselineKey: readiness.baselineKey,
        },
      },
      feedback: projectBaselineWarmupFeedback(readiness),
    };
  } catch (error) {
    return {
      warning:
        error instanceof Error ? error.message : "baseline 预热失败。",
    };
  }
}

export function applyBaselineWarmupResult({
  result,
  setBaselineStatuses,
  setGenerationWarning,
  setGenerationWarningAction,
}: ApplyBaselineWarmupResultParams) {
  if (!result) {
    return;
  }
  if ("baselineStatuses" in result) {
    setBaselineStatuses(result.baselineStatuses);
    setGenerationWarning(result.feedback.message);
    setGenerationWarningAction(result.feedback.action);
    return;
  }
  setGenerationWarning(result.warning);
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
  upsertSavedBatch = upsertRecentSavedBatch,
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
