import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  useSheinStudioActiveBatchScope,
  useSheinStudioCurrentBatchSelection,
  useSheinStudioStoreSelection,
  useSheinStudioWorkbenchTraceContext,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import { buildInitialSheinStudioWorkbenchState } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";

function buildSavedBatch(
  overrides: Partial<SheinStudioSavedBatch> = {},
): SheinStudioSavedBatch {
  return {
    id: "batch-1",
    name: "Saved batch",
    prompt: "saved prompt",
    styleCount: "1",
    sheinStoreId: "869",
    designs: [],
    selectedIds: [],
    createdTasks: [],
    updatedAt: "2026-06-20T00:00:00.000Z",
    ...overrides,
  };
}

describe("useSheinStudioActiveBatchScope", () => {
  it("keeps the dedicated route batch id as the active batch", () => {
    const { result } = renderHook(() =>
      useSheinStudioActiveBatchScope({
        initialBatchId: "batch-route",
        selectionVariantId: 101,
      }),
    );

    act(() => {
      result.current.setActiveBatchId("batch-local");
    });

    expect(result.current.activeBatchId).toBe("batch-route");
  });

  it("keeps a local active batch only while the selection variant matches", () => {
    const { result, rerender } = renderHook(
      ({ selectionVariantId }: { selectionVariantId: number | null }) =>
        useSheinStudioActiveBatchScope({
          selectionVariantId,
        }),
      {
        initialProps: { selectionVariantId: 101 },
      },
    );

    act(() => {
      result.current.setActiveBatchId("batch-101");
    });

    expect(result.current.activeBatchId).toBe("batch-101");

    rerender({ selectionVariantId: 202 });

    expect(result.current.activeBatchId).toBe("");
  });
});

describe("useSheinStudioWorkbenchTraceContext", () => {
  it("projects trace context with queued batch taking priority over active and route batches", () => {
    const { result, rerender } = renderHook(
      ({
        currentQueuedBatchId,
      }: {
        currentQueuedBatchId: string;
      }) =>
        useSheinStudioWorkbenchTraceContext({
          activeBatchId: "batch-active",
          batchQueueMode: "generate",
          currentQueuedBatchId,
          initialBatchId: "batch-route",
          queuedBatchIds: ["batch-queued-1", "batch-queued-2"],
          queuedBatchIndex: 1,
        }),
      {
        initialProps: {
          currentQueuedBatchId: "batch-queued-2",
        },
      },
    );

    expect(result.current).toEqual({
      batchId: "batch-queued-2",
      queueIndex: 2,
      queueMode: "generate",
      queueTotal: 2,
    });

    rerender({ currentQueuedBatchId: "" });

    expect(result.current.batchId).toBe("batch-active");
  });
});

describe("useSheinStudioCurrentBatchSelection", () => {
  it("selects the current saved batch and dedicated route batch", () => {
    const savedBatch = buildSavedBatch({ id: "batch-route" });
    const { result } = renderHook(() =>
      useSheinStudioCurrentBatchSelection({
        activeBatchId: "",
        initialBatchId: "batch-route",
        savedBatches: [savedBatch],
        workbenchState: buildInitialSheinStudioWorkbenchState(),
      }),
    );

    expect(result.current.currentActiveBatch).toBe(savedBatch);
    expect(result.current.currentDedicatedBatch).toBe(savedBatch);
  });

  it("falls back to workbench state when the active batch has not been saved", () => {
    const workbenchState = {
      ...buildInitialSheinStudioWorkbenchState(),
      prompt: "draft prompt",
      styleCount: "3",
    };
    const { result } = renderHook(() =>
      useSheinStudioCurrentBatchSelection({
        activeBatchId: "batch-local",
        initialBatchId: undefined,
        savedBatches: [],
        workbenchState,
      }),
    );

    expect(result.current.currentActiveBatch).toMatchObject({
      id: "batch-local",
      prompt: "draft prompt",
      styleCount: "3",
    });
    expect(result.current.currentDedicatedBatch).toBeNull();
  });
});

describe("useSheinStudioStoreSelection", () => {
  it("projects current store labels and recent batch store options", () => {
    const { result } = renderHook(() =>
      useSheinStudioStoreSelection({
        currentStoreId: " 42 ",
        enabledProfiles: [
          {
            name: "Main",
            store_id: 42,
            storeId: "42",
            site: "US",
          },
          {
            name: "Backup",
            store_id: 99,
            storeId: "99",
            site: "UK",
          },
        ],
      }),
    );

    expect(result.current).toEqual({
      currentStoreLabel: "Main (42 / US)",
      effectiveCurrentStoreId: "42",
      recentBatchStoreOptions: [
        { id: "42", label: "Main (42 / US)" },
        { id: "99", label: "Backup (99 / UK)" },
      ],
      storeRequiredMessage: "",
    });
  });
});
