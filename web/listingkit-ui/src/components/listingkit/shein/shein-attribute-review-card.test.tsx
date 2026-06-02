import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SheinAttributeReviewCard } from "@/components/listingkit/shein/shein-attribute-review-card";

describe("SheinAttributeReviewCard", () => {
  it("renders attribute mapping summary and notes", () => {
    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              resolved_count: 3,
              unresolved_count: 1,
              resolved_attributes: [
                {
                  name: "Upper Material",
                  value: "flyknit",
                  attribute_id: 112,
                  attribute_value_id: 5930427,
                },
              ],
              review_notes: ["还有 1 个普通属性未命中模板值"],
            },
          },
        }}
      />,
    );

    expect(screen.getByText("SHEIN 普通属性确认")).toBeInTheDocument();
    expect(screen.getByText("已确认属性")).toBeInTheDocument();
    expect(screen.getByText("Upper Material")).toBeInTheDocument();
    expect(screen.getByText("flyknit")).toBeInTheDocument();
    expect(screen.getByText("attribute_id 112 · value_id 5930427")).toBeInTheDocument();
    expect(screen.getByText("还有 1 个普通属性未命中模板值")).toBeInTheDocument();
  });

  it("allows regenerating current attributes", async () => {
    const user = userEvent.setup();
    const onRegenerateAttributes = vi.fn();

    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              review_notes: ["还有 1 个普通属性未命中模板值"],
            },
          },
        }}
        onRegenerateAttributes={onRegenerateAttributes}
      />,
    );

    await user.click(screen.getByRole("button", { name: "重新生成普通属性" }));

    expect(onRegenerateAttributes).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "重新生成普通属性" })).toHaveClass("w-full");
  });

  it("returns manually selected pending attribute candidates", async () => {
    const user = userEvent.setup();
    const onConfirmAttributes = vi.fn();

    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              pending_attribute_candidates: [
                {
                  name: "Power Supply",
                  attribute_id: 147,
                  attribute_type: 4,
                  attribute_mode: 1,
                  required: true,
                  important: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 3165066,
                      value: "电池式(其他类电池)",
                      value_en: "Battery Powered(Others Battery)",
                    },
                  ],
                },
              ],
            },
          },
        }}
        onConfirmAttributes={onConfirmAttributes}
      />,
    );

    expect(screen.getByText("必填未完成")).toBeInTheDocument();
    expect(screen.getByText(/attribute_id 147 · 必填 · 重要/i)).toBeInTheDocument();
    expect(screen.getByText("SHEIN 模板必填，未确认会阻断提交。")).toBeInTheDocument();

    await user.selectOptions(
      screen.getByRole("combobox"),
      "3165066",
    );
    await user.click(screen.getByRole("button", { name: "保存已选择属性" }));

    expect(onConfirmAttributes).toHaveBeenCalledWith([
      expect.objectContaining({
        name: "Power Supply",
        value: "Battery Powered(Others Battery)",
        attribute_id: 147,
        attribute_value_id: 3165066,
        matched_by: "manual_attribute_review",
      }),
    ]);
  });

  it("renders and applies recommended optional attribute candidates", async () => {
    const user = userEvent.setup();
    const onConfirmAttributes = vi.fn();

    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "resolved",
              recommended_attribute_candidates: [
                {
                  name: "Product Features",
                  attribute_id: 260,
                  attribute_type: 4,
                  attribute_mode: 1,
                  important: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 1001,
                      value: "无",
                      value_en: "None",
                    },
                  ],
                },
              ],
            },
          },
        }}
        onConfirmAttributes={onConfirmAttributes}
      />,
    );

    expect(screen.getByText("重要建议")).toBeInTheDocument();
    expect(screen.getByText(/attribute_id 260/i)).toBeInTheDocument();

    await user.selectOptions(screen.getByRole("combobox"), "1001");
    await user.click(screen.getByRole("button", { name: "保存已选择属性" }));

    expect(onConfirmAttributes).toHaveBeenCalledWith([
      expect.objectContaining({
        name: "Product Features",
        value: "None",
        attribute_id: 260,
        attribute_value_id: 1001,
      }),
    ]);
  });

  it("shows non-required candidates as non-blocking optional details", () => {
    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              pending_attribute_candidates: [
                {
                  name: "Composition",
                  attribute_id: 991,
                  required: false,
                  important: false,
                },
              ],
            },
          },
        }}
      />,
    );

    expect(screen.getByText("其他建议属性（不阻断提交）")).toBeInTheDocument();
    expect(screen.queryByText("必填未完成")).not.toBeInTheDocument();
  });

  it("shows apply revision errors near the attribute form", () => {
    render(
      <SheinAttributeReviewCard
        applyErrorMessage='resolve shein sku sale attribute "40x30cm" failed'
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              pending_attribute_candidates: [
                {
                  name: "Quantity",
                  attribute_id: 1000411,
                  required: true,
                },
              ],
            },
          },
        }}
      />,
    );

    expect(
      screen.getByText(
        '保存普通属性失败：resolve shein sku sale attribute "40x30cm" failed',
      ),
    ).toBeInTheDocument();
  });

  it("keeps resolved attribute summaries on a mobile-first grid", () => {
    const { container } = render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "resolved",
              resolved_attributes: [
                { name: "Upper Material", value: "flyknit" },
              ],
            },
          },
        }}
      />,
    );

    expect(
      Array.from(container.querySelectorAll("div")).some((element) =>
        String(element.className).includes("xl:grid-cols-2"),
      ),
    ).toBe(true);
  });

  it("allows manual extra value input for option-based attributes", async () => {
    const user = userEvent.setup();
    const onConfirmAttributes = vi.fn();

    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              pending_attribute_candidates: [
                {
                  name: "Quantity",
                  attribute_id: 1000411,
                  attribute_type: 4,
                  attribute_mode: 4,
                  data_dimension: 2,
                  required: true,
                  important: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 1002451,
                      value: "件",
                      value_en: "piece(s)",
                    },
                  ],
                },
              ],
            },
          },
        }}
        onConfirmAttributes={onConfirmAttributes}
      />,
    );

    await user.selectOptions(screen.getByRole("combobox"), "1002451");
    await user.type(
      screen.getByPlaceholderText("如模板要求手填补充值，可在这里输入数字或文本"),
      "3",
    );
    await user.click(screen.getByRole("button", { name: "保存已选择属性" }));

    expect(onConfirmAttributes).toHaveBeenCalledWith([
      expect.objectContaining({
        name: "Quantity",
        value: "piece(s)",
        attribute_id: 1000411,
        attribute_value_id: 1002451,
        attribute_extra_value: "3",
        matched_by: "manual_attribute_review",
      }),
    ]);
  });

  it("allows selecting multiple values for multi-input attributes", async () => {
    const user = userEvent.setup();
    const onConfirmAttributes = vi.fn();

    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              pending_attribute_candidates: [
                {
                  name: "Applicable Space",
                  attribute_id: 2000,
                  attribute_type: 4,
                  attribute_mode: 1,
                  attribute_input_num: 2,
                  required: true,
                  important: true,
                  attribute_value_list: [
                    {
                      attribute_value_id: 2001,
                      value: "客厅",
                      value_en: "Living Room",
                    },
                    {
                      attribute_value_id: 2002,
                      value: "卧室",
                      value_en: "Bedroom",
                    },
                  ],
                },
              ],
            },
          },
        }}
        onConfirmAttributes={onConfirmAttributes}
      />,
    );

    await user.click(screen.getByRole("checkbox", { name: /Living Room/i }));
    await user.click(screen.getByRole("checkbox", { name: /Bedroom/i }));
    await user.click(screen.getByRole("button", { name: "保存已选择属性" }));

    expect(onConfirmAttributes).toHaveBeenCalledWith([
      expect.objectContaining({
        name: "Applicable Space",
        value: "Living Room",
        attribute_id: 2000,
        attribute_value_id: 2001,
        matched_by: "manual_attribute_review",
      }),
      expect.objectContaining({
        name: "Applicable Space",
        value: "Bedroom",
        attribute_id: 2000,
        attribute_value_id: 2002,
        matched_by: "manual_attribute_review",
      }),
    ]);
  });

  it("allows manual text entry when a required template attribute has no options", async () => {
    const user = userEvent.setup();
    const onConfirmAttributes = vi.fn();

    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              pending_attribute_candidates: [
                {
                  name: "Product Model",
                  attribute_id: 1000546,
                  attribute_type: 1,
                  attribute_mode: 2,
                  required: true,
                  important: true,
                  attribute_value_list: [],
                },
              ],
            },
          },
        }}
        onConfirmAttributes={onConfirmAttributes}
      />,
    );

    expect(
      screen.getByText("这个模板属性没有可选值，请手工输入要写入 SHEIN 的文本属性值。"),
    ).toBeInTheDocument();

    await user.type(
      screen.getByPlaceholderText("输入属性值，例如型号或材质"),
      "MG17701061001",
    );
    await user.click(screen.getByRole("button", { name: "保存已选择属性" }));

    expect(onConfirmAttributes).toHaveBeenCalledWith([
      expect.objectContaining({
        name: "Product Model",
        value: "MG17701061001",
        attribute_id: 1000546,
        attribute_extra_value: "MG17701061001",
        matched_by: "manual_attribute_review",
      }),
    ]);
  });

  it("allows fallback confirmation when template candidates are unavailable", async () => {
    const user = userEvent.setup();
    const onConfirmFallbackAttributes = vi.fn();

    render(
      <SheinAttributeReviewCard
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              source: "fallback",
              pending_attributes: [
                { name: "material", value: "涤纶" },
                { name: "product_sku", value: "MG8089002" },
              ],
              review_notes: [
                "缺少 SHEIN AttributeAPI，当前无法加载在线属性模板",
                "SHEIN 店铺 cookie 不可用，已降级为离线解析",
              ],
            },
          },
        }}
        onConfirmFallbackAttributes={onConfirmFallbackAttributes}
      />,
    );

    expect(screen.getByText("当前没有可选的 SHEIN 模板候选值")).toBeInTheDocument();
    expect(screen.getByText("material")).toBeInTheDocument();
    expect(screen.getByText("涤纶")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "按当前 SDS 属性确认" }));

    expect(onConfirmFallbackAttributes).toHaveBeenCalledTimes(1);
  });

  it("shows in-card attribute refresh feedback", () => {
    render(
      <SheinAttributeReviewCard
        statusMessage="已触发普通属性刷新，系统会按当前类目模板重新生成属性候选。"
        statusTone="success"
        editorContext={{
          attributes: {
            current: {
              status: "partial",
              review_notes: ["还有 1 个普通属性未命中模板值"],
            },
          },
        }}
      />,
    );

    expect(
      screen.getByText("已触发普通属性刷新，系统会按当前类目模板重新生成属性候选。"),
    ).toBeInTheDocument();
  });
});
