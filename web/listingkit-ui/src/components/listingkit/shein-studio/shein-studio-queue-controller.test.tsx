import { act, renderHook, waitFor } from "@testing-library/react";
import { createRef } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSheinStudioQueueController } from "@/components/listingkit/shein-studio/shein-studio-queue-controller";
import type { SheinStudioBatchQueueResumeState } from "@/lib/shein-studio/batch-queue";
import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";

function buildBatch(id: string): SheinStudioSavedBatch {
  return {
    id,
    name: id,
    prompt: "prompt",
    styleCount: "1",
    sheinStoreId: "869",
    designs: [],
    selectedIds: [],
    createdTasks: [],
    updatedAt: "2026-06-22T00:00:00.000Z",
  };
}

describe("useSheinStudioQueueController", () => {
  const startBatchRun = vi.fn();
  const setActiveBatchRunId = vi.fn();
  const setBatchQueueMode = vi.fn();
  const setBatchRunError = vi.fn();
  const setEffectiveStep = vi.fn();
  const setQueueMessage = vi.fn();
  const setQueueResumeState = vi.fn();
  const setQueuedBatchIds = vi.fn();
  const setQueuedBatchIndex = vi.fn();
  const setSelectedRecentBatchSummaryIds = vi.fn();
  const loadBatch = vi.fn();
  const loadHydratedBatch = vi.fn();
  const hydrateBatchSelection = vi.fn();

  function renderController(
    overrides: Partial<Parameters<typeof useSheinStudioQueueController>[0]> = {},
  ) {
    const requestVersionRef = { current: 0 };
    const recentOpenVersionRef = { current: 0 };
    return renderHook(() =>
      useSheinStudioQueueController({
        batchQueueMode: "generate",
        currentQueuedBatchId: "batch-2",
        effectiveStep: "generate",
        queuedBatchIds: ["batch-1", "batch-2"],
        queuedBatchIndex: 1,
        queueResumeState: {
          batchIds: ["batch-1", "batch-2"],
          mode: "generate",
          startIndex: 1,
          total: 2,
        },
        savedBatches: [buildBatch("batch-1"), buildBatch("batch-2")],
        selectedRecentBatchHydrations: {},
        getBatchRunStartErrorMessage: (error) =>
          error instanceof Error ? error.message : "run failed",
        hydrateBatchSelection,
        loadBatch,
        loadHydratedBatch,
        requestVersionRef,
        recentOpenVersionRef,
        setActiveBatchRunId,
        setBatchQueueMode,
        setBatchRunError,
        setEffectiveStep,
        setQueueMessage,
        setQueueResumeState,
        setQueuedBatchIds,
        setQueuedBatchIndex,
        setSelectedRecentBatchSummaryIds,
        promptInputRef: { current: null },
        startBatchRun,
        ...overrides,
      }),
    );
  }

  beforeEach(() => {
    startBatchRun.mockReset();
    setActiveBatchRunId.mockReset();
    setBatchQueueMode.mockReset();
    setBatchRunError.mockReset();
    setEffectiveStep.mockReset();
    setQueueMessage.mockReset();
    setQueueResumeState.mockReset();
    setQueuedBatchIds.mockReset();
    setQueuedBatchIndex.mockReset();
    setSelectedRecentBatchSummaryIds.mockReset();
    loadBatch.mockReset();
    loadHydratedBatch.mockReset();
    hydrateBatchSelection.mockReset();

    startBatchRun.mockResolvedValue({
      run: { id: "run-1" },
      items: [],
    });
    hydrateBatchSelection.mockResolvedValue({});
  });

  it("starts a backend batch run when opening a queue", async () => {
    const { result } = renderController();

    act(() => {
      result.current.handleOpenBatchQueue({
        batchIds: ["batch-1", "batch-2"],
        mode: "generate",
      });
    });

    await waitFor(() => expect(setActiveBatchRunId).toHaveBeenCalledWith("run-1"));

    expect(setBatchRunError).toHaveBeenCalledWith("");
    expect(startBatchRun).toHaveBeenCalledWith(["batch-1", "batch-2"], "generate");
    expect(setQueueResumeState).toHaveBeenCalledWith(null);
  });

  it("preserves resume state when exiting an active queue", () => {
    const { result } = renderController();

    act(() => {
      result.current.handleExitBatchQueue();
    });

    expect(setQueueResumeState).toHaveBeenCalledWith({
      batchIds: ["batch-1", "batch-2"],
      mode: "generate",
      startIndex: 1,
      total: 2,
    } satisfies SheinStudioBatchQueueResumeState);
    expect(setBatchQueueMode).toHaveBeenCalledWith(null);
    expect(setQueuedBatchIds).toHaveBeenCalledWith([]);
    expect(setQueuedBatchIndex).toHaveBeenCalledWith(0);
  });

  it("clears queued selection context without touching queue mode", () => {
    const { result } = renderController();

    act(() => {
      result.current.clearQueuedSelectionContext();
    });

    expect(setQueueResumeState).toHaveBeenCalledWith(null);
    expect(setSelectedRecentBatchSummaryIds).toHaveBeenCalledWith([]);
    expect(setQueueMessage).toHaveBeenCalledWith("");
    expect(setBatchQueueMode).not.toHaveBeenCalled();
  });

  it("scrolls and focuses the generator when a generate queue item becomes active", () => {
    vi.useFakeTimers();
    const generator = document.createElement("div");
    generator.id = "shein-studio-generator";
    generator.scrollIntoView = vi.fn();
    document.body.appendChild(generator);
    const input = document.createElement("textarea");
    input.focus = vi.fn();
    const promptInputRef = createRef<HTMLTextAreaElement>();
    promptInputRef.current = input;

    renderController({
      batchQueueMode: "generate",
      currentQueuedBatchId: "batch-1",
      effectiveStep: "generate",
      promptInputRef,
    });

    act(() => {
      vi.runOnlyPendingTimers();
    });

    expect(generator.scrollIntoView).toHaveBeenCalledWith({
      behavior: "smooth",
      block: "start",
    });
    expect(input.focus).toHaveBeenCalled();
    vi.useRealTimers();
  });
});
