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
    expect(screen.getByText("主图、色块图或最终提交图片不完整，请先检查图片角色。")).toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "去确认属性" })[0]);
    expect(onSelectBlockingItem).toHaveBeenCalledWith(
      expect.objectContaining({ key: "attributes" }),
    );
  });

  it("shows image role summary and image repair action", async () => {
    const user = userEvent.setup();
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

    await user.click(screen.getAllByRole("button", { name: "去检查图片" })[0]);
    expect(onSelectBlockingItem).toHaveBeenCalledWith(
      expect.objectContaining({ key: "images" }),
    );
  });

  it("requires final draft confirmation before publish when ready", () => {
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

    expect(screen.getByText("资料已通过检查，请先确认最终草稿。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "发布到 SHEIN" })).toBeDisabled();
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

  it("requires publish confirmation before final submit", async () => {
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

    expect(screen.getByText("确认发布到 SHEIN")).toBeInTheDocument();
    expect(
      screen.getByText("这会把当前已确认资料正式提交到 SHEIN，请先核对类目、图片和 SKU。"),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "确认发布" }));
    expect(onSubmit).toHaveBeenCalledWith("publish");
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
