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
                  attribute_value_id: 739,
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

  it("offers regeneration instead of confirm when value ids are missing", async () => {
    const onRegenerate = vi.fn();
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 1001466,
              skc_attributes: [
                {
                  scope: "skc",
                  name: "Plug(Voltage)",
                  value: "white",
                  attribute_id: 1001466,
                },
              ],
            },
          },
        }}
        onConfirmCurrentSaleAttributes={vi.fn()}
        onRegenerateSaleAttributes={onRegenerate}
      />,
    );

    expect(screen.queryByRole("button", { name: "确认当前规格" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "按当前类目重新生成属性" })).toBeInTheDocument();
    expect(screen.getByText(/缺少真实 `value_id`/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "按当前类目重新生成属性" }));

    expect(onRegenerate).toHaveBeenCalledTimes(1);
  });

  it("allows manually filling sale attribute value ids", async () => {
    const onApplyManual = vi.fn();
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 1001466,
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              skc_attributes: [
                {
                  scope: "skc",
                  name: "Plug(Voltage)",
                  value: "white",
                  attribute_id: 1001466,
                },
              ],
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  skc_scope: true,
                  attribute_value_list: [
                    { attribute_value_id: 112, value: "White", value_en: "White" },
                  ],
                },
                {
                  attribute_id: 87,
                  name: "Size",
                  name_en: "Size",
                  attribute_value_list: [
                    { attribute_value_id: 991, value: "M", value_en: "M" },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  skc_name: "White",
                  attributes: { Color: "White" },
                  sku_patches: [
                    {
                      supplier_sku: "SKU-1",
                      attributes: { Size: "M" },
                    },
                  ],
                },
              ],
            },
          },
        }}
        onApplyManualSaleAttributes={onApplyManual}
      />,
    );

    expect(screen.getByText("手工填写销售属性")).toBeInTheDocument();

    await user.selectOptions(screen.getAllByRole("combobox")[0], "27");
    await user.selectOptions(screen.getAllByRole("combobox")[1], "87");
    await user.selectOptions(screen.getAllByRole("combobox")[2], "112");
    await user.selectOptions(screen.getAllByRole("combobox")[3], "991");
    await user.click(screen.getByRole("button", { name: "保存手工销售属性" }));

    expect(onApplyManual).toHaveBeenCalledWith({
      primaryOption: expect.objectContaining({ attribute_id: 27 }),
      secondaryOption: expect.objectContaining({ attribute_id: 87 }),
      skcSelections: { "SKC-1": expect.objectContaining({ valueId: 112 }) },
      skuSelections: { "SKU-1": expect.objectContaining({ valueId: 991 }) },
    });
  });

  it("keeps manual sale attribute editing available after resolution", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "resolved",
              primary_attribute_id: 27,
              secondary_attribute_id: 87,
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              skc_attributes: [
                {
                  scope: "skc",
                  name: "Color",
                  value: "White",
                  attribute_id: 27,
                  attribute_value_id: 112,
                },
              ],
              sku_attributes: [
                {
                  scope: "sku",
                  name: "Size",
                  value: "M",
                  attribute_id: 87,
                  attribute_value_id: 991,
                },
              ],
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  skc_scope: true,
                  attribute_value_list: [
                    { attribute_value_id: 112, value: "White", value_en: "White" },
                  ],
                },
                {
                  attribute_id: 87,
                  name: "Size",
                  name_en: "Size",
                  attribute_value_list: [
                    { attribute_value_id: 991, value: "M", value_en: "M" },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  skc_name: "White",
                  attributes: { Color: "White" },
                  sku_patches: [
                    {
                      supplier_sku: "SKU-1",
                      attributes: { Size: "M" },
                    },
                  ],
                },
              ],
            },
          },
        }}
        onApplyManualSaleAttributes={vi.fn()}
      />,
    );

    expect(screen.getByText("手工填写销售属性")).toBeInTheDocument();
  });

  it("allows manually typing sale attribute text values", async () => {
    const onApplyManual = vi.fn();
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 1001466,
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              skc_attributes: [
                {
                  scope: "skc",
                  name: "Plug(Voltage)",
                  value: "white",
                  attribute_id: 1001466,
                },
              ],
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  skc_scope: true,
                  attribute_value_list: [
                    { attribute_value_id: 112, value: "White", value_en: "White" },
                  ],
                },
                {
                  attribute_id: 87,
                  name: "Size",
                  name_en: "Size",
                  attribute_value_list: [
                    { attribute_value_id: 991, value: "M", value_en: "M" },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  skc_name: "White",
                  attributes: { Color: "White" },
                  sku_patches: [
                    {
                      supplier_sku: "SKU-1",
                      attributes: { Size: "M" },
                    },
                  ],
                },
              ],
            },
          },
        }}
        onApplyManualSaleAttributes={onApplyManual}
      />,
    );

    const textboxes = screen.getAllByRole("textbox");
    await user.clear(textboxes[0]);
    await user.type(textboxes[0], "Cream Beige");
    await user.clear(textboxes[1]);
    await user.type(textboxes[1], '30"×40"');
    await user.click(screen.getByRole("button", { name: "保存手工销售属性" }));

    expect(onApplyManual).toHaveBeenCalledWith({
      primaryOption: expect.objectContaining({ attribute_id: 27 }),
      secondaryOption: expect.objectContaining({ attribute_id: 87 }),
      skcSelections: { "SKC-1": expect.objectContaining({ textValue: "Cream Beige" }) },
      skuSelections: { "SKU-1": expect.objectContaining({ textValue: '30"×40"' }) },
    });
  });
});
