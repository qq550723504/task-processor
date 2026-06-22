import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSheinStudioDedicatedBatchRunController } from "@/components/listingkit/shein-studio/shein-studio-dedicated-batch-run-controller";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";

const hydratedBatch: SheinStudioWorkbenchHydratedBatch = {
  savedBatch: {
    id: "batch-1",
    name: "Batch 1",
    prompt: "prompt",
    styleCount: "1",
    sheinStoreId: "869",
    designs: [],
    selectedIds: [],
    createdTasks: [],
    updatedAt: "2026-06-22T00:00:00.000Z",
  },
  detail: {
    batch: {
      id: "batch-1",
      status: "draft",
      prompt: "prompt",
      styleCount: "1",
      sheinStoreId: 869,
      createdAt: "2026-06-22T00:00:00.000Z",
      updatedAt: "2026-06-22T00:00:00.000Z",
    },
    items: [],
  },
};

describe("useSheinStudioDedicatedBatchRunController", () => {
  const getHydratedBatch = vi.fn();
  const loadHydratedBatch = vi.fn();
  const refreshSavedBatches = vi.fn();
  const setActiveBatchRunId = vi.fn();
  const setBatchRunError = vi.fn();
  const startBatchRun = vi.fn();

  beforeEach(() => {
    getHydratedBatch.mockReset();
    loadHydratedBatch.mockReset();
    refreshSavedBatches.mockReset();
    setActiveBatchRunId.mockReset();
    setBatchRunError.mockReset();
    startBatchRun.mockReset();

    getHydratedBatch.mockResolvedValue(hydratedBatch);
    refreshSavedBatches.mockResolvedValue(undefined);
    startBatchRun.mockResolvedValue({
      run: { id: "run-1" },
      items: [],
    });
  });

  it("starts a dedicated generate run and exposes starting state", async () => {
    const { result } = renderHook(() =>
      useSheinStudioDedicatedBatchRunController({
        getBatchRunStartErrorMessage: (error) =>
          error instanceof Error ? error.message : "failed",
        getHydratedBatch,
        initialBatchId: "batch-1",
        loadHydratedBatch,
        refreshSavedBatches,
        setActiveBatchRunId,
        setBatchRunError,
        startBatchRun,
      }),
    );

    act(() => {
      result.current.handleStartDedicatedBatchRun();
    });

    expect(result.current.isStartingDedicatedBatchRun).toBe(true);
    await waitFor(() => expect(setActiveBatchRunId).toHaveBeenCalledWith("run-1"));

    expect(startBatchRun).toHaveBeenCalledWith(["batch-1"], "generate");
    expect(setBatchRunError).toHaveBeenCalledWith("");
    await waitFor(() =>
      expect(result.current.isStartingDedicatedBatchRun).toBe(false),
    );
  });

  it("returns from a dedicated run by clearing run state and hydrating the batch", async () => {
    const { result } = renderHook(() =>
      useSheinStudioDedicatedBatchRunController({
        getBatchRunStartErrorMessage: () => "failed",
        getHydratedBatch,
        initialBatchId: "batch-1",
        loadHydratedBatch,
        refreshSavedBatches,
        setActiveBatchRunId,
        setBatchRunError,
        startBatchRun,
      }),
    );

    act(() => {
      result.current.handleReturnFromDedicatedBatchRun();
    });

    await waitFor(() => expect(loadHydratedBatch).toHaveBeenCalledWith(hydratedBatch));

    expect(setActiveBatchRunId).toHaveBeenCalledWith("");
    expect(refreshSavedBatches).toHaveBeenCalled();
    expect(getHydratedBatch).toHaveBeenCalledWith("batch-1");
  });

  it("does not start a dedicated run without an initial batch id", () => {
    const { result } = renderHook(() =>
      useSheinStudioDedicatedBatchRunController({
        getBatchRunStartErrorMessage: () => "failed",
        getHydratedBatch,
        loadHydratedBatch,
        refreshSavedBatches,
        setActiveBatchRunId,
        setBatchRunError,
        startBatchRun,
      }),
    );

    act(() => {
      result.current.handleStartDedicatedBatchRun();
    });

    expect(startBatchRun).not.toHaveBeenCalled();
    expect(setBatchRunError).not.toHaveBeenCalled();
  });
});
