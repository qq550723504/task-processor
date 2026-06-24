import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  useSheinStudioActiveBatchScope,
  useSheinStudioActiveGroupPromptHistory,
  useSheinStudioActiveGroupPrimarySelection,
  useSheinStudioActiveSelectionSummary,
  useSheinStudioBusyMessage,
  useSheinStudioCreateActionDisabledReason,
  useSheinStudioCurrentBatchSelection,
  useSheinStudioItemizedGenerationInFlight,
  useSheinStudioStoreSelection,
  useSheinStudioSubscriptionGate,
  useSheinStudioWorkbenchTraceContext,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import { buildInitialSheinStudioWorkbenchState } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import type { SubscriptionSummary } from "@/lib/api/subscription";
import type { SheinStudioBatchDetail } from "@/lib/types/shein-studio-batch";
import type {
  SheinStudioGroupedWorkspace,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

const selection = {
  productId: 1,
  parentProductId: 1,
  variantId: 100,
  prototypeGroupId: 200,
  layerId: "layer-1",
  productName: "tee",
  variantLabel: "M / black",
};

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

function buildGroup(
  overrides: Partial<SheinStudioGroupedWorkspace> = {},
): SheinStudioGroupedWorkspace {
  return {
    id: "group-1",
    name: "Group 1",
    primarySelection: selection,
    groupedSelections: [],
    sheinStoreId: "869",
    currentPrompt: "current prompt",
    promptHistory: [],
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

describe("useSheinStudioActiveGroupPromptHistory", () => {
  it("returns prompt history for the selected group only", () => {
    const promptHistory: SheinStudioGroupedWorkspace["promptHistory"] = [
      {
        prompt: "first prompt",
        groupedImageMode: "shared_by_size",
        createdAt: "2026-06-20T00:00:00.000Z",
      },
    ];
    const { result } = renderHook(() =>
      useSheinStudioActiveGroupPromptHistory({
        activeGroupId: "group-2",
        groups: [
          buildGroup({ id: "group-1" }),
          buildGroup({
            id: "group-2",
            promptHistory,
          }),
        ],
      }),
    );

    expect(result.current).toBe(promptHistory);
  });
});

describe("useSheinStudioActiveGroupPrimarySelection", () => {
  it("returns the primary selection for the active group", () => {
    const activeSelection = {
      ...selection,
      variantId: 202,
      productName: "hoodie",
    };
    const { result } = renderHook(() =>
      useSheinStudioActiveGroupPrimarySelection({
        activeGroupId: "group-2",
        groups: [
          buildGroup({ id: "group-1" }),
          buildGroup({
            id: "group-2",
            primarySelection: activeSelection,
          }),
        ],
      }),
    );

    expect(result.current).toBe(activeSelection);
  });
});

describe("useSheinStudioActiveSelectionSummary", () => {
  it("derives the selection key and display summary", () => {
    const activeSelection = {
      ...selection,
      printableWidth: 120,
      printableHeight: 160,
      variants: [
        { variantId: 100, size: "M", color: "black" },
        { variantId: 101, size: "L", color: "black" },
      ],
    };

    const { result } = renderHook(() =>
      useSheinStudioActiveSelectionSummary(activeSelection),
    );

    expect(result.current.activeSelectionKey).toContain('"variantId":100');
    expect(result.current.printableAreaLabel).toBe("120 × 160px");
    expect(result.current.selectedColorCount).toBe(1);
    expect(result.current.selectedSizeCount).toBe(2);
    expect(result.current.selectedVariants).toBe(activeSelection.variants);
  });
});

describe("useSheinStudioSubscriptionGate", () => {
  it("blocks Studio access when the subscription denies the studio module", () => {
    const subscription: SubscriptionSummary = {
      tenant_id: "tenant-1",
      modules: [],
      entitlements: [
        {
          module: {
            code: "studio",
            name: "Studio",
            sort_order: 1,
            active: true,
          },
          usage: [],
          allowed: false,
        },
      ],
    };

    const { result } = renderHook(() =>
      useSheinStudioSubscriptionGate(subscription),
    );

    expect(result.current.studioAccessAllowed).toBe(false);
    expect(result.current.subscriptionBlockedMessage).toContain("Studio");
  });
});

describe("useSheinStudioCreateActionDisabledReason", () => {
  it("returns a blocking gallery ratio message before selected style checks", () => {
    const { result } = renderHook(() =>
      useSheinStudioCreateActionDisabledReason({
        galleryRatioCheck: {
          status: "blocking",
          message: "图片比例不符合 SDS 模板",
        },
        selectedIds: [],
        selection,
      }),
    );

    expect(result.current).toBe("图片比例不符合 SDS 模板");
  });
});

describe("useSheinStudioBusyMessage", () => {
  it("returns the generation busy message while generation is in flight", () => {
    const { result } = renderHook(() =>
      useSheinStudioBusyMessage({
        isCreatingTasks: false,
        isGenerating: true,
        regeneratingId: "design-1",
      }),
    );

    expect(result.current).toBe("正在生成款式图");
  });
});

describe("useSheinStudioItemizedGenerationInFlight", () => {
  it("detects a generating itemized batch", () => {
    const detail: SheinStudioBatchDetail = {
      batch: {
        id: "batch-1",
        status: "generating",
        prompt: "prompt",
        styleCount: "1",
        sheinStoreId: 42,
        createdAt: "2026-06-20T00:00:00.000Z",
        updatedAt: "2026-06-20T00:00:00.000Z",
      },
      items: [],
    };

    const { result } = renderHook(() =>
      useSheinStudioItemizedGenerationInFlight(detail),
    );

    expect(result.current).toBe(true);
  });
});
