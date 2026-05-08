import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { SheinSaleAttributeReviewCard } from "@/components/listingkit/shein/shein-sale-attribute-review-card";

describe("SheinSaleAttributeReviewCard", () => {
  it("renders current sale attribute summary and candidate reasons", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 27,
              selection_summary: ["主销售属性已命中，次销售属性仍需确认"],
              skc_attributes: [
                {
                  scope: "skc",
                  name: "Color",
                  value: "Black",
                  attribute_id: 27,
                  attribute_value_id: 112,
                },
              ],
              sku_attributes: [
                {
                  scope: "sku",
                  name: "Size",
                  value: "One Size",
                  attribute_id: 87,
                  attribute_value_id: 991,
                },
              ],
              candidates: [
                {
                  source_dimension: "颜色",
                  name: "Color",
                  attribute_id: 27,
                  selected_scope: "primary",
                  reasons: ["模板值拟合度 1/2"],
                },
                {
                  source_dimension: "尺码",
                  name: "Size",
                  attribute_id: 87,
                  selected_scope: "secondary",
                  reasons: ["SKU 维度覆盖 1/1"],
                },
              ],
              review_notes: ["尺码模板候选仍待确认"],
            },
          },
        }}
      />,
    );

    expect(screen.getByText("SHEIN 销售属性确认")).toBeInTheDocument();
    expect(screen.getByText("状态 待补齐")).toBeInTheDocument();
    expect(screen.getByText("主规格 27")).toBeInTheDocument();
    expect(screen.getByText("主规格确认")).toBeInTheDocument();
    expect(screen.getByText("其他规格确认")).toBeInTheDocument();
    expect(screen.getByText("变体覆盖检查")).toBeInTheDocument();
    expect(screen.getByText("已映射销售属性")).toBeInTheDocument();
    expect(screen.getAllByText("Color").length).toBeGreaterThanOrEqual(3);
    expect(screen.getAllByText("Black").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("attribute_id 27 · value_id 112").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Size").length).toBeGreaterThanOrEqual(3);
    expect(screen.getAllByText("One Size").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("attribute_id 87 · value_id 991").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("主销售属性已命中，次销售属性仍需确认")).toBeInTheDocument();
    expect(screen.getAllByText("模板值拟合度 1/2").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("SKU 维度覆盖 1/1").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("尺码模板候选仍待确认")).toBeInTheDocument();
  });

  it("allows confirming partial current sale attributes", async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 1001184,
              secondary_attribute_id: 87,
              skc_attributes: [
                {
                  scope: "skc",
                  name: "Style Type",
                  value: "White",
                  attribute_id: 1001184,
                },
              ],
              sku_attributes: [
                {
                  scope: "sku",
                  name: "Size",
                  value: "90x180cm",
                  attribute_id: 87,
                  attribute_value_id: 417,
                },
              ],
            },
          },
        }}
        onConfirmCurrentSaleAttributes={onConfirm}
      />,
    );

    await user.click(screen.getByRole("button", { name: "确认当前规格" }));

    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});
