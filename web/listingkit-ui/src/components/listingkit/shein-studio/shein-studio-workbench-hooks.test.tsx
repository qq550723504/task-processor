import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  useSheinStudioActiveBatchScope,
  useSheinStudioWorkbenchTraceContext,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";

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
