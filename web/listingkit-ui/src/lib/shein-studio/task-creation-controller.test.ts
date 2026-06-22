import { describe, expect, it } from "vitest";

import {
  buildBatchTaskCreationFailureSummary,
  buildGroupedTaskCreationWarningSummary,
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
});
