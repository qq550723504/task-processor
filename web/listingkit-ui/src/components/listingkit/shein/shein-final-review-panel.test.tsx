import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SheinFinalReviewPanel } from "@/components/listingkit/shein/shein-final-review-panel";

describe("SheinFinalReviewPanel", () => {
  it("refreshes final price inputs when pricing review updates before user edits", () => {
    const { rerender } = render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          pricing: {
            updated_at: "2026-06-11T10:00:00Z",
            sku_prices: [
              {
                supplier_sku: "SKU-1",
                cost_cny: 91,
                calculated_price: 19.99,
                final_price: 19.99,
                currency: "USD",
              },
            ],
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            images: [{ url: "https://example.com/main.jpg", main: true, final: true }],
          },
        }}
      />,
    );

    expect(screen.getByDisplayValue("19.99")).toBeInTheDocument();

    rerender(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          pricing: {
            updated_at: "2026-06-11T10:05:00Z",
            sku_prices: [
              {
                supplier_sku: "SKU-1",
                cost_cny: 91,
                calculated_price: 21.99,
                final_price: 21.99,
                currency: "USD",
              },
            ],
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            images: [{ url: "https://example.com/main.jpg", main: true, final: true }],
          },
        }}
      />,
    );

    expect(screen.getByDisplayValue("21.99")).toBeInTheDocument();
  });

  it("does not overwrite edited final price inputs when pricing review updates", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          pricing: {
            updated_at: "2026-06-11T10:00:00Z",
            sku_prices: [
              {
                supplier_sku: "SKU-1",
                cost_cny: 91,
                calculated_price: 19.99,
                final_price: 19.99,
                currency: "USD",
              },
            ],
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            images: [{ url: "https://example.com/main.jpg", main: true, final: true }],
          },
        }}
      />,
    );

    const input = screen.getByDisplayValue("19.99");
    await user.clear(input);
    await user.type(input, "25.55");

    rerender(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          pricing: {
            updated_at: "2026-06-11T10:05:00Z",
            sku_prices: [
              {
                supplier_sku: "SKU-1",
                cost_cny: 91,
                calculated_price: 21.99,
                final_price: 21.99,
                currency: "USD",
              },
            ],
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            images: [{ url: "https://example.com/main.jpg", main: true, final: true }],
          },
        }}
      />,
    );

    expect(screen.getByDisplayValue("25.55")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("21.99")).not.toBeInTheDocument();
  });

  it("shows blocked summary and repair actions", async () => {
    const user = userEvent.setup();
    const onSelectBlockingItem = vi.fn();

    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: {
            ready: false,
            blocking_items: [
              { key: "attributes", label: "属性", message: "材质必填" },
            ],
          },
          final_review: {
            category_id: 123,
            images: [
              { url: "https://example.com/main.jpg", main: true, final: true },
            ],
            skus: [{ supplier_sku: "SKU-1" }],
          },
        }}
        canSelectBlockingItem={() => true}
        onSelectBlockingItem={onSelectBlockingItem}
      />,
    );

    expect(screen.getByText("提交前检查")).toBeInTheDocument();
    expect(screen.getByText("还差 1 个阻断项，修复后才能提交。")).toBeInTheDocument();
    expect(screen.getByText("普通属性")).toBeInTheDocument();
    expect(screen.getByText("图片资料")).toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "去确认属性" })[0]);
    expect(onSelectBlockingItem).toHaveBeenCalledWith(
      expect.objectContaining({ key: "attributes" }),
    );
  });

  it("shows image role summary without blocking on missing swatch or size map", () => {
    const onSelectBlockingItem = vi.fn();

    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: false },
          final_review: {
            confirmed: false,
            images: [
              { url: "https://example.com/main.jpg", role: "main", final: true },
              { url: "https://example.com/gallery.jpg", role: "gallery", final: true },
            ],
          },
        }}
        onSelectBlockingItem={onSelectBlockingItem}
      />,
    );

    expect(
      screen.getByText("最终图片 2 张 · 主图 1 张 · 色块图 0 张 · SKC 图 0 张 · 尺寸图 0 张 · 图库 1 张"),
    ).toBeInTheDocument();
    expect(screen.getByText("图片结构完整")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "去检查图片" })).not.toBeInTheDocument();
    expect(onSelectBlockingItem).not.toHaveBeenCalled();
  });

  it("shows resolved store explanation when preview includes store resolution", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          store_resolution: {
            store_id: 903,
            site: "GB",
            strategy: "country",
            reason: "根据任务国家信息命中了对应店铺。",
            matched_rule_kinds: ["country"],
            matched_profile_id: 17,
            resolved_at: "2026-05-18T08:15:00Z",
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            images: [{ url: "https://example.com/main.jpg", main: true, final: true }],
          },
        }}
      />,
    );

    expect(screen.getByText("店铺解析")).toBeInTheDocument();
    expect(screen.getByText("SHEIN 店铺 903 · GB")).toBeInTheDocument();
    expect(screen.getByText("根据任务国家信息命中了对应店铺。")).toBeInTheDocument();
    expect(screen.getByText("Profile #17")).toBeInTheDocument();
    expect(screen.getByText(/固化时间：/)).toBeInTheDocument();
  });

  it("shows generated SHEIN size chart from preview payload", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          editor_context: {
            attributes: {
              current: {
                size_chart_attributes: [
                  {
                    attribute_id: 55,
                    attribute_name: "长度 (cm)",
                    attribute_name_en: "Length (cm)",
                  },
                  {
                    attribute_id: 20,
                    attribute_name: "胸围 (cm)",
                    attribute_name_en: "Bust (cm)",
                  },
                ],
              },
            },
          },
          draft_payload: {
            skc_list: [
              {
                sku_list: [
                  {
                    sale_attributes: [
                      {
                        name: "Size",
                        value: "S",
                        attribute_id: 87,
                        attribute_value_id: 568,
                      },
                    ],
                  },
                  {
                    sale_attributes: [
                      {
                        name: "Size",
                        value: "M",
                        attribute_id: 87,
                        attribute_value_id: 417,
                      },
                    ],
                  },
                ],
              },
            ],
          },
          preview_payload: {
            size_attribute_list: [
              {
                attribute_id: 55,
                attribute_extra_value: "87.5",
                relate_sale_attribute_id: 87,
                relate_sale_attribute_value_id: 568,
              },
              {
                attribute_id: 20,
                attribute_extra_value: "87",
                relate_sale_attribute_id: 87,
                relate_sale_attribute_value_id: 568,
              },
              {
                attribute_id: 55,
                attribute_extra_value: "88.5",
                relate_sale_attribute_id: 87,
                relate_sale_attribute_value_id: 417,
              },
              {
                attribute_id: 20,
                attribute_extra_value: "91",
                relate_sale_attribute_id: 87,
                relate_sale_attribute_value_id: 417,
              },
            ],
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            images: [{ url: "https://example.com/main.jpg", main: true, final: true }],
          },
        }}
      />,
    );

    expect(screen.getByText("SHEIN 尺码表")).toBeInTheDocument();
    expect(screen.getByText("2 个尺码 · 2 个尺码字段")).toBeInTheDocument();
    expect(screen.getByText("Length (cm)")).toBeInTheDocument();
    expect(screen.getByText("Bust (cm)")).toBeInTheDocument();
    expect(screen.getByText("S")).toBeInTheDocument();
    expect(screen.getByText("M")).toBeInTheDocument();
    expect(screen.getByText("87.5")).toBeInTheDocument();
    expect(screen.getByText("91")).toBeInTheDocument();
  });

  it("saves manually completed required SHEIN size chart attributes", async () => {
    const user = userEvent.setup();
    const onSaveFinalDraft = vi.fn();

    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: false },
          editor_context: {
            attributes: {
              current: {
                size_chart_attributes: [
                  {
                    attribute_id: 55,
                    attribute_name: "长度 (cm)",
                    attribute_name_en: "Length (cm)",
                    required: true,
                  },
                  {
                    attribute_id: 20,
                    attribute_name: "胸围 (cm)",
                    attribute_name_en: "Bust (cm)",
                    required: true,
                  },
                ],
              },
            },
          },
          draft_payload: {
            skc_list: [
              {
                sku_list: [
                  {
                    sale_attributes: [
                      {
                        name: "Size",
                        value: "S",
                        attribute_id: 87,
                        attribute_value_id: 568,
                      },
                    ],
                  },
                  {
                    sale_attributes: [
                      {
                        name: "Size",
                        value: "M",
                        attribute_id: 87,
                        attribute_value_id: 417,
                      },
                    ],
                  },
                ],
              },
            ],
          },
          preview_payload: {
            size_attribute_list: [
              {
                attribute_id: 55,
                attribute_extra_value: "87.5",
                relate_sale_attribute_id: 87,
                relate_sale_attribute_value_id: 568,
              },
              {
                attribute_id: 55,
                attribute_extra_value: "88.5",
                relate_sale_attribute_id: 87,
                relate_sale_attribute_value_id: 417,
              },
            ],
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            images: [{ url: "https://example.com/main.jpg", main: true, final: true }],
          },
        }}
        onSaveFinalDraft={onSaveFinalDraft}
      />,
    );

    expect(screen.getByText("待补 2 项")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存尺码表" })).toBeDisabled();

    await user.type(screen.getByRole("textbox", { name: "S Bust (cm)" }), "87");
    await user.type(screen.getByRole("textbox", { name: "M Bust (cm)" }), "91");
    await user.click(screen.getByRole("button", { name: "保存尺码表" }));

    expect(onSaveFinalDraft).toHaveBeenCalledWith(
      {
        size_attribute_list: [
          {
            attribute_id: 55,
            attribute_extra_value: "87.5",
            relate_sale_attribute_id: 87,
            relate_sale_attribute_value_id: 568,
          },
          {
            attribute_id: 20,
            attribute_extra_value: "87",
            relate_sale_attribute_id: 87,
            relate_sale_attribute_value_id: 568,
          },
          {
            attribute_id: 55,
            attribute_extra_value: "88.5",
            relate_sale_attribute_id: 87,
            relate_sale_attribute_value_id: 417,
          },
          {
            attribute_id: 20,
            attribute_extra_value: "91",
            relate_sale_attribute_id: 87,
            relate_sale_attribute_value_id: 417,
          },
        ],
      },
      "尺码表已保存，发布前会使用当前 SHEIN 尺码字段。",
    );
  });

  it("shows size chart payload blockers without marking sale attributes blocked", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: {
            ready: false,
            blocking_items: [
              {
                key: "variants",
                label: "发布载荷结构",
                message:
                  "SHEIN publish blocked: missing required size chart attributes: 内侧裤长 (cm), 胸围 (cm)",
              },
            ],
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            attributes: [{ name: "Material", value: "Polyester" }],
            sale_attributes: [{ name: "Size", value: "XXXXL" }],
            blocking_items: [
              {
                key: "variants",
                label: "发布载荷结构",
                message:
                  "SHEIN publish blocked: missing required size chart attributes: 内侧裤长 (cm), 胸围 (cm)",
              },
            ],
            images: [{ url: "https://example.com/main.jpg", main: true, final: true }],
          },
        }}
      />,
    );

    expect(screen.getByText("发布载荷")).toBeInTheDocument();
    expect(screen.getByText("可以先保存草稿")).toBeInTheDocument();
    expect(
      screen.getByText(
        "SHEIN publish blocked: missing required size chart attributes: 内侧裤长 (cm), 胸围 (cm)",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("已映射 1 个销售属性")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "发布到 SHEIN" })).toBeDisabled();
  });

  it("does not locally block single-variant final images when swatch and size map are not selected", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          final_review: {
            confirmed: false,
            category_id: 123,
            attributes: [{ name: "Material", value: "MDF" }],
            sale_attributes: [{ name: "Style Type", value: "White" }],
            skus: [{ supplier_sku: "SKU-1" }],
            images: [
              { url: "https://cdn.sdspod.com/out/main.jpg", role: "main", main: true, final: true },
              { url: "https://cdn.sdspod.com/out/gallery.jpg", role: "gallery", final: true },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("图片结构完整")).toBeInTheDocument();
    expect(screen.queryByText("图片资料需处理")).not.toBeInTheDocument();
    expect(screen.queryByText("缺色块来源图")).not.toBeInTheDocument();
    expect(screen.queryByText("缺尺寸图")).not.toBeInTheDocument();
  });

  it("allows first-time final review submit actions before confirmed is persisted", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          final_review: {
            confirmed: false,
            category_id: 123,
            attributes: [{ name: "Material", value: "Cotton" }],
            sale_attributes: [{ name: "Color", value: "Black" }],
            images: [
              { url: "https://example.com/main.jpg", main: true, final: true },
              { url: "https://example.com/swatch.jpg", swatch: true, final: true },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("资料已通过检查，请先确认当前结果无误。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "发布到 SHEIN" })).not.toBeDisabled();
    expect(screen.queryByRole("button", { name: "确认最终草稿" })).not.toBeInTheDocument();
  });

  it("treats final_review as an in-page confirmation step instead of a hard blocker", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: {
            ready: false,
            blocking_items: [
              {
                key: "final_review",
                label: "最终确认",
                message: "提交前必须在最终确认页核对图片、价格、属性和 SKU 后确认",
              },
            ],
          },
          final_review: {
            confirmed: false,
            category_id: 123,
            attributes: [{ name: "Material", value: "Cotton" }],
            sale_attributes: [{ name: "Color", value: "Black" }],
            images: [
              { url: "https://example.com/main.jpg", main: true, final: true },
              { url: "https://example.com/swatch.jpg", swatch: true, final: true },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("资料已通过检查，请先确认当前结果无误。")).toBeInTheDocument();
    expect(screen.getByText("等待最终确认")).toBeInTheDocument();
    expect(screen.queryByText("价格或库存信息不完整")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "发布到 SHEIN" })).not.toBeDisabled();
  });

  it("shows manual fallback attribute confirmation as done when final attributes are empty", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          editor_context: {
            attributes: {
              current: {
                status: "resolved",
                source: "manual_fallback_review",
                resolved_count: 18,
              },
            },
          },
          final_review: {
            confirmed: false,
            category_id: 123,
            sale_attributes: [{ name: "Color", value: "Black" }],
            images: [
              { url: "https://example.com/main.jpg", main: true, final: true },
              { url: "https://example.com/swatch.jpg", swatch: true, final: true },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("已按当前 SDS 属性确认 18 个普通属性")).toBeInTheDocument();
    expect(
      screen.queryByText("普通属性未展示已确认结果，建议检查必填属性。"),
    ).not.toBeInTheDocument();
  });

  it("enables save draft and publish after confirmed ready review", () => {
    const { container } = render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          final_review: {
            confirmed: true,
            category_id: 123,
            attributes: [{ name: "Material", value: "Cotton" }],
            sale_attributes: [{ name: "Color", value: "Black" }],
            images: [
              { url: "https://example.com/main.jpg", main: true, final: true },
              { url: "https://example.com/swatch.jpg", swatch: true, final: true },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("可以保存到 SHEIN 草稿箱，也可以正式发布。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "发布到 SHEIN" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).toHaveClass("w-full");
    expect(screen.getByRole("button", { name: "发布到 SHEIN" })).toHaveClass("w-full");
    expect(container.querySelector(".overflow-x-auto")).not.toBeNull();
  });

  it("submits publish with auto-saved final draft payload", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();

    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          final_review: {
            confirmed: true,
            category_id: 123,
            attributes: [{ name: "Material", value: "Cotton" }],
            sale_attributes: [{ name: "Color", value: "Black" }],
            images: [
              { url: "https://example.com/main.jpg", main: true, final: true },
              { url: "https://example.com/swatch.jpg", swatch: true, final: true },
            ],
            skus: [{ supplier_sku: "SKU-1" }],
          },
        }}
        onSubmit={onSubmit}
      />,
    );

    await user.click(screen.getByRole("button", { name: "发布到 SHEIN" }));

    expect(onSubmit).toHaveBeenCalledWith("publish", {
      confirmed: true,
      submit_mode: "publish",
      manual_price_overrides: {},
    });
  });

  it("submits save draft with auto-saved final draft payload", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();

    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          final_review: {
            confirmed: true,
            category_id: 123,
            attributes: [{ name: "Material", value: "Cotton" }],
            sale_attributes: [{ name: "Color", value: "Black" }],
            images: [
              { url: "https://example.com/main.jpg", main: true, final: true },
              { url: "https://example.com/swatch.jpg", swatch: true, final: true },
            ],
          },
        }}
        onSubmit={onSubmit}
      />,
    );

    await user.click(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" }));

    expect(onSubmit).toHaveBeenCalledWith("save_draft", {
      confirmed: true,
      submit_mode: "save_draft",
      manual_price_overrides: {},
    });
  });

  it("disables publish action after publish succeeds", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: { ready: true },
          submission_state: {
            last_action: "publish",
            last_status: "success",
            last_result: {
              success: true,
              spu_name: "h2605201253579421",
            },
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            attributes: [{ name: "Material", value: "Cotton" }],
            sale_attributes: [{ name: "Color", value: "Black" }],
            images: [
              { url: "https://example.com/main.jpg", main: true, final: true },
              { url: "https://example.com/swatch.jpg", swatch: true, final: true },
            ],
          },
        }}
      />,
    );

    expect(screen.getByRole("button", { name: "已发布到 SHEIN" })).toBeDisabled();
  });

  it("disables save draft while readiness is blocked", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: {
            ready: false,
            blocking_items: [
              { key: "final_images", label: "最终图片", message: "缺少尺寸图标记" },
            ],
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            images: [
              { url: "https://example.com/main.jpg", main: true, final: true },
              { url: "https://example.com/swatch.jpg", swatch: true, final: true },
            ],
          },
        }}
      />,
    );

    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).toBeDisabled();
    expect(screen.getByText("还差 1 个阻断项，修复后才能提交。")).toBeInTheDocument();
  });

  it("renders structured submit failure guidance in final review mode", () => {
    render(
      <SheinFinalReviewPanel
        shein={{
          submit_readiness: {
            ready: false,
            blocking_items: [
              { key: "images", label: "最终图片", message: "缺少色块来源图" },
            ],
          },
          final_review: {
            confirmed: true,
            category_id: 123,
            images: [{ url: "https://example.com/main.jpg", main: true, final: true }],
          },
        }}
        submitErrorMessage="SHEIN image upload unavailable: token missing"
      />,
    );

    expect(screen.getByText("提交失败")).toBeInTheDocument();
    expect(screen.getByText("发生了什么")).toBeInTheDocument();
    expect(screen.getByText("可能影响")).toBeInTheDocument();
    expect(screen.getByText("下一步怎么做")).toBeInTheDocument();
    expect(
      screen.getByText("本次不会把资料提交到 SHEIN，请先处理阻断项或上传问题后再重试。"),
    ).toBeInTheDocument();
  });
});
