import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
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

type BatchQueueRequestVersionRef = {
  current: number;
};

type BatchQueueControllerDependencies = {
  getSavedBatches: () => SheinStudioSavedBatch[];
  getSelectedHydrations: () => Record<string, SheinStudioWorkbenchHydratedBatch>;
  hydrateBatchSelection: (
    batchIds: string[],
  ) => Promise<Record<string, SheinStudioWorkbenchHydratedBatch>>;
  loadBatch: (batch: SheinStudioSavedBatch) => void;
  loadHydratedBatch: (batch: SheinStudioWorkbenchHydratedBatch) => void;
  requestVersionRef: BatchQueueRequestVersionRef;
  recentOpenVersionRef: BatchQueueRequestVersionRef;
  setBatchQueueMode: (value: SheinStudioBatchQueueMode | null) => void;
  setEffectiveStep: (value: SheinStudioStepKey) => void;
  setQueueMessage: (value: string) => void;
  setQueueResumeState: (value: SheinStudioBatchQueueResumeState | null) => void;
  setQueuedBatchIds: (value: string[]) => void;
  setQueuedBatchIndex: (value: number) => void;
};

type LoadQueuedBatchOptions = {
  keepResumeState?: boolean;
  hydratedBatches?: Record<string, SheinStudioWorkbenchHydratedBatch>;
  requestVersion?: number;
};

type StartBatchQueueInput = {
  batchIds: string[];
  mode: SheinStudioBatchQueueMode;
  startIndex?: number;
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

export function createBatchQueueController({
  getSavedBatches,
  getSelectedHydrations,
  hydrateBatchSelection,
  loadBatch,
  loadHydratedBatch,
  requestVersionRef,
  recentOpenVersionRef,
  setBatchQueueMode,
  setEffectiveStep,
  setQueueMessage,
  setQueueResumeState,
  setQueuedBatchIds,
  setQueuedBatchIndex,
}: BatchQueueControllerDependencies) {
  function clear() {
    setBatchQueueMode(null);
    setQueuedBatchIds([]);
    setQueuedBatchIndex(0);
  }

  async function load(
    batchIds: string[],
    index: number,
    mode: SheinStudioBatchQueueMode,
    options?: LoadQueuedBatchOptions,
  ) {
    for (let nextIndex = index; nextIndex < batchIds.length; nextIndex += 1) {
      if (
        options?.requestVersion != null &&
        requestVersionRef.current !== options.requestVersion
      ) {
        return false;
      }
      const batchId = batchIds[nextIndex];
      const hydratedBatch =
        options?.hydratedBatches?.[batchId] ??
        getSelectedHydrations()[batchId] ??
        (await hydrateBatchSelection([batchId]))[batchId];
      if (
        options?.requestVersion != null &&
        requestVersionRef.current !== options.requestVersion
      ) {
        return false;
      }
      const queuedBatch = resolveNextQueuedBatch({
        batchIds: [batchId],
        savedBatches: getSavedBatches(),
        startIndex: 0,
      });
      const batch = hydratedBatch?.savedBatch ?? queuedBatch?.batch;
      if (!batch) {
        continue;
      }
      if (hydratedBatch) {
        loadHydratedBatch(hydratedBatch);
      } else {
        loadBatch(batch);
      }
      setQueuedBatchIndex(nextIndex);
      setEffectiveStep(resolveQueuedBatchStep(batch, mode));
      setQueueMessage("");
      return true;
    }
    clear();
    if (!options?.keepResumeState) {
      setQueueResumeState(null);
    }
    setQueueMessage(buildBatchQueueCompletionMessage(mode, batchIds.length));
    return false;
  }

  async function start(input: StartBatchQueueInput) {
    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;
    recentOpenVersionRef.current += 1;
    const queueStartState = getBatchQueueStartState({
      batchIds: input.batchIds,
      savedBatches: getSavedBatches(),
      startIndex: input.startIndex,
    });
    const validBatchIds = queueStartState.batchIds;
    if (validBatchIds.length === 0) {
      clear();
      setQueueResumeState(null);
      setQueueMessage(buildBatchQueueCompletionMessage(input.mode, 0));
      return;
    }
    const startIndex = queueStartState.startIndex;
    setBatchQueueMode(input.mode);
    setQueuedBatchIds(validBatchIds);
    setQueuedBatchIndex(startIndex);
    setQueueResumeState(null);
    const hydratedBatches = await hydrateBatchSelection(validBatchIds);
    if (requestVersionRef.current !== requestVersion) {
      return;
    }
    await load(validBatchIds, startIndex, input.mode, {
      hydratedBatches,
      requestVersion,
    });
  }

  function exit({
    batchIds,
    currentIndex,
    mode,
  }: {
    batchIds: string[];
    currentIndex: number;
    mode: SheinStudioBatchQueueMode | null;
  }) {
    const resumeState = buildBatchQueueResumeState({
      batchIds,
      mode,
      startIndex: currentIndex,
    });
    if (resumeState) {
      setQueueResumeState(resumeState);
      setQueueMessage("");
    }
    requestVersionRef.current += 1;
    clear();
  }

  function resume(state: SheinStudioBatchQueueResumeState | null) {
    if (!state) {
      return Promise.resolve();
    }
    return start({
      batchIds: state.batchIds,
      mode: state.mode,
      startIndex: state.startIndex,
    });
  }

  function advance({
    batchIds,
    currentIndex,
    mode,
  }: {
    batchIds: string[];
    currentIndex: number;
    mode: SheinStudioBatchQueueMode | null;
  }) {
    if (!mode) {
      return Promise.resolve(false);
    }
    return load(batchIds, currentIndex + 1, mode, {
      requestVersion: requestVersionRef.current,
    });
  }

  return {
    advance,
    clear,
    exit,
    load,
    resume,
    start,
  };
}
