import { useCallback, useEffect, useRef, useState } from "react";

import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";

type DedicatedBatchRunStarter = (
  batchIds: string[],
  mode: "generate",
) => Promise<{ run: { id: string } }>;

type DedicatedBatchRunControllerParams = {
  getBatchRunStartErrorMessage: (error: unknown) => string;
  getHydratedBatch: (
    batchId: string,
  ) => Promise<SheinStudioWorkbenchHydratedBatch | null>;
  initialBatchId?: string;
  loadHydratedBatch: (batch: SheinStudioWorkbenchHydratedBatch) => void;
  refreshSavedBatches: () => void | Promise<unknown>;
  setActiveBatchRunId: (runId: string) => void;
  setBatchRunError: (message: string) => void;
  startBatchRun: DedicatedBatchRunStarter;
};

export function useSheinStudioDedicatedBatchRunController({
  getBatchRunStartErrorMessage,
  getHydratedBatch,
  initialBatchId,
  loadHydratedBatch,
  refreshSavedBatches,
  setActiveBatchRunId,
  setBatchRunError,
  startBatchRun,
}: DedicatedBatchRunControllerParams) {
  const [isStartingDedicatedBatchRun, setIsStartingDedicatedBatchRun] =
    useState(false);
  const loadHydratedBatchRef = useRef(loadHydratedBatch);

  useEffect(() => {
    loadHydratedBatchRef.current = loadHydratedBatch;
  }, [loadHydratedBatch]);

  const handleReturnFromDedicatedBatchRun = useCallback(() => {
    setActiveBatchRunId("");
    void refreshSavedBatches();
    if (!initialBatchId) {
      return;
    }
    void getHydratedBatch(initialBatchId).then((hydratedBatch) => {
      if (!hydratedBatch) {
        return;
      }
      loadHydratedBatchRef.current(hydratedBatch);
    });
  }, [getHydratedBatch, initialBatchId, refreshSavedBatches, setActiveBatchRunId]);

  const handleStartDedicatedBatchRun = useCallback(() => {
    if (!initialBatchId) {
      return;
    }
    setBatchRunError("");
    setIsStartingDedicatedBatchRun(true);
    void startBatchRun([initialBatchId], "generate")
      .then((response) => {
        setActiveBatchRunId(response.run.id);
      })
      .catch((error) => {
        setBatchRunError(getBatchRunStartErrorMessage(error));
      })
      .finally(() => {
        setIsStartingDedicatedBatchRun(false);
      });
  }, [
    getBatchRunStartErrorMessage,
    initialBatchId,
    setActiveBatchRunId,
    setBatchRunError,
    startBatchRun,
  ]);

  return {
    handleReturnFromDedicatedBatchRun,
    handleStartDedicatedBatchRun,
    isStartingDedicatedBatchRun,
  };
}
