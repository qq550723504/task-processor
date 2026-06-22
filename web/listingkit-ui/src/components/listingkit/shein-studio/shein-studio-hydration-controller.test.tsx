import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSheinStudioInitialBatchHydration } from "@/components/listingkit/shein-studio/shein-studio-hydration-controller";

const hydratedBatch = {
  savedBatch: {
    id: "batch-1",
    name: "Batch 1",
    prompt: "remote prompt",
    updatedAt: "2026-06-21T10:00:00.000Z",
  },
  detail: {
    batch: {
      id: "batch-1",
      updatedAt: "2026-06-21T10:00:00.000Z",
    },
    items: [],
  },
};

describe("useSheinStudioInitialBatchHydration", () => {
  const getHydratedBatch = vi.fn();
  const loadLocalSnapshot = vi.fn();
  const resolveHydration = vi.fn();
  const loadHydratedBatch = vi.fn();
  const setGenerationError = vi.fn();
  const setQueueMessage = vi.fn();
  const setLoaded = vi.fn();

  beforeEach(() => {
    getHydratedBatch.mockReset();
    loadLocalSnapshot.mockReset();
    resolveHydration.mockReset();
    loadHydratedBatch.mockReset();
    setGenerationError.mockReset();
    setQueueMessage.mockReset();
    setLoaded.mockReset();

    getHydratedBatch.mockResolvedValue(hydratedBatch);
    loadLocalSnapshot.mockReturnValue({
      batchId: "batch-1",
      draft: {
        prompt: "local prompt",
        updatedAt: "2026-06-21T11:00:00.000Z",
      },
    });
    resolveHydration.mockReturnValue({
      ...hydratedBatch,
      savedBatch: {
        ...hydratedBatch.savedBatch,
        prompt: "local prompt",
      },
    });
  });

  it("loads a dedicated batch once and applies local snapshot resolution", async () => {
    const { rerender } = renderHook(
      ({ batchId }) =>
        useSheinStudioInitialBatchHydration({
          initialBatchId: batchId,
          getHydratedBatch,
          loadLocalSnapshot,
          loadHydratedBatch,
          promptOverride: "override prompt",
          resolveHydration,
          setGenerationError,
          setLoaded,
          setQueueMessage,
        }),
      {
        initialProps: {
          batchId: "batch-1",
        },
      },
    );

    await waitFor(() => expect(loadHydratedBatch).toHaveBeenCalledTimes(1));

    expect(getHydratedBatch).toHaveBeenCalledWith("batch-1");
    expect(resolveHydration).toHaveBeenCalledWith({
      batchId: "batch-1",
      hydratedBatch,
      localSnapshot: expect.objectContaining({ batchId: "batch-1" }),
      promptOverride: "override prompt",
    });
    expect(loadHydratedBatch).toHaveBeenCalledWith(
      expect.objectContaining({
        savedBatch: expect.objectContaining({ prompt: "local prompt" }),
      }),
    );
    expect(setLoaded).toHaveBeenCalledWith(true);

    rerender({ batchId: "batch-1" });
    expect(getHydratedBatch).toHaveBeenCalledTimes(1);
  });

  it("reports load failures through generation error and queue message", async () => {
    getHydratedBatch.mockRejectedValue(new Error("expired session"));

    renderHook(() =>
      useSheinStudioInitialBatchHydration({
        initialBatchId: "batch-1",
        getHydratedBatch,
        loadLocalSnapshot,
        loadHydratedBatch,
        resolveHydration,
        setGenerationError,
        setLoaded,
        setQueueMessage,
      }),
    );

    await waitFor(() =>
      expect(setGenerationError).toHaveBeenCalledWith(
        "当前批次加载失败：expired session。请重新登录后再继续。",
      ),
    );

    expect(setQueueMessage).toHaveBeenCalledWith(
      "当前批次加载失败：expired session。请重新登录后再继续。",
    );
    expect(setLoaded).toHaveBeenCalledWith(true);
    expect(loadHydratedBatch).not.toHaveBeenCalled();
  });
});
