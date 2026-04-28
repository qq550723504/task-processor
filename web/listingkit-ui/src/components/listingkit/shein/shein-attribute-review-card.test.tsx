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
});
