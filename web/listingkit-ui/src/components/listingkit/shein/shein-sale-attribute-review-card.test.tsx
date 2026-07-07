import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { SheinSaleAttributeReviewCard } from "@/components/listingkit/shein/shein-sale-attribute-review-card";

function getManualCorrectionDetails() {
  const disclosure = screen.getByText("手工修正规格");
  const manualDetails = disclosure.closest("details");
  expect(manualDetails).not.toBeNull();
  return manualDetails!;
}

async function openManualCorrection(user: ReturnType<typeof userEvent.setup>) {
  const manualDetails = getManualCorrectionDetails();
  if (manualDetails.hasAttribute("open")) {
    return;
  }
  const summary = manualDetails?.querySelector("summary");
  expect(summary).not.toBeNull();
  await user.click(summary!);
}

function getSKUSelectionCard(supplierSKU: string) {
  const skuHeading = screen.getByText(supplierSKU);
  const skuCard = skuHeading.parentElement;
  expect(skuCard).not.toBeNull();
  return skuCard!;
}

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
    expect(screen.getByText("主规格 27")).toBeInTheDocument();
    expect(screen.getByText("Color：Black")).toBeInTheDocument();
    expect(screen.getByText("Size：One Size")).toBeInTheDocument();
    expect(screen.getAllByText("Color").length).toBeGreaterThanOrEqual(3);
    expect(screen.getAllByText("Black").length).toBeGreaterThanOrEqual(1);
    expect(
      screen.getAllByText("attribute_id 27 · value_id 112").length,
    ).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Size").length).toBeGreaterThanOrEqual(3);
    expect(screen.getAllByText("One Size").length).toBeGreaterThanOrEqual(1);
    expect(
      screen.getAllByText("attribute_id 87 · value_id 991").length,
    ).toBeGreaterThanOrEqual(1);
    expect(
      screen.getByText("主销售属性已命中，次销售属性仍需确认"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("模板值拟合度 1/2").length,
    ).toBeGreaterThanOrEqual(1);
    expect(
      screen.getAllByText("SKU 维度覆盖 1/1").length,
    ).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("尺码模板候选仍待确认")).toBeInTheDocument();
  });

  it("surfaces a clear hint when the current category denies custom sale attribute values", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "blocked",
              secondary_attribute_id: 87,
              sku_attributes: [
                {
                  scope: "sku",
                  name: "Size",
                  value: "40x30cm",
                  attribute_id: 87,
                },
              ],
              review_notes: [
                '模板属性 "Size" 的值 "40x30cm" 校验报错: 验证自定义属性值失败: 没有自定义属性值权限',
                "已确认没有自定义属性值权限，已跳过自定义尝试",
              ],
            },
          },
        }}
      />,
    );

    expect(
      screen.getByText("当前类目不支持该销售属性自定义值"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/当前类目的「Size」不支持自定义值/),
    ).toBeInTheDocument();
    expect(screen.getByText(/建议切换类目后再重试/)).toBeInTheDocument();
    expect(
      screen.getByText(/已确认没有自定义属性值权限，已跳过自定义尝试/),
    ).toBeInTheDocument();
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

    await user.click(screen.getByRole("button", { name: "直接确认当前结果" }));

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

    expect(
      screen.queryByRole("button", { name: "确认当前规格" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "重新生成属性" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/还缺少真实 `value_id`，不能直接确认/),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "重新生成属性" }));

    expect(onRegenerate).toHaveBeenCalledTimes(1);
  });

  it("offers regeneration when sale attribute templates are still unavailable", async () => {
    const onRegenerate = vi.fn();
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  skc_name: "White",
                  attributes: { Color: "white" },
                  sku_patches: [
                    {
                      supplier_sku: "SKU-1",
                      attributes: { Size: "One size" },
                    },
                  ],
                },
              ],
              review_notes: [
                "缺少 SHEIN AttributeAPI，当前无法加载销售属性模板",
                "当前销售属性仍缺少真实 sale attribute value 映射，请重新确认规格。",
              ],
            },
          },
        }}
        onRegenerateSaleAttributes={onRegenerate}
      />,
    );

    expect(
      screen.getByRole("button", { name: "重新生成属性" }),
    ).toBeInTheDocument();
    expect(screen.getByText("状态 建议重新生成")).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "重新生成属性" }),
    ).toHaveLength(1);
    expect(
      screen.getByText(/当前还没有拿到可用的销售属性模板或主规格识别结果/),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "重新生成属性" }));

    expect(onRegenerate).toHaveBeenCalledTimes(1);
  });

  it("keeps manual correction as a secondary action when regeneration is required and manual editing is available", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
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
                  attributes: { Color: "white" },
                  sku_patches: [
                    {
                      supplier_sku: "SKU-1",
                      attributes: { Size: "One size" },
                    },
                    {
                      supplier_sku: "SKU-2",
                      attributes: { Size: "Two size" },
                    },
                  ],
                },
              ],
              review_notes: [
                "缺少 SHEIN AttributeAPI，当前无法加载销售属性模板",
                "当前销售属性仍缺少真实 sale attribute value 映射，请重新确认规格。",
              ],
            },
          },
        }}
        onRegenerateSaleAttributes={vi.fn()}
        onApplyManualSaleAttributes={vi.fn()}
      />,
    );

    expect(screen.getByText("状态 建议重新生成")).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "重新生成属性" }),
    ).toHaveLength(1);
    expect(
      screen.getByText("手工修正规格"),
    ).toBeInTheDocument();
  });

  it("updates manual-review defaults on rerender without relying on a remount", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
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
              candidates: [
                {
                  source_dimension: "Color",
                  name: "Color",
                  attribute_id: 27,
                  selected_scope: "primary",
                  skc_scope: true,
                },
                {
                  source_dimension: "Size",
                  name: "Size",
                  attribute_id: 87,
                  selected_scope: "secondary",
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  attributes: { Color: "White" },
                  sku_patches: [
                    {
                      supplier_sku: "SKU-1",
                      attributes: { Size: "M" },
                    },
                    {
                      supplier_sku: "SKU-2",
                      attributes: { Size: "L" },
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

    await openManualCorrection(user);
    expect(
      screen.getByLabelText("第 1 步：主规格字段 · 来源 Color"),
    ).toHaveValue("27");
    expect(
      screen.getByLabelText("第 2 步：其他规格字段（必填） · 来源 Size"),
    ).toHaveValue("87");

    rerender(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              template_options: [
                {
                  attribute_id: 1001184,
                  name: "Style Type",
                  name_en: "Style Type",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 739,
                      value: "Solid",
                      value_en: "Solid",
                    },
                  ],
                },
                {
                  attribute_id: 91,
                  name: "Quantity",
                  name_en: "Quantity",
                  attribute_value_list: [
                    {
                      attribute_value_id: 3001,
                      value: "2pc",
                      value_en: "2pc",
                    },
                  ],
                },
              ],
              candidates: [
                {
                  source_dimension: "Color",
                  name: "Style Type",
                  attribute_id: 1001184,
                  selected_scope: "primary",
                  skc_scope: true,
                },
                {
                  source_dimension: "Size",
                  name: "Quantity",
                  attribute_id: 91,
                  selected_scope: "secondary",
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  attributes: { Color: "Black" },
                  sku_patches: [
                    {
                      supplier_sku: "SKU-1",
                      attributes: { Size: "2pc" },
                    },
                    {
                      supplier_sku: "SKU-2",
                      attributes: { Size: "4pc" },
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

    expect(
      screen.getByLabelText("第 1 步：主规格字段 · 来源 Color"),
    ).toHaveValue("1001184");
    expect(
      screen.getByLabelText("第 2 步：其他规格字段（必填） · 来源 Size"),
    ).toHaveValue("91");
  });

  it("marks secondary as optional when the current category has no matching secondary template", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 1001184,
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              skc_attributes: [
                {
                  scope: "skc",
                  name: "Style Type",
                  value: "white",
                  attribute_id: 1001184,
                  attribute_value_id: 739,
                },
              ],
              template_options: [
                {
                  attribute_id: 1001184,
                  name: "Style Type",
                  name_en: "Style Type",
                  important: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 739,
                      value: "white",
                      value_en: "white",
                    },
                  ],
                },
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  attribute_value_list: [
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
                  ],
                },
              ],
              candidates: [
                {
                  source_dimension: "Color",
                  name: "Style Type",
                  attribute_id: 1001184,
                  selected_scope: "primary",
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  skc_name: "white",
                  attributes: { Color: "white" },
                  sku_patches: [
                    {
                      supplier_sku: "SKU-1",
                      attributes: { Size: "30×40cm" },
                    },
                    {
                      supplier_sku: "SKU-2",
                      attributes: { Size: "35×50cm" },
                    },
                  ],
                },
              ],
            },
          },
        }}
        onConfirmCurrentSaleAttributes={vi.fn()}
        onApplyManualSaleAttributes={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "直接确认当前结果" }),
    ).toBeInTheDocument();
    expect(screen.getByText("状态 可直接确认")).toBeInTheDocument();
    expect(screen.queryByText("状态 待补齐")).not.toBeInTheDocument();
    expect(screen.queryByText("怎么操作")).not.toBeInTheDocument();
    expect(screen.queryByText("推荐操作")).not.toBeInTheDocument();
    expect(screen.queryByText("下一步")).not.toBeInTheDocument();
    expect(
      screen.getByText("第 2 步：其他规格字段（选填） · 来源 Size"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "当前类目存在其他规格字段，但没有可用于当前来源维度的模板，可保持只使用主规格。",
      ),
    ).toBeInTheDocument();
    expect(getManualCorrectionDetails()).not.toHaveAttribute("open");
  });

  it("marks secondary as required when a matching secondary template exists", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 27,
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
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
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
              candidates: [
                {
                  source_dimension: "Color",
                  name: "Color",
                  attribute_id: 27,
                  selected_scope: "primary",
                  skc_scope: true,
                },
                {
                  source_dimension: "Size",
                  name: "Size",
                  attribute_id: 87,
                  selected_scope: "secondary",
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
                    {
                      supplier_sku: "SKU-2",
                      attributes: { Size: "L" },
                    },
                  ],
                },
              ],
            },
          },
        }}
        onConfirmCurrentSaleAttributes={vi.fn()}
        onApplyManualSaleAttributes={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "直接确认当前结果" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("状态 需要补其他规格")).toBeInTheDocument();
    expect(
      screen.getByText("第 2 步：其他规格字段（必填） · 来源 Size"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("手工修正规格"),
    ).toBeInTheDocument();
  });

  it("shows a loading label while regenerating sale attributes", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              review_notes: [
                "缺少 SHEIN AttributeAPI，当前无法加载销售属性模板",
              ],
            },
          },
        }}
        isApplying
        onRegenerateSaleAttributes={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "重新生成中..." }),
    ).toBeDisabled();
  });

  it("shows manual correction as a collapsed disclosure when editing is available", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  attribute_value_list: [
                    { attribute_value_id: 112, value: "White" },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  skc_name: "White",
                  attributes: { Color: "White" },
                },
              ],
            },
          },
        }}
        onApplyManualSaleAttributes={vi.fn()}
      />,
    );

    expect(getManualCorrectionDetails()).not.toHaveAttribute("open");
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
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
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

    await openManualCorrection(user);

    await user.selectOptions(
      screen.getByLabelText("第 1 步：主规格字段 · 来源 Color"),
      "27",
    );
    await user.selectOptions(
      screen.getByLabelText(/第 2 步：其他规格字段.*来源 Size/),
      "87",
    );
    await user.selectOptions(
      screen.getByLabelText(/第 3 步：主规格值/),
      "112",
    );

    const skuRow = getSKUSelectionCard("SKU-1");
    await user.selectOptions(
      within(skuRow).getByLabelText(/第 3 步：其他规格值/),
      "991",
    );
    await user.click(screen.getByRole("button", { name: "保存手工修正" }));

    expect(onApplyManual).toHaveBeenCalledWith({
      primaryOption: expect.objectContaining({ attribute_id: 27 }),
      secondaryOption: expect.objectContaining({ attribute_id: 87 }),
      skcSelections: { "SKC-1": expect.objectContaining({ valueId: 112 }) },
      skuSelections: { "SKU-1": expect.objectContaining({ valueId: 991 }) },
    });
  });

  it("clears stale manual value selections after template field changes", async () => {
    const onApplyManual = vi.fn();
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
                  ],
                },
                {
                  attribute_id: 1001184,
                  name: "Style Type",
                  name_en: "Style Type",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 739,
                      value: "Solid",
                      value_en: "Solid",
                    },
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
                {
                  attribute_id: 91,
                  name: "Quantity",
                  name_en: "Quantity",
                  attribute_value_list: [
                    {
                      attribute_value_id: 3001,
                      value: "2pc",
                      value_en: "2pc",
                    },
                  ],
                },
              ],
              candidates: [
                {
                  source_dimension: "Color",
                  name: "Color",
                  attribute_id: 27,
                  selected_scope: "primary",
                  skc_scope: true,
                },
                {
                  source_dimension: "Size",
                  name: "Size",
                  attribute_id: 87,
                  selected_scope: "secondary",
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
                    {
                      supplier_sku: "SKU-2",
                      attributes: { Size: "L" },
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

    await openManualCorrection(user);

    await user.selectOptions(
      screen.getByLabelText("第 1 步：主规格字段 · 来源 Color"),
      "27",
    );
    await user.selectOptions(
      screen.getByLabelText("第 2 步：其他规格字段（必填） · 来源 Size"),
      "87",
    );
    await user.selectOptions(screen.getByLabelText(/第 3 步：主规格值/), "112");
    const firstSkuRow = getSKUSelectionCard("SKU-1");
    await user.selectOptions(
      within(firstSkuRow).getByLabelText(/第 3 步：其他规格值/),
      "991",
    );
    const secondSkuRow = getSKUSelectionCard("SKU-2");
    await user.selectOptions(
      within(secondSkuRow).getByLabelText(/第 3 步：其他规格值/),
      "991",
    );

    expect(
      screen.getByRole("button", { name: "保存手工修正" }),
    ).toBeInTheDocument();

    await user.selectOptions(
      screen.getByLabelText("第 1 步：主规格字段 · 来源 Color"),
      "1001184",
    );
    await user.selectOptions(
      screen.getByLabelText("第 2 步：其他规格字段（必填） · 来源 Size"),
      "91",
    );

    expect(
      screen.getByRole("button", { name: "保存手工修正" }),
    ).toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText(/第 3 步：主规格值/), "739");
    await user.selectOptions(
      within(firstSkuRow).getByLabelText(/第 3 步：其他规格值/),
      "3001",
    );
    await user.selectOptions(
      within(secondSkuRow).getByLabelText(/第 3 步：其他规格值/),
      "3001",
    );
    await user.click(screen.getByRole("button", { name: "保存手工修正" }));

    expect(onApplyManual).toHaveBeenCalledWith({
      primaryOption: expect.objectContaining({ attribute_id: 1001184 }),
      secondaryOption: expect.objectContaining({ attribute_id: 91 }),
      skcSelections: { "SKC-1": expect.objectContaining({ valueId: 739 }) },
      skuSelections: {
        "SKU-1": expect.objectContaining({ valueId: 3001 }),
        "SKU-2": expect.objectContaining({ valueId: 3001 }),
      },
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
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
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

    expect(screen.getByText("手工修正规格")).toBeInTheDocument();
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
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
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

    await openManualCorrection(user);

    const primaryTextbox = screen.getByPlaceholderText("手工输入，建议值：White");
    await user.clear(primaryTextbox);
    await user.type(primaryTextbox, "Cream Beige");

    const skuRow = getSKUSelectionCard("SKU-1");
    const secondaryTextbox =
      within(skuRow).getByPlaceholderText("手工输入，建议值：M");
    await user.clear(secondaryTextbox);
    await user.type(secondaryTextbox, '30"×40"');
    await user.click(screen.getByRole("button", { name: "保存手工修正" }));

    expect(onApplyManual).toHaveBeenCalledWith({
      primaryOption: expect.objectContaining({ attribute_id: 27 }),
      secondaryOption: expect.objectContaining({ attribute_id: 87 }),
      skcSelections: {
        "SKC-1": expect.objectContaining({ textValue: "Cream Beige" }),
      },
      skuSelections: {
        "SKU-1": expect.objectContaining({ textValue: '30"×40"' }),
      },
    });
  });

  it("prefers the selected template value after switching from manual text back to the dropdown", async () => {
    const onApplyManual = vi.fn();
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
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
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
                    {
                      attribute_value_id: 114,
                      value: "Black",
                      value_en: "Black",
                    },
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

    await openManualCorrection(user);

    const primaryTextbox = screen.getByPlaceholderText("手工输入，建议值：White");
    await user.clear(primaryTextbox);
    await user.type(primaryTextbox, "Cream Beige");
    await user.selectOptions(screen.getByLabelText(/第 3 步：主规格值/), "114");

    const skuRow = getSKUSelectionCard("SKU-1");
    await user.selectOptions(
      within(skuRow).getByLabelText(/第 3 步：其他规格值/),
      "991",
    );
    await user.click(screen.getByRole("button", { name: "保存手工修正" }));

    expect(onApplyManual).toHaveBeenCalledWith({
      primaryOption: expect.objectContaining({ attribute_id: 27 }),
      secondaryOption: expect.objectContaining({ attribute_id: 87 }),
      skcSelections: {
        "SKC-1": expect.objectContaining({ valueId: 114, textValue: "" }),
      },
      skuSelections: {
        "SKU-1": expect.objectContaining({ valueId: 991 }),
      },
    });
  });

  it("initializes refreshed SKU selections from saved sale attributes instead of the original source text", async () => {
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
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
                  value: "XXXXL",
                  attribute_id: 87,
                  attribute_value_id: 4004,
                },
              ],
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
                  ],
                },
                {
                  attribute_id: 87,
                  name: "Size",
                  name_en: "Size",
                  attribute_value_list: [
                    { attribute_value_id: 5005, value: "5XL", value_en: "5XL" },
                    {
                      attribute_value_id: 4004,
                      value: "XXXXL",
                      value_en: "XXXXL",
                    },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  skc_name: "White",
                  attributes: { Color: "White" },
                  sale_attribute: {
                    scope: "skc",
                    name: "Color",
                    value: "White",
                    attribute_id: 27,
                    attribute_value_id: 112,
                  },
                  sku_patches: [
                    {
                      supplier_sku: "NS6104229008",
                      attributes: { Size: "5XL" },
                      sale_attributes: [
                        {
                          scope: "sku",
                          name: "Size",
                          value: "XXXXL",
                          attribute_id: 87,
                          attribute_value_id: 4004,
                        },
                      ],
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

    await openManualCorrection(user);

    const skuRow = getSKUSelectionCard("NS6104229008");
    expect(within(skuRow).getByLabelText(/第 3 步：其他规格值/)).toHaveValue(
      "4004",
    );
    expect(within(skuRow).getByPlaceholderText("手工输入，建议值：5XL")).toHaveValue(
      "",
    );
  });

  it("prefers important template as manual primary option", async () => {
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              template_options: [
                {
                  attribute_id: 87,
                  name: "Size",
                  name_en: "Size",
                  skc_scope: false,
                  attribute_value_list: [
                    { attribute_value_id: 991, value: "M", value_en: "M" },
                  ],
                },
                {
                  attribute_id: 1001184,
                  name: "Style Type",
                  name_en: "Style Type",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 739,
                      value: "Solid",
                      value_en: "Solid",
                    },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  attributes: { Color: "White" },
                },
              ],
            },
          },
        }}
        onApplyManualSaleAttributes={vi.fn()}
      />,
    );

    await openManualCorrection(user);
    const primaryTemplateSelect = screen.getByLabelText(
      "第 1 步：主规格字段 · 来源 Color",
    );
    expect(primaryTemplateSelect).toHaveValue("1001184");
    expect(
      within(primaryTemplateSelect).getByRole("option", {
        name: "Style Type · 主规格",
      }),
    ).toBeInTheDocument();
    expect(
      within(primaryTemplateSelect).queryByRole("option", { name: "Size" }),
    ).not.toBeInTheDocument();
  });

  it("keeps important template out of manual secondary default when primary is missing", async () => {
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              template_options: [
                {
                  attribute_id: 1001184,
                  name: "Style Type",
                  name_en: "Style Type",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 739,
                      value: "Solid",
                      value_en: "Solid",
                    },
                  ],
                },
                {
                  attribute_id: 87,
                  name: "Size",
                  name_en: "Size",
                  skc_scope: false,
                  attribute_value_list: [
                    { attribute_value_id: 991, value: "M", value_en: "M" },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
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

    await openManualCorrection(user);
    expect(
      screen.getByLabelText("第 1 步：主规格字段 · 来源 Color"),
    ).toHaveValue("1001184");
    expect(
      screen.getByLabelText("第 2 步：其他规格字段（选填） · 来源 Size"),
    ).toHaveValue("87");
  });

  it("keeps an explicit empty optional-secondary selection stable and saves without a secondary option", async () => {
    const onApplyManual = vi.fn();
    const user = userEvent.setup();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
                  ],
                },
                {
                  attribute_id: 87,
                  name: "Size",
                  name_en: "Size",
                  skc_scope: false,
                  attribute_value_list: [
                    { attribute_value_id: 991, value: "M", value_en: "M" },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
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

    await openManualCorrection(user);

    const secondaryTemplateSelect = screen.getByLabelText(
      "第 2 步：其他规格字段（选填） · 来源 Size",
    );
    expect(secondaryTemplateSelect).toHaveValue("87");

    await user.selectOptions(secondaryTemplateSelect, "");

    expect(secondaryTemplateSelect).toHaveValue("");
    await user.selectOptions(screen.getByLabelText(/第 3 步：主规格值/), "112");
    await user.click(screen.getByRole("button", { name: "保存手工修正" }));

    expect(onApplyManual).toHaveBeenCalledWith({
      primaryOption: expect.objectContaining({ attribute_id: 27 }),
      secondaryOption: null,
      skcSelections: {
        "SKC-1": expect.objectContaining({ valueId: 112 }),
      },
      skuSelections: {},
    });
  });

  it("keeps the selected primary template out of the secondary template options and submits the visible fallback option", async () => {
    const user = userEvent.setup();
    const onApplyManual = vi.fn();

    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 27,
              secondary_attribute_id: 1001184,
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
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 112,
                      value: "White",
                      value_en: "White",
                    },
                  ],
                },
                {
                  attribute_id: 1001184,
                  name: "Style Type",
                  name_en: "Style Type",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 739,
                      value: "Solid",
                      value_en: "Solid",
                    },
                  ],
                },
                {
                  attribute_id: 87,
                  name: "Size",
                  name_en: "Size",
                  skc_scope: false,
                  attribute_value_list: [
                    { attribute_value_id: 991, value: "M", value_en: "M" },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
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

    await openManualCorrection(user);
    const primaryTemplateSelect = screen.getByLabelText(
      "第 1 步：主规格字段 · 来源 Color",
    );
    const secondaryTemplateSelect = screen.getByLabelText(
      "第 2 步：其他规格字段（选填） · 来源 Size",
    );
    expect(secondaryTemplateSelect).toHaveValue("1001184");

    await user.selectOptions(primaryTemplateSelect, "1001184");

    expect(primaryTemplateSelect).toHaveValue("1001184");
    expect(secondaryTemplateSelect).toHaveValue("87");
    expect(
      within(secondaryTemplateSelect).queryByRole("option", {
        name: "Style Type",
      }),
    ).not.toBeInTheDocument();
    expect(
      within(secondaryTemplateSelect).getByRole("option", {
        name: "Color · 主规格",
      }),
    ).toBeInTheDocument();
    expect(
      within(secondaryTemplateSelect).getByRole("option", { name: "Size" }),
    ).toBeInTheDocument();

    await user.selectOptions(
      screen.getByLabelText(/第 3 步：主规格值/),
      "739",
    );
    const skuRow = getSKUSelectionCard("SKU-1");
    await user.selectOptions(
      within(skuRow).getByLabelText(/第 3 步：其他规格值/),
      "991",
    );
    await user.click(screen.getByRole("button", { name: "保存手工修正" }));

    expect(onApplyManual).toHaveBeenCalledWith({
      primaryOption: expect.objectContaining({ attribute_id: 1001184 }),
      secondaryOption: expect.objectContaining({ attribute_id: 87 }),
      skcSelections: {
        "SKC-1": expect.objectContaining({ valueId: 739 }),
      },
      skuSelections: {
        "SKU-1": expect.objectContaining({ valueId: 991 }),
      },
    });
  });

  it("shows in-card sale attribute refresh feedback", () => {
    render(
      <SheinSaleAttributeReviewCard
        statusMessage="已触发销售属性刷新，系统会重新拉取模板并刷新颜色、尺寸等映射。"
        statusTone="success"
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              review_notes: [
                "缺少 SHEIN AttributeAPI，当前无法加载销售属性模板",
              ],
            },
          },
        }}
      />,
    );

    expect(
      screen.getByText(
        "已触发销售属性刷新，系统会重新拉取模板并刷新颜色、尺寸等映射。",
      ),
    ).toBeInTheDocument();
  });

  it("opens manual correction by default when the secondary sale attribute is required", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              primary_attribute_id: 27,
              secondary_attribute_id: 87,
              primary_source_dimension: "Color",
              secondary_source_dimension: "Size",
              skc_attributes: [
                {
                  scope: "skc",
                  name: "Color",
                  value: "white",
                  attribute_id: 27,
                  attribute_value_id: 739,
                },
              ],
              sku_attributes: [
                {
                  scope: "sku",
                  name: "Size",
                  value: "M",
                  attribute_id: 87,
                  attribute_value_id: 417,
                },
              ],
              template_options: [
                {
                  attribute_id: 27,
                  name: "Color",
                  name_en: "Color",
                  skc_scope: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 739,
                      value: "White",
                      value_en: "White",
                    },
                  ],
                },
                {
                  attribute_id: 87,
                  name: "Size",
                  name_en: "Size",
                  attribute_value_list: [
                    { attribute_value_id: 417, value: "M", value_en: "M" },
                  ],
                },
              ],
              skc_patches: [
                {
                  supplier_code: "SKC-1",
                  skc_name: "white",
                  attributes: { Color: "white" },
                  sku_patches: [
                    {
                      supplier_sku: "SKU-S",
                      attributes: { Size: "S" },
                    },
                    {
                      supplier_sku: "SKU-M",
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

    expect(getManualCorrectionDetails()).toHaveAttribute("open");
    expect(screen.getByText("收起")).toBeInTheDocument();
    expect(screen.getByLabelText(/第 2 步：其他规格字段/)).toBeInTheDocument();
    expect(screen.getByText("SKU-S")).toBeInTheDocument();
    expect(screen.getByText("SKU-M")).toBeInTheDocument();
    expect(
      within(getSKUSelectionCard("SKU-S")).getByPlaceholderText(
        "手工输入，建议值：S",
      ),
    ).toHaveValue("S");
    expect(
      screen.getByRole("button", { name: "保存手工修正" }),
    ).toBeInTheDocument();
  });
});
