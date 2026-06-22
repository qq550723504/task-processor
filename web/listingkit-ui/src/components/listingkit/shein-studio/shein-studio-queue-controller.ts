import { useCallback } from "react";

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

export function useSheinStudioQueueController({
  batchQueueMode,
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
