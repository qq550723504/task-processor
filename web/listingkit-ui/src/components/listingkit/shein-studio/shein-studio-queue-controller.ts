import { useCallback, useEffect, type RefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  createBatchQueueController,
  type SheinStudioBatchQueueResumeState,
} from "@/lib/shein-studio/batch-queue";
import type {
  SheinStudioBatchQueueMode,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

type BatchRunStarter = (
  batchIds: string[],
  mode: SheinStudioBatchQueueMode,
) => Promise<{ run: { id: string } }>;

type QueueRequestVersionRef = {
  current: number;
};

type QueueControllerParams = {
  batchQueueMode: SheinStudioBatchQueueMode | null;
  currentQueuedBatchId: string;
  effectiveStep: SheinStudioStepKey;
  hydrateBatchSelection: (
    batchIds: string[],
  ) => Promise<Record<string, SheinStudioWorkbenchHydratedBatch>>;
  loadBatch: (batch: SheinStudioSavedBatch) => void;
  loadHydratedBatch: (batch: SheinStudioWorkbenchHydratedBatch) => void;
  queueResumeState: SheinStudioBatchQueueResumeState | null;
  queuedBatchIds: string[];
  queuedBatchIndex: number;
  recentOpenVersionRef: QueueRequestVersionRef;
  requestVersionRef: QueueRequestVersionRef;
  savedBatches: SheinStudioSavedBatch[];
  selectedRecentBatchHydrations: Record<string, SheinStudioWorkbenchHydratedBatch>;
  getBatchRunStartErrorMessage: (error: unknown) => string;
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  setActiveBatchRunId: (value: string) => void;
  setBatchQueueMode: (value: SheinStudioBatchQueueMode | null) => void;
  setBatchRunError: (value: string) => void;
  setEffectiveStep: (value: SheinStudioStepKey) => void;
  setQueueMessage: (value: string) => void;
  setQueueResumeState: (value: SheinStudioBatchQueueResumeState | null) => void;
  setQueuedBatchIds: (value: string[]) => void;
  setQueuedBatchIndex: (value: number) => void;
  setSelectedRecentBatchSummaryIds: (value: string[]) => void;
  startBatchRun: BatchRunStarter;
};

type QueueProjectionParams = {
  batchQueueMode: SheinStudioBatchQueueMode | null;
  effectiveStep: SheinStudioStepKey;
  queueResumeState: SheinStudioBatchQueueResumeState | null;
  queuedBatchIds: string[];
  queuedBatchIndex: number;
  savedBatches: SheinStudioSavedBatch[];
};

export function projectSheinStudioQueueState({
  batchQueueMode,
  effectiveStep,
  queueResumeState,
  queuedBatchIds,
  queuedBatchIndex,
  savedBatches,
}: QueueProjectionParams) {
  const currentQueuedBatchId = batchQueueMode
    ? queuedBatchIds[queuedBatchIndex] ?? ""
    : "";
  const currentQueuedBatch =
    savedBatches.find((item) => item.id === currentQueuedBatchId) ?? null;
  const resumableQueueBatchIds = queueResumeState
    ? queueResumeState.batchIds.filter((batchId) =>
        savedBatches.some((item) => item.id === batchId),
      )
    : [];
  let batchQueueGuidance =
    "当前批次还没有可用设计，已回到生成区继续处理。";
  if (effectiveStep === "tasks") {
    batchQueueGuidance = "已定位到任务区，可继续查看已创建的任务。";
  } else if (effectiveStep === "review") {
    batchQueueGuidance = "已定位到审核区，可直接创建任务或调整款式。";
  } else if (batchQueueMode === "generate") {
    batchQueueGuidance = "已定位到生成区，可直接修改提示词或继续生成。";
  }

  return {
    batchQueueGuidance,
    currentQueuedBatch,
    currentQueuedBatchId,
    resumableQueueBatchIds,
  };
}

export function useSheinStudioQueueController({
  batchQueueMode,
  currentQueuedBatchId,
  effectiveStep,
  getBatchRunStartErrorMessage,
  hydrateBatchSelection,
  loadBatch,
  loadHydratedBatch,
  queueResumeState,
  queuedBatchIds,
  queuedBatchIndex,
  recentOpenVersionRef,
  requestVersionRef,
  savedBatches,
  selectedRecentBatchHydrations,
  promptInputRef,
  setActiveBatchRunId,
  setBatchQueueMode,
  setBatchRunError,
  setEffectiveStep,
  setQueueMessage,
  setQueueResumeState,
  setQueuedBatchIds,
  setQueuedBatchIndex,
  setSelectedRecentBatchSummaryIds,
  startBatchRun,
}: QueueControllerParams) {
  useEffect(() => {
    if (!batchQueueMode || !currentQueuedBatchId) {
      return;
    }
    const timer = window.setTimeout(() => {
      if (batchQueueMode === "generate" || effectiveStep === "generate") {
        document
          .getElementById("shein-studio-generator")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
        const promptInput =
          promptInputRef.current ??
          (document.getElementById("prompt") as HTMLInputElement | HTMLTextAreaElement | null);
        promptInput?.focus();
        return;
      }
      if (effectiveStep === "review") {
        document
          .getElementById("shein-style-review")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
        return;
      }
      if (effectiveStep === "tasks") {
        document
          .getElementById("shein-created-tasks")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
      }
    }, 0);
    return () => {
      window.clearTimeout(timer);
    };
  }, [batchQueueMode, currentQueuedBatchId, effectiveStep, promptInputRef]);

  const getBatchQueueController = useCallback(
    () =>
      createBatchQueueController({
        getSavedBatches: () => savedBatches,
        getSelectedHydrations: () => selectedRecentBatchHydrations,
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
      }),
    [
      hydrateBatchSelection,
      loadBatch,
      loadHydratedBatch,
      recentOpenVersionRef,
      requestVersionRef,
      savedBatches,
      selectedRecentBatchHydrations,
      setBatchQueueMode,
      setEffectiveStep,
      setQueueMessage,
      setQueueResumeState,
      setQueuedBatchIds,
      setQueuedBatchIndex,
    ],
  );

  const handleOpenBatchQueue = useCallback(
    (input: { batchIds: string[]; mode: SheinStudioBatchQueueMode }) => {
      setBatchRunError("");
      void startBatchRun(input.batchIds, input.mode)
        .then((response) => {
          setQueueResumeState(null);
          setActiveBatchRunId(response.run.id);
        })
        .catch((error) => {
          setBatchRunError(getBatchRunStartErrorMessage(error));
        });
    },
    [
      getBatchRunStartErrorMessage,
      setActiveBatchRunId,
      setBatchRunError,
      setQueueResumeState,
      startBatchRun,
    ],
  );

  const handleExitBatchQueue = useCallback(() => {
    getBatchQueueController().exit({
      batchIds: queuedBatchIds,
      currentIndex: queuedBatchIndex,
      mode: batchQueueMode,
    });
  }, [batchQueueMode, getBatchQueueController, queuedBatchIds, queuedBatchIndex]);

  const handleResumeBatchQueue = useCallback(() => {
    void getBatchQueueController().resume(queueResumeState);
  }, [getBatchQueueController, queueResumeState]);

  const clearQueuedSelectionContext = useCallback(() => {
    setQueueResumeState(null);
    setSelectedRecentBatchSummaryIds([]);
    setQueueMessage("");
  }, [setQueueMessage, setQueueResumeState, setSelectedRecentBatchSummaryIds]);

  const handleAdvanceBatchQueue = useCallback(() => {
    if (!batchQueueMode) {
      return;
    }
    void getBatchQueueController().advance({
      batchIds: queuedBatchIds,
      currentIndex: queuedBatchIndex,
      mode: batchQueueMode,
    });
  }, [batchQueueMode, getBatchQueueController, queuedBatchIds, queuedBatchIndex]);

  return {
    clearQueuedSelectionContext,
    handleAdvanceBatchQueue,
    handleExitBatchQueue,
    handleOpenBatchQueue,
    handleResumeBatchQueue,
  };
}
