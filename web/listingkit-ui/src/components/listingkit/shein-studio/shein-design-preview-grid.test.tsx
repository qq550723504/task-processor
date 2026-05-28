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
  it("uses imgproxy thumbnails for oss-hosted design cards when configured", () => {
    process.env.NEXT_PUBLIC_LISTINGKIT_IMGPROXY_BASE_URL = "https://pod.shuomiai.com/img";

    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[
          {
            id: "design-1",
            imageUrl: "https://oss.shuomiai.com/listingkit-assets/20260529/design-1.png",
          },
        ]}
        imageStrategy="hybrid"
        onCreateReviewTasks={vi.fn()}
        onRegenerate={vi.fn()}
        onToggle={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
      />,
    );

    expect(screen.getByAltText("生成款式 1")).toHaveAttribute(
      "src",
      "https://pod.shuomiai.com/img/insecure/rs:fit:720:720/plain/s3://listingkit-assets/20260529/design-1.png@webp",
    );

    delete process.env.NEXT_PUBLIC_LISTINGKIT_IMGPROXY_BASE_URL;
  });

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

  it("shows SDS image usage instead of a configurable count in pure SDS mode", () => {
    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[{ id: "design-1", imageUrl: "https://example.com/design-1.png" }]}
        imageStrategy="sds_official"
        onBackToGenerate={vi.fn()}
        onCreateReviewTasks={vi.fn()}
        onRegenerate={vi.fn()}
        onToggle={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={["design-1"]}
      />,
    );

    expect(screen.getByText("商品图方式：SDS 官方渲染")).toBeInTheDocument();
    expect(screen.getByText("商品图数量：使用全部 SDS 图")).toBeInTheDocument();
    expect(screen.queryByText("商品图数量：3 张")).not.toBeInTheDocument();
  });
});
