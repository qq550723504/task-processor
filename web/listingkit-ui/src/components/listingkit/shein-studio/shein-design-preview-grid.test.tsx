import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";
import { resolveGeneratedDesignFinalSrc, resolveGeneratedDesignOriginalSrc } from "@/lib/shein-studio/design-image";
import { toThumbnailPreviewUrl } from "@/lib/utils/imgproxy-url";

vi.mock("next/image", () => ({
  default: (props: React.ImgHTMLAttributes<HTMLImageElement>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img alt={props.alt ?? ""} {...props} />
  ),
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-lightbox", () => ({
  SheinDesignLightbox: () => null,
}));

describe("SheinDesignPreviewGrid", () => {
  it("shows original and removed previews plus the manual retry action after removal succeeds", () => {
    const onRetryBackgroundRemoval = vi.fn();

    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[
          {
            id: "design-1",
            imageUrl: "https://cdn.example.test/final.png",
            originalImageUrl: "https://cdn.example.test/original.png",
            backgroundRemovalStatus: "succeeded",
            transparentBackgroundMode: "removal",
          },
        ]}
        imageStrategy="hybrid"
        onCreateReviewTasks={vi.fn()}
        onRegenerate={vi.fn()}
        onRetryBackgroundRemoval={onRetryBackgroundRemoval}
        onToggle={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
      />,
    );

    expect(screen.getByRole("button", { name: "重新抠图" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "原图" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "抠图后" })).toBeInTheDocument();

    const originalPreview = screen.getByAltText("款式 1 原图预览");
    const finalPreview = screen.getByAltText("款式 1 抠图后预览");

    expect(originalPreview).toHaveAttribute(
      "src",
      toThumbnailPreviewUrl(
        resolveGeneratedDesignOriginalSrc({
          id: "design-1",
          imageUrl: "https://cdn.example.test/final.png",
          originalImageUrl: "https://cdn.example.test/original.png",
          backgroundRemovalStatus: "succeeded",
          transparentBackgroundMode: "removal",
        }),
        { width: 720, height: 720 },
      ),
    );
    expect(finalPreview).toHaveAttribute(
      "src",
      toThumbnailPreviewUrl(
        resolveGeneratedDesignFinalSrc({
          id: "design-1",
          imageUrl: "https://cdn.example.test/final.png",
          originalImageUrl: "https://cdn.example.test/original.png",
          backgroundRemovalStatus: "succeeded",
          transparentBackgroundMode: "removal",
        }),
        { width: 720, height: 720 },
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "重新抠图" }));
    expect(onRetryBackgroundRemoval).toHaveBeenCalledWith("design-1");
  });

  it("keeps the manual retry action and not-yet-removed status when no original image exists", () => {
    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[
          {
            id: "design-1",
            imageUrl: "https://cdn.example.test/final.png",
            backgroundRemovalStatus: "not_requested",
            transparentBackgroundMode: "removal",
          },
        ]}
        imageStrategy="hybrid"
        onCreateReviewTasks={vi.fn()}
        onRegenerate={vi.fn()}
        onRetryBackgroundRemoval={vi.fn()}
        onToggle={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
      />,
    );

    expect(screen.getByRole("button", { name: "重新抠图" })).toBeInTheDocument();
    expect(screen.getAllByText("尚未抠图")).not.toHaveLength(0);
  });

  it("keeps both previews visible in read-only mode while hiding the manual retry action", () => {
    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[
          {
            id: "design-1",
            imageUrl: "https://cdn.example.test/final.png",
            originalImageUrl: "https://cdn.example.test/original.png",
            backgroundRemovalStatus: "succeeded",
            transparentBackgroundMode: "removal",
          },
        ]}
        imageStrategy="hybrid"
        onCreateReviewTasks={vi.fn()}
        onRegenerate={vi.fn()}
        onRetryBackgroundRemoval={vi.fn()}
        onToggle={vi.fn()}
        productImageCount="3"
        readOnly
        renderSizeImagesWithSds
        selectedIds={["design-1"]}
      />,
    );

    expect(screen.getByAltText("款式 1 原图预览")).toBeInTheDocument();
    expect(screen.getByAltText("款式 1 抠图后预览")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重新抠图" })).not.toBeInTheDocument();
  });

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

  it("keeps design cards focused on the generated image", () => {
    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[
          {
            id: "design-1",
            imageUrl: "https://example.com/design-1.png",
            reviewNote: "图案过于复杂，不适合印刷。",
          },
        ]}
        imageStrategy="hybrid"
        onCreateReviewTasks={vi.fn()}
        onNoteChange={vi.fn()}
        onRegenerate={vi.fn()}
        onToggle={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
      />,
    );

    expect(screen.queryByRole("button", { name: "查看原图" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "查看效果图" })).not.toBeInTheDocument();
    expect(screen.queryByText(/当前卡片只展示原始款式图/)).not.toBeInTheDocument();
    expect(screen.queryByText("太复杂")).not.toBeInTheDocument();
    expect(screen.queryByText("线太细")).not.toBeInTheDocument();
    expect(
      screen.queryByPlaceholderText("可选：填写这个款式的问题或修改建议。"),
    ).not.toBeInTheDocument();
  });

  it("uses a wider gallery grid on large screens", () => {
    const { container } = render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[
          { id: "design-1", imageUrl: "https://example.com/design-1.png" },
          { id: "design-2", imageUrl: "https://example.com/design-2.png" },
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

    const galleryGrid = container.querySelector(".\\32xl\\:grid-cols-4");
    expect(galleryGrid).not.toBeNull();
  });
});
