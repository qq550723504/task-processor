import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSheinStudioBatchGenerationContext } from "@/components/listingkit/shein-studio/shein-studio-generation-controller";
import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

function buildSavedBatch(id = "batch-1"): SheinStudioSavedBatch {
  return {
    id,
    name: "Batch 1",
    prompt: "prompt",
    styleCount: "1",
    sheinStoreId: "869",
    designs: [],
    selectedIds: [],
    createdTasks: [],
    updatedAt: "2026-06-22T00:00:00.000Z",
  };
}

describe("useSheinStudioBatchGenerationContext", () => {
  const buildDraftInput = vi.fn();
  const getHydratedBatch = vi.fn();
  const saveBatch = vi.fn();
  const setActiveBatchId = vi.fn();
  const setActiveSavedBatchId = vi.fn();
  const setActiveBatchRunId = vi.fn();
  const setBatchRunError = vi.fn();
  const setSavedBatches = vi.fn();
  const startBatchRun = vi.fn();

  beforeEach(() => {
    buildDraftInput.mockReset();
    getHydratedBatch.mockReset();
    saveBatch.mockReset();
    setActiveBatchId.mockReset();
    setActiveSavedBatchId.mockReset();
    setActiveBatchRunId.mockReset();
    setBatchRunError.mockReset();
    setSavedBatches.mockReset();
    startBatchRun.mockReset();

    buildDraftInput.mockReturnValue({
      prompt: "prompt",
      styleCount: "1",
      updatedAt: "2026-06-22T01:00:00.000Z",
    } satisfies Partial<SheinStudioSaveInput>);
    getHydratedBatch.mockResolvedValue({
      savedBatch: {
        ...buildSavedBatch("batch-existing"),
        draftUpdatedAt: "2026-06-22T02:00:00.000Z",
      },
      detail: {
        batch: {
          id: "batch-existing",
          draftUpdatedAt: "2026-06-22T03:00:00.000Z",
        },
        items: [],
      },
    });
    saveBatch.mockResolvedValue(buildSavedBatch("batch-saved"));
    startBatchRun.mockResolvedValue({
      run: { id: "run-1" },
      items: [],
    });
  });

  it("does not provide batch generation context without an active selection", () => {
    const { result } = renderHook(() =>
      useSheinStudioBatchGenerationContext({
        activeBatchId: "",
        buildDraftInput,
        currentGenerationJobId: "",
        enabled: false,
        generationError: "",
        getHydratedBatch,
        initialBatchId: "",
        saveBatch,
        setActiveBatchId,
        setActiveBatchRunId,
        setActiveSavedBatchId,
        setBatchRunError,
        setSavedBatches,
        startBatchRun,
      }),
    );

    expect(result.current.batchGenerationContext).toBeUndefined();
  });

  it("saves and activates the current batch before starting generation", async () => {
    const { result } = renderHook(() =>
      useSheinStudioBatchGenerationContext({
        activeBatchId: "",
        buildDraftInput,
        currentGenerationJobId: "job-1",
        enabled: true,
        generationError: "previous warning",
        getHydratedBatch,
        initialBatchId: "batch-existing",
        saveBatch,
        setActiveBatchId,
        setActiveBatchRunId,
        setActiveSavedBatchId,
        setBatchRunError,
        setSavedBatches,
        startBatchRun,
      }),
    );

    const saved = await result.current.batchGenerationContext?.ensureBatch();

    expect(getHydratedBatch).toHaveBeenCalledWith("batch-existing");
    expect(buildDraftInput).toHaveBeenCalledWith({
      createdTasks: [],
      designs: [],
      generationError: "previous warning",
      generationJobId: "job-1",
      generationJobs: [],
      selectedIds: [],
    });
    expect(saveBatch).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "batch-existing",
        updatedAt: "2026-06-22T03:00:00.000Z",
      }),
      { makeActive: false },
    );
    expect(saved).toEqual(expect.objectContaining({ id: "batch-saved" }));
    expect(setActiveBatchId).toHaveBeenCalledWith("batch-saved");
    expect(setActiveSavedBatchId).toHaveBeenCalledWith("batch-saved");
    expect(setSavedBatches).toHaveBeenCalled();
  });

  it("starts a backend generation run for the saved batch", async () => {
    const { result } = renderHook(() =>
      useSheinStudioBatchGenerationContext({
        activeBatchId: "",
        buildDraftInput,
        currentGenerationJobId: "",
        enabled: true,
        generationError: "",
        getHydratedBatch,
        initialBatchId: "",
        saveBatch,
        setActiveBatchId,
        setActiveBatchRunId,
        setActiveSavedBatchId,
        setBatchRunError,
        setSavedBatches,
        startBatchRun,
      }),
    );

    await result.current.batchGenerationContext?.startGenerationRun(
      buildSavedBatch("batch-saved"),
    );

    await waitFor(() => expect(setActiveBatchRunId).toHaveBeenCalledWith("run-1"));
    expect(setBatchRunError).toHaveBeenCalledWith("");
    expect(startBatchRun).toHaveBeenCalledWith(["batch-saved"], "generate");
  });
});
