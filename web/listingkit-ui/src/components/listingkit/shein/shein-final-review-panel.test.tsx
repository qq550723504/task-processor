import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SheinFinalReviewPanel } from "@/components/listingkit/shein/shein-final-review-panel";

describe("SheinFinalReviewPanel", () => {
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
    expect(screen.getByText("命中规则：国家规则")).toBeInTheDocument();
    expect(screen.getByText("按国家匹配")).toBeInTheDocument();
    expect(screen.getByText("Profile #17")).toBeInTheDocument();
    expect(screen.getByText(/固化时间：/)).toBeInTheDocument();
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
      />,
    );

    expect(screen.getByText("可以保存到 SHEIN 草稿箱，也可以正式发布。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "发布到 SHEIN" })).not.toBeDisabled();
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
          submission: {
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
