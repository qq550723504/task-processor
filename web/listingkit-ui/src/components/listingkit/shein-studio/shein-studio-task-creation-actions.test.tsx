import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSheinStudioTaskCreationAction } from "@/components/listingkit/shein-studio/shein-studio-task-creation-actions";

const createSheinStudioBatchTasks = vi.fn();

vi.mock("@/lib/api/shein-studio-batches", () => ({
  createSheinStudioBatchTasks: (...args: unknown[]) =>
    createSheinStudioBatchTasks(...args),
}));

const selection = {
  layerId: "layer-1",
  parentProductId: 1,
  printableHeight: 1000,
  printableWidth: 1000,
  productId: 1,
  productName: "tee",
  prototypeGroupId: 200,
  variantId: 100,
  variantLabel: "M / black",
};

describe("useSheinStudioTaskCreationAction", () => {
  beforeEach(() => {
    createSheinStudioBatchTasks.mockReset();
  });

  it("creates itemized batch tasks from server approved designs when local selection is stale", async () => {
    createSheinStudioBatchTasks.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "tasks_created",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:06:00.000Z",
      },
      items: [],
      createdTasks: [
        { id: "task-1", title: "Task 1", designId: "design-1" },
        { id: "task-2", title: "Task 2", designId: "design-2" },
      ],
    });

    const setCreatedTasks = vi.fn();
    const { result } = renderHook(() =>
      useSheinStudioTaskCreationAction({
        activeSelection: selection,
        designs: [
          { id: "design-1", imageUrl: "https://example.com/design-1.png" },
          { id: "design-2", imageUrl: "https://example.com/design-2.png" },
        ],
        groupedImageMode: "shared_by_size",
        imageStrategy: "sds_official",
        navigateToStep: vi.fn(),
        persistDraft: vi.fn().mockResolvedValue(undefined),
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        prompt: "retro cherries",
        renderSizeImagesWithSds: true,
        selectedIds: ["design-1"],
        selectedSdsImages: [],
        groupedSelections: [],
        activeSelectionBaselineStatus: "ready",
        activeSelectionBaselineReason: "",
        setCreatedTasks,
        setCreatingError: vi.fn(),
        setCreatingMessage: vi.fn(),
        setCreatingWarning: vi.fn(),
        setGalleryRatioCheck: vi.fn(),
        setIsCreatingTasks: vi.fn(),
        sheinStoreId: "869",
        hasLocalWorkflowStateRef: { current: false },
        itemizedBatchContext: {
          batchId: "batch-1",
          detail: {
            batch: {
              id: "batch-1",
              status: "review_ready",
              prompt: "retro cherries",
              styleCount: "1",
              sheinStoreId: 869,
              createdAt: "2026-05-26T09:59:00.000Z",
              updatedAt: "2026-05-26T10:00:00.000Z",
            },
            items: [
              {
                item: {
                  id: "item-1",
                  batchId: "batch-1",
                  targetGroupKey: "size:1000x1000",
                  status: "review_ready",
                  selectionCount: 1,
                  createdAt: "2026-05-26T09:59:00.000Z",
                  updatedAt: "2026-05-26T10:00:00.000Z",
                },
                designs: [
                  {
                    id: "design-1",
                    batchId: "batch-1",
                    itemId: "item-1",
                    sourceAttemptId: "attempt-1",
                    targetGroupKey: "size:1000x1000",
                    imageUrl: "https://example.com/design-1.png",
                    reviewStatus: "approved",
                    createdAt: "2026-05-26T09:59:30.000Z",
                    updatedAt: "2026-05-26T10:00:00.000Z",
                  },
                  {
                    id: "design-2",
                    batchId: "batch-1",
                    itemId: "item-1",
                    sourceAttemptId: "attempt-2",
                    targetGroupKey: "size:1000x1000",
                    imageUrl: "https://example.com/design-2.png",
                    reviewStatus: "approved",
                    createdAt: "2026-05-26T09:59:31.000Z",
                    updatedAt: "2026-05-26T10:00:00.000Z",
                  },
                ],
              },
            ],
          },
          onCreated: vi.fn(),
        },
      }),
    );

    await act(async () => {
      await result.current.handleCreateTasks();
    });

    expect(createSheinStudioBatchTasks).toHaveBeenCalledWith("batch-1", [
      "design-1",
      "design-2",
    ]);
    expect(setCreatedTasks).toHaveBeenCalledWith([
      { id: "task-1", title: "Task 1", designId: "design-1" },
      { id: "task-2", title: "Task 2", designId: "design-2" },
    ]);
  });
});
