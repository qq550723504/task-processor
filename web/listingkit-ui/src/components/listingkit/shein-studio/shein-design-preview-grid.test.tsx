import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";

vi.mock("next/image", () => ({
  default: (props: React.ImgHTMLAttributes<HTMLImageElement>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img alt={props.alt ?? ""} {...props} />
  ),
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-lightbox", () => ({
  SheinDesignLightbox: () => null,
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-review-note", () => ({
  SheinDesignReviewNote: () => <div>review note</div>,
}));

describe("SheinDesignPreviewGrid", () => {
  it("shows cancel approval and continue generating actions for selected designs", () => {
    const onToggle = vi.fn();
    const onRegenerate = vi.fn();
    const onBackToGenerate = vi.fn();

    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[{ id: "design-1", imageUrl: "https://example.com/design-1.png" }]}
        imageStrategy="hybrid"
        onBackToGenerate={onBackToGenerate}
        onCreateReviewTasks={vi.fn()}
        onRegenerate={onRegenerate}
        onToggle={onToggle}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={["design-1"]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "取消批准" }));
    expect(onToggle).toHaveBeenCalledWith("design-1");

    expect(screen.getByText("当前商品图设置")).toBeInTheDocument();
    expect(screen.getByText("商品图方式：混合生成")).toBeInTheDocument();
    expect(screen.getByText("商品图数量：3 张")).toBeInTheDocument();
    expect(screen.getByText("尺寸图：使用 SDS 渲染")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "修改商品图设置" }));
    expect(onBackToGenerate).toHaveBeenCalledTimes(1);
  });
});
