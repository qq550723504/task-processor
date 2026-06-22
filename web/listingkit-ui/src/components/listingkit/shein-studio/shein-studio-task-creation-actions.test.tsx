import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSheinStudioTaskCreationAction } from "@/components/listingkit/shein-studio/shein-studio-task-creation-actions";
import type {
  SheinStudioBatchDetail,
  SheinStudioBatchStatus,
} from "@/lib/types/shein-studio";

type TaskCreationActionParams = Parameters<
  typeof useSheinStudioTaskCreationAction
>[0];

const createSheinStudioBatchTasks = vi.fn();
const createGroupedSheinReviewTasks = vi.fn();
const createSheinReviewTasks = vi.fn();

vi.mock("@/lib/api/shein-studio-batches", () => ({
  createSheinStudioBatchTasks: (...args: unknown[]) =>
    createSheinStudioBatchTasks(...args),
}));

vi.mock("@/lib/shein-studio/create-review-tasks", () => ({
  createGroupedSheinReviewTasks: (...args: unknown[]) =>
    createGroupedSheinReviewTasks(...args),
  createSheinReviewTasks: (...args: unknown[]) => createSheinReviewTasks(...args),
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

function buildInFlightBatchDetail(
  status: SheinStudioBatchStatus = "generating",
): SheinStudioBatchDetail {
  return {
    batch: {
      id: "batch-1",
      status,
      prompt: "retro cherries",
      styleCount: "2",
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
        ],
      },
      {
        item: {
          id: "item-2",
          batchId: "batch-1",
          targetGroupKey: "size:1200x1200",
          status: "generating",
          selectionCount: 1,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        designs: [],
      },
    ],
  };
}

function buildReviewReadyBatchDetail(): SheinStudioBatchDetail {
  return {
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
          targetGroupKey: "compat:fp-1",
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
            targetGroupKey: "compat:fp-1",
            imageUrl: "https://example.com/design-1.png",
            reviewStatus: "approved",
            createdAt: "2026-05-26T09:59:30.000Z",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
        ],
      },
    ],
  };
}

function renderTaskCreationAction(overrides: Partial<TaskCreationActionParams> = {}) {
  const params: TaskCreationActionParams = {
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
    setCreatedTasks: vi.fn(),
    setCreatingError: vi.fn(),
    setCreatingMessage: vi.fn(),
    setCreatingWarning: vi.fn(),
    setGalleryRatioCheck: vi.fn(),
    setIsCreatingTasks: vi.fn(),
    sheinStoreId: "869",
    hasLocalWorkflowStateRef: { current: false },
    itemizedBatchContext: {
      batchId: "batch-1",
      detail: buildInFlightBatchDetail("review_ready"),
      onCreated: vi.fn(),
    },
    ...overrides,
  };

  return {
    params,
    ...renderHook(() => useSheinStudioTaskCreationAction(params)),
  };
}

describe("useSheinStudioTaskCreationAction", () => {
  beforeEach(() => {
    createSheinStudioBatchTasks.mockReset();
    createGroupedSheinReviewTasks.mockReset();
    createSheinReviewTasks.mockReset();
    vi.restoreAllMocks();
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

    const navigateToStep = vi.fn();
    const onCreated = vi.fn();
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
        navigateToStep,
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
          onCreated,
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
    expect(onCreated).toHaveBeenCalledWith(
      expect.objectContaining({
        createdTasks: [
          { id: "task-1", title: "Task 1", designId: "design-1" },
          { id: "task-2", title: "Task 2", designId: "design-2" },
        ],
      }),
    );
    expect(navigateToStep).toHaveBeenCalledWith("tasks");
  });

  it("confirms before creating tasks while the itemized batch is still generating", async () => {
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    const { result } = renderTaskCreationAction({
      itemizedBatchContext: {
        batchId: "batch-1",
        detail: buildInFlightBatchDetail(),
        onCreated: vi.fn(),
      },
    });

    await act(async () => {
      await result.current.handleCreateTasks();
    });

    expect(confirmSpy).toHaveBeenCalledWith(
      "当前批次仍有图片正在生成。本次只会为当前已批准的 1 个款式创建 SHEIN 资料，剩余图片生成完成并批准后需要再次创建。是否继续？",
    );
    expect(createSheinStudioBatchTasks).not.toHaveBeenCalled();
  });

  it("passes partial creation allowance after confirming task creation during generation", async () => {
    const inFlightDetail = buildInFlightBatchDetail();
    createSheinStudioBatchTasks.mockResolvedValue({
      ...inFlightDetail,
      batch: {
        ...inFlightDetail.batch,
        status: "tasks_creating",
      },
      createdTasks: [],
    });
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const { result } = renderTaskCreationAction({
      itemizedBatchContext: {
        batchId: "batch-1",
        detail: inFlightDetail,
        onCreated: vi.fn(),
      },
    });

    await act(async () => {
      await result.current.handleCreateTasks();
    });

    expect(createSheinStudioBatchTasks).toHaveBeenCalledWith(
      "batch-1",
      ["design-1"],
      { allowPartialWhileGenerating: true },
    );
  });

  it("treats rejected-only batch task creation as a failure instead of navigating to tasks", async () => {
    const reviewReadyDetail = buildReviewReadyBatchDetail();
    createSheinStudioBatchTasks.mockResolvedValue({
      ...reviewReadyDetail,
      batch: {
        ...reviewReadyDetail.batch,
        status: "tasks_created",
      },
      createdTasks: [],
      reusedTasks: [],
      rejectedTasks: [
        {
          designId: "design-1",
          itemId: "item-1",
          selectionId: "selection-1",
          compatibilityFingerprint: "fp-1",
          reasonCode: "baseline_not_ready",
          message: "baseline 还没准备好",
          outcome: "rejected",
        },
      ],
      failedTasks: [],
    });
    const navigateToStep = vi.fn();
    const setCreatingError = vi.fn();
    const setCreatingMessage = vi.fn();
    const { result } = renderTaskCreationAction({
      navigateToStep,
      setCreatingError,
      setCreatingMessage,
      itemizedBatchContext: {
        batchId: "batch-1",
        detail: reviewReadyDetail,
        onCreated: vi.fn(),
      },
    });

    await act(async () => {
      await result.current.handleCreateTasks();
    });

    expect(setCreatingError).toHaveBeenCalledWith(
      expect.stringContaining("baseline 还没准备好"),
    );
    expect(setCreatingError).toHaveBeenCalledWith(
      expect.stringContaining("部分任务被拒绝"),
    );
    expect(setCreatingMessage).toHaveBeenLastCalledWith("");
    expect(navigateToStep).not.toHaveBeenCalledWith("tasks");
  });

  it("blocks standalone task creation when an imported gallery image ratio mismatches the SDS ratio", async () => {
    const navigateToStep = vi.fn();
    const setCreatingError = vi.fn();
    const setGalleryRatioCheck = vi.fn();
    const { result } = renderTaskCreationAction({
      designs: [
        {
          id: "gallery-style-1",
          imageUrl: "https://example.com/gallery-style.png",
          role: "gallery",
          sourceHeight: 1000,
          sourceWidth: 1400,
        },
      ],
      itemizedBatchContext: undefined,
      navigateToStep,
      selectedIds: ["gallery-style-1"],
      setCreatingError,
      setGalleryRatioCheck,
    });

    await act(async () => {
      await result.current.handleCreateTasks();
    });

    expect(setGalleryRatioCheck).toHaveBeenCalledWith(
      expect.objectContaining({
        status: "blocking",
      }),
    );
    expect(setCreatingError).toHaveBeenCalledWith(
      "图库图片比例与 SDS 款式比例差异过大，请换图或更换 SDS 款式。",
    );
    expect(createSheinReviewTasks).not.toHaveBeenCalled();
    expect(createGroupedSheinReviewTasks).not.toHaveBeenCalled();
    expect(navigateToStep).not.toHaveBeenCalled();
  });
});
