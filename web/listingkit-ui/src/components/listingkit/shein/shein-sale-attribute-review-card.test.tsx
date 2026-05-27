import { render, screen, within } from "@testing-library/react";
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
    expect(screen.getByText("推荐操作")).toBeInTheDocument();
    expect(screen.getByText("下一步")).toBeInTheDocument();
    expect(screen.getByText("状态 待补齐")).toBeInTheDocument();
    expect(screen.getByText("主规格 27")).toBeInTheDocument();
    expect(screen.getByText("当前识别结果")).toBeInTheDocument();
    expect(screen.getByText("为什么会这样匹配")).toBeInTheDocument();
    expect(screen.getByText("当前写入资料包的规格")).toBeInTheDocument();
    expect(screen.getByText("Color：Black")).toBeInTheDocument();
    expect(screen.getByText("Size：One Size")).toBeInTheDocument();
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
                "模板属性 \"Size\" 的值 \"40x30cm\" 校验报错: 验证自定义属性值失败: 没有自定义属性值权限",
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
    expect(
      screen.getByText(/建议切换类目后再重试/),
    ).toBeInTheDocument();
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

    expect(screen.queryByRole("button", { name: "确认当前规格" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重新生成属性" })).toBeInTheDocument();
    expect(screen.getByText(/还缺少真实 `value_id`，不能直接确认/)).toBeInTheDocument();

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

    expect(screen.getByRole("button", { name: "重新生成属性" })).toBeInTheDocument();
    expect(
      screen.getByText(/当前还没有拿到可用的销售属性模板或主规格识别结果/),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "重新生成属性" }));

    expect(onRegenerate).toHaveBeenCalledTimes(1);
  });

  it("shows a loading label while regenerating sale attributes", () => {
    render(
      <SheinSaleAttributeReviewCard
        editorContext={{
          sale_attributes: {
            current: {
              status: "partial",
              review_notes: ["缺少 SHEIN AttributeAPI，当前无法加载销售属性模板"],
            },
          },
        }}
        isApplying
        onRegenerateSaleAttributes={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "重新生成中..." })).toBeDisabled();
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

    expect(screen.getByText("怎么操作")).toBeInTheDocument();
    expect(screen.getByText("结果不对？再手工修正规格")).toBeInTheDocument();
    expect(screen.getByText(/先选主规格字段；2 再选其他规格字段；3 最后给每个 SKC\/SKU 填值/)).toBeInTheDocument();
    expect(screen.getByText(/如果系统当前结果已经正确，可以直接确认/)).toBeInTheDocument();

    await user.selectOptions(screen.getAllByRole("combobox")[0], "27");
    await user.selectOptions(screen.getAllByRole("combobox")[1], "87");
    await user.selectOptions(screen.getAllByRole("combobox")[2], "112");
    await user.selectOptions(screen.getAllByRole("combobox")[3], "991");
    await user.click(screen.getByRole("button", { name: "保存手工修正" }));

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

    expect(screen.getByText("结果不对？再手工修正规格")).toBeInTheDocument();
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
    await user.click(screen.getByRole("button", { name: "保存手工修正" }));

    expect(onApplyManual).toHaveBeenCalledWith({
      primaryOption: expect.objectContaining({ attribute_id: 27 }),
      secondaryOption: expect.objectContaining({ attribute_id: 87 }),
      skcSelections: { "SKC-1": expect.objectContaining({ textValue: "Cream Beige" }) },
      skuSelections: { "SKU-1": expect.objectContaining({ textValue: '30"×40"' }) },
    });
  });

  it("prefers important template as manual primary option", () => {
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
                    { attribute_value_id: 739, value: "Solid", value_en: "Solid" },
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

    const selects = screen.getAllByRole("combobox");
    expect(selects[0]).toHaveValue("1001184");
    expect(
      within(selects[0]).getByRole("option", { name: "Style Type · 主规格" }),
    ).toBeInTheDocument();
    expect(within(selects[0]).queryByRole("option", { name: "Size" })).not.toBeInTheDocument();
  });

  it("keeps important template out of manual secondary default when primary is missing", () => {
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
                    { attribute_value_id: 739, value: "Solid", value_en: "Solid" },
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

    const selects = screen.getAllByRole("combobox");
    expect(selects[0]).toHaveValue("1001184");
    expect(selects[1]).toHaveValue("87");
  });

  it("keeps the selected primary template out of the secondary template options", async () => {
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
                    { attribute_value_id: 112, value: "White", value_en: "White" },
                  ],
                },
                {
                  attribute_id: 1001184,
                  name: "Style Type",
                  name_en: "Style Type",
                  important: true,
                  skc_scope: true,
                  attribute_value_list: [
                    { attribute_value_id: 739, value: "Solid", value_en: "Solid" },
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

    const selects = screen.getAllByRole("combobox");
    await user.selectOptions(selects[0], "27");

    expect(selects[0]).toHaveValue("27");
    expect(
      within(selects[1]).queryByRole("option", { name: "Color · 主规格" }),
    ).not.toBeInTheDocument();
    expect(
      within(selects[1]).getByRole("option", { name: "Style Type · 主规格" }),
    ).toBeInTheDocument();
    expect(within(selects[1]).getByRole("option", { name: "Size" })).toBeInTheDocument();
  });
});
