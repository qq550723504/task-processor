import { describe, expect, it, vi } from "vitest";

import {
  buildBatchTaskCreationFailureSummary,
  buildGroupedTaskCreationWarningSummary,
  executeItemizedBatchTaskCreation,
  executeStandaloneTaskCreation,
  groupTaskCreationSelectionsByStore,
  resolveTaskCreationStartValidation,
} from "@/lib/shein-studio/task-creation-controller";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

const selection: SDSProductVariantSelection = {
  productId: 100,
  parentProductId: 100,
  variantId: 101,
  prototypeGroupId: 1,
  layerId: "layer-1",
  productName: "T-shirt",
  variantLabel: "L / white",
};

function groupedSelection(
  sheinStoreId: string,
  variantId: number,
): GroupedSDSSelectionEligibility {
  return {
    selection: { ...selection, variantId },
    selectionId: `selection-${variantId}`,
    sheinStoreId,
    baselineStatus: "ready",
    baselineReason: "",
    eligible: true,
  };
}

describe("SHEIN Studio task creation controller", () => {
  it("validates task creation start with existing messages", () => {
    expect(
      resolveTaskCreationStartValidation({
        activeSelection: undefined,
        sheinStoreId: "869",
        approvedCount: 1,
      }),
    ).toEqual({ error: "请先选择 SDS 变体。" });
    expect(
      resolveTaskCreationStartValidation({
        activeSelection: selection,
        sheinStoreId: " ",
        approvedCount: 1,
      }),
    ).toEqual({ error: "请先选择批次店铺。" });
    expect(
      resolveTaskCreationStartValidation({
        activeSelection: selection,
        sheinStoreId: "869",
        approvedCount: 0,
      }),
    ).toEqual({ error: "请至少批准 1 个款式后再创建 SHEIN 任务。" });
    expect(
      resolveTaskCreationStartValidation({
        activeSelection: selection,
        sheinStoreId: "869",
        approvedCount: 1,
      }),
    ).toBeNull();
  });

  it("summarizes batch task creation failures and rejections", () => {
    expect(
      buildBatchTaskCreationFailureSummary(
        [
          {
            designId: "design-1",
            title: "Failed design",
            message: "backend failed",
            reasonCode: "UPSTREAM",
          },
        ],
        [
          {
            designId: "design-2",
            title: "Rejected design",
            message: "not eligible",
            reasonCode: "INVALID",
          },
        ],
      ),
    ).toBe(
      "部分任务被拒绝或创建失败：Rejected design: INVALID · not eligible；Failed design: UPSTREAM · backend failed",
    );
  });

  it("summarizes grouped task creation warnings", () => {
    expect(
      buildGroupedTaskCreationWarningSummary([
        {
          label: "商品 A",
          selectionId: "a",
          reason: "missing_design_match",
          message: "",
        },
        {
          label: "商品 B",
          selectionId: "b",
          reason: "missing_design_match",
          message: "",
        },
      ]),
    ).toBe(
      "有 2 款商品因为没有匹配到自己的款式图而被跳过：商品 A、商品 B 共 2 款商品。这些商品不会创建错误任务，你可以回到生成区补图后再重试。",
    );
  });

  it("groups task creation selections by trimmed store id", () => {
    expect(
      groupTaskCreationSelectionsByStore([
        groupedSelection(" 869 ", 101),
        groupedSelection("870", 102),
        groupedSelection("869", 103),
      ]),
    ).toEqual([
      {
        sheinStoreId: "869",
        items: [groupedSelection(" 869 ", 101), groupedSelection("869", 103)],
      },
      {
        sheinStoreId: "870",
        items: [groupedSelection("870", 102)],
      },
    ]);
  });

  it("executes grouped standalone task creation and persists created tasks", async () => {
    const createdTask = {
      id: "task-1",
      designId: "design-1",
      title: "Task 1",
    };
    const createGroupedTasks = vi.fn().mockResolvedValue({
      created: [createdTask],
      warnings: [
        {
          label: "商品 B",
          selectionId: "selection-102",
          reason: "missing_design_match",
          message: "",
        },
      ],
    });
    const createTasks = vi.fn();
    const setCreatedTasks = vi.fn();
    const setCreatingMessage = vi.fn();
    const setCreatingWarning = vi.fn();
    const navigateToStep = vi.fn();
    const persistDraft = vi.fn().mockResolvedValue(undefined);
    const localWorkflowStateRef = { current: false };

    const result = await executeStandaloneTaskCreation({
      activeSelection: selection,
      activeSelectionBaselineReason: "",
      activeSelectionBaselineStatus: "ready",
      approvedDesigns: [{ id: "design-1" }],
      createGroupedTasks,
      createTasks,
      groupedImageMode: "shared_by_size",
      groupedSelections: [groupedSelection("870", 102)],
      hasLocalWorkflowStateRef: localWorkflowStateRef,
      imageStrategy: "ai_generated",
      navigateToStep,
      persistDraft,
      productImageCount: "2",
      productImagePrompt: "front and back",
      productImagePrompts: [],
      prompt: "summer flowers",
      renderSizeImagesWithSds: true,
      selectedSdsImages: [],
      setCreatedTasks,
      setCreatingMessage,
      setCreatingWarning,
      sheinStoreId: "869",
    });

    expect(result.availableTasks).toEqual([createdTask]);
    expect(createGroupedTasks).toHaveBeenCalledWith(
      expect.objectContaining({
        prompt: "summer flowers",
        groupedImageMode: "shared_by_size",
        groups: [
          expect.objectContaining({
            sheinStoreId: "869",
            approvedDesigns: [{ id: "design-1" }],
          }),
          expect.objectContaining({
            sheinStoreId: "870",
            approvedDesigns: [{ id: "design-1" }],
          }),
        ],
      }),
    );
    expect(createTasks).not.toHaveBeenCalled();
    expect(setCreatedTasks).toHaveBeenCalledWith([createdTask]);
    expect(setCreatingMessage).toHaveBeenCalledWith(
      "已为 1 个 SDS 商品生成或复用 SHEIN 资料任务。请在下方打开并审核。",
    );
    expect(setCreatingWarning).toHaveBeenCalledWith(
      "有 1 款商品因为没有匹配到自己的款式图而被跳过：商品 B。这些商品不会创建错误任务，你可以回到生成区补图后再重试。",
    );
    expect(navigateToStep).toHaveBeenCalledWith("tasks");
    expect(persistDraft).toHaveBeenCalledWith(
      { createdTasks: [createdTask] },
      { navigationTriggered: true, source: "task_creation_success" },
    );
    expect(localWorkflowStateRef.current).toBe(true);
  });

  it("executes itemized batch task creation with tenant and partial options", async () => {
    const resultPayload = {
      batch: {
        id: "batch-1",
        status: "tasks_creating",
      },
      createdTasks: [
        {
          id: "task-1",
          designId: "design-1",
          title: "Task 1",
        },
      ],
      reusedTasks: [
        {
          id: "task-2",
          designId: "design-2",
          title: "Task 2",
        },
      ],
      rejectedTasks: [
        {
          designId: "design-3",
          title: "Rejected design",
          message: "not eligible",
        },
      ],
      failedTasks: [
        {
          designId: "design-4",
          title: "Failed design",
          message: "backend failed",
        },
      ],
    };
    const createBatchTasks = vi.fn().mockResolvedValue(resultPayload);
    const onCreated = vi.fn();

    const result = await executeItemizedBatchTaskCreation({
      allowPartialWhileGenerating: true,
      approvedDesignIds: ["design-1", "design-2"],
      batchId: "batch-1",
      createBatchTasks,
      onCreated,
      tenantId: " tenant-1 ",
    });

    expect(createBatchTasks).toHaveBeenCalledWith(
      "batch-1",
      ["design-1", "design-2"],
      { tenantId: "tenant-1", allowPartialWhileGenerating: true },
    );
    expect(onCreated).toHaveBeenCalledWith(resultPayload);
    expect(result).toEqual({
      created: resultPayload.createdTasks,
      reused: resultPayload.reusedTasks,
      rejected: resultPayload.rejectedTasks,
      failed: resultPayload.failedTasks,
      keepCreatingState: true,
      rawResult: resultPayload,
    });
  });
});
