import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";
import { resolveGeneratedDesignFinalSrc, resolveGeneratedDesignOriginalSrc } from "@/lib/shein-studio/design-image";
import { downloadStudioImage } from "@/lib/shein-studio/download-image";
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

vi.mock("@/lib/shein-studio/download-image", () => ({
  downloadStudioImage: vi.fn(),
}));

function createPngFile(name = "manual-cutout.png") {
  const pngBytes = new Uint8Array([
    0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
    0x00, 0x00, 0x00, 0x0d,
    0x49, 0x48, 0x44, 0x52,
    0x00, 0x00, 0x00, 0x01,
    0x00, 0x00, 0x00, 0x01,
    0x08, 0x06, 0x00, 0x00, 0x00,
    0x1f, 0x15, 0xc4, 0x89,
  ]);
  return new File([pngBytes], name, { type: "image/png" });
}

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

  it("shows download and manual upload actions for completed designs", () => {
    const { container } = render(
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
        onToggle={vi.fn()}
        onUploadManualBackgroundRemoval={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
      />,
    );

    expect(screen.getByRole("button", { name: "下载原图" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "上传手动抠图" })).toBeInTheDocument();
    expect(container.querySelector('input[type="file"]')).toHaveAttribute(
      "accept",
      "image/png,.png",
    );
  });

  it("downloads the original image with a stable original filename", async () => {
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
        onToggle={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "下载原图" }));

    await waitFor(() =>
      expect(downloadStudioImage).toHaveBeenCalledWith(
        "https://cdn.example.test/original.png",
        "studio-design-1-original.png",
      ),
    );
  });

  it("selecting a png uploads it with the design id and file", async () => {
    const onUploadManualBackgroundRemoval = vi.fn().mockResolvedValue(undefined);
    const pngFile = createPngFile();
    const { container } = render(
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
        onToggle={vi.fn()}
        onUploadManualBackgroundRemoval={onUploadManualBackgroundRemoval}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
      />,
    );

    const input = container.querySelector('input[type="file"]');
    expect(input).not.toBeNull();

    fireEvent.change(input!, { target: { files: [pngFile] } });

    await waitFor(() =>
      expect(onUploadManualBackgroundRemoval).toHaveBeenCalledWith(
        "design-1",
        pngFile,
      ),
    );
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

  it("renders both panes and the neutral not-requested state for ordinary cards", () => {
    const onRetryBackgroundRemoval = vi.fn();

    render(
      <SheinDesignPreviewGrid
        canRegenerate={false}
        createActionDisabledReason={undefined}
        designs={[
          {
            id: "design-ordinary",
            imageUrl: "https://cdn.example.test/ordinary.png",
            backgroundRemovalStatus: "not_requested",
            transparentBackgroundMode: "none",
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
    expect(screen.queryByRole("button", { name: "重新生成" })).not.toBeInTheDocument();
    expect(screen.getByText("原图")).toBeInTheDocument();
    expect(screen.getByText("抠图后")).toBeInTheDocument();
    expect(screen.getByAltText("款式 1 原图预览")).toBeInTheDocument();
    const notRequestedLabels = screen.getAllByText("尚未抠图");
    expect(notRequestedLabels).not.toHaveLength(0);
    expect(notRequestedLabels[0]).toHaveClass("bg-zinc-100", "text-zinc-600");
    expect(screen.queryByText("抠图完成")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重新抠图" }));
    expect(onRetryBackgroundRemoval).toHaveBeenCalledWith("design-ordinary");
  });

  it("disables the other operation while regenerating or removing the same design", () => {
    const props = {
      createActionDisabledReason: undefined,
      designs: [
        {
          id: "design-1",
          imageUrl: "https://cdn.example.test/design.png",
          backgroundRemovalStatus: "pending" as const,
        },
      ],
      imageStrategy: "hybrid" as const,
      onCreateReviewTasks: vi.fn(),
      onRegenerate: vi.fn(),
      onRetryBackgroundRemoval: vi.fn(),
      onToggle: vi.fn(),
      productImageCount: "3",
      renderSizeImagesWithSds: true,
      selectedIds: [] as string[],
    };
    const { rerender } = render(
      <SheinDesignPreviewGrid
        {...props}
        retryingBackgroundRemovalId="design-1"
      />,
    );

    expect(screen.getByRole("button", { name: "重新生成" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "抠图中..." })).toBeDisabled();

    rerender(
      <SheinDesignPreviewGrid
        {...props}
        regeneratingId="design-1"
      />,
    );

    expect(screen.getByRole("button", { name: "重新生成中..." })).toBeDisabled();
    expect(screen.getByRole("button", { name: "重新抠图" })).toBeDisabled();
  });

  it("hides manual upload while background removal is pending", () => {
    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[
          {
            id: "design-1",
            imageUrl: "https://cdn.example.test/design.png",
            backgroundRemovalStatus: "pending",
            transparentBackgroundMode: "removal",
          },
        ]}
        imageStrategy="hybrid"
        onCreateReviewTasks={vi.fn()}
        onRegenerate={vi.fn()}
        onToggle={vi.fn()}
        onUploadManualBackgroundRemoval={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
      />,
    );

    expect(screen.queryByRole("button", { name: "上传手动抠图" })).not.toBeInTheDocument();
  });

  it("disables only the uploading card's mutating controls while leaving other cards enabled", () => {
    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[
          {
            id: "design-1",
            imageUrl: "https://cdn.example.test/design-1-final.png",
            originalImageUrl: "https://cdn.example.test/design-1-original.png",
            backgroundRemovalStatus: "succeeded",
            transparentBackgroundMode: "removal",
          },
          {
            id: "design-2",
            imageUrl: "https://cdn.example.test/design-2-final.png",
            originalImageUrl: "https://cdn.example.test/design-2-original.png",
            backgroundRemovalStatus: "succeeded",
            transparentBackgroundMode: "removal",
          },
        ]}
        imageStrategy="hybrid"
        onCreateReviewTasks={vi.fn()}
        onRegenerate={vi.fn()}
        onRetryBackgroundRemoval={vi.fn()}
        onToggle={vi.fn()}
        onUploadManualBackgroundRemoval={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
        uploadingManualBackgroundRemovalIds={["design-1"]}
      />,
    );

    const approveButtons = screen.getAllByRole("button", { name: "批准" });
    const regenerateButtons = screen.getAllByRole("button", { name: "重新生成" });
    const retryButtons = screen.getAllByRole("button", { name: "重新抠图" });

    expect(approveButtons[0]).toBeDisabled();
    expect(regenerateButtons[0]).toBeDisabled();
    expect(retryButtons[0]).toBeDisabled();

    expect(approveButtons[1]).toBeEnabled();
    expect(regenerateButtons[1]).toBeEnabled();
    expect(retryButtons[1]).toBeEnabled();
  });

  it("ignores non-png uploads at the UI boundary and shows a local error", async () => {
    const onUploadManualBackgroundRemoval = vi.fn().mockResolvedValue(undefined);
    const { container } = render(
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
        onToggle={vi.fn()}
        onUploadManualBackgroundRemoval={onUploadManualBackgroundRemoval}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={[]}
      />,
    );

    const input = container.querySelector('input[type="file"]');
    expect(input).not.toBeNull();

    fireEvent.change(input!, {
      target: {
        files: [new File(["fake"], "fake.jpg", { type: "image/jpeg" })],
      },
    });

    expect(onUploadManualBackgroundRemoval).not.toHaveBeenCalled();
    expect(
      await screen.findByText("仅支持上传真实 PNG 图片。"),
    ).toBeInTheDocument();
  });

  it("blocks task creation while any background removal request is running", () => {
    const onCreateReviewTasks = vi.fn();

    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason="请等待手动抠图上传完成后再生成 SHEIN 资料。"
        designs={[
          { id: "design-1", imageUrl: "https://cdn.example.test/design-1.png" },
          { id: "design-2", imageUrl: "https://cdn.example.test/design-2.png" },
        ]}
        imageStrategy="hybrid"
        onCreateReviewTasks={onCreateReviewTasks}
        onRegenerate={vi.fn()}
        onRetryBackgroundRemoval={vi.fn()}
        onToggle={vi.fn()}
        productImageCount="3"
        renderSizeImagesWithSds
        selectedIds={["design-1"]}
      />,
    );

    const createButton = screen.getByRole("button", {
      name: "为已批准款式生成 SHEIN 资料",
    });
    expect(createButton).toBeDisabled();
    fireEvent.click(createButton);
    expect(onCreateReviewTasks).not.toHaveBeenCalled();
  });

  it("keeps download visible in read-only mode while hiding manual upload", () => {
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
    expect(screen.getByRole("button", { name: "下载原图" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "上传手动抠图" })).not.toBeInTheDocument();
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

    expect(screen.getByAltText("款式 1 原图预览")).toHaveAttribute(
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
