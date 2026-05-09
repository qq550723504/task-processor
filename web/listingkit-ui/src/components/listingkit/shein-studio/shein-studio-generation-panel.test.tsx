import type { ImgHTMLAttributes } from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";

vi.mock("@/components/listingkit/shein-studio/shein-created-tasks-list", () => ({
  SheinCreatedTasksList: () => <div>created tasks</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-saved-batches-panel", () => ({
  SheinSavedBatchesPanel: () => <div>saved batches</div>,
}));

vi.mock("next/image", () => ({
  default: (props: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean; src: string }) => {
    const { fill, alt, src, ...rest } = props;
    void fill;

    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img alt={alt ?? ""} src={src} {...rest} />
    );
  },
}));

function renderPanel(options?: {
  imageStrategy?: "ai_generated" | "sds_official" | "hybrid";
  availableSdsImages?: SheinStudioSelectableSDSImage[];
  styleCount?: string;
}) {
  return render(
    <SheinStudioGenerationPanel
      availableSdsImages={options?.availableSdsImages ?? []}
      artworkModel="nanobanana"
      createdTasks={[]}
      creatingError=""
      creatingMessage=""
      generationError=""
      imageStrategy={options?.imageStrategy ?? "ai_generated"}
      isCreatingTasks={false}
      isGenerating={false}
      onCreateTasks={() => undefined}
      onDeleteBatch={() => undefined}
      onGenerate={() => undefined}
      onLoadBatch={() => undefined}
      onSaveBatch={() => undefined}
      productImageCount="5"
      productImagePrompt=""
      productImagePrompts={[]}
      prompt=""
      promptInputRef={{ current: null }}
      renderSizeImagesWithSds={true}
      saveMessage=""
      savedBatches={[]}
      selectedSdsImages={[]}
      selectedStyleCount={0}
      selectionReady={true}
      setArtworkModel={() => undefined}
      setImageStrategy={() => undefined}
      setProductImageCount={() => undefined}
      setProductImagePrompt={() => undefined}
      setProductImagePrompts={() => undefined}
      setPrompt={() => undefined}
      setRenderSizeImagesWithSds={() => undefined}
      setSelectedSdsImages={() => undefined}
      setSheinStoreId={() => undefined}
      setStyleCount={() => undefined}
      setTransparentBackground={() => undefined}
      sheinStoreId="869"
      styleCount={options?.styleCount ?? "1"}
      transparentBackground={false}
    />,
  );
}

describe("SheinStudioGenerationPanel", () => {
  const sizeReferenceImage: SheinStudioSelectableSDSImage = {
    imageUrl: "https://example.com/size.jpg",
    kind: "size_reference",
    label: "当前款式尺寸图 · SDS 图 1",
  };

  it("shows the SDS size-image toggle when hybrid or AI mode can actually use it", () => {
    renderPanel({
      imageStrategy: "hybrid",
      availableSdsImages: [sizeReferenceImage],
    });

    expect(screen.getByText("尺寸图也使用 SDS 渲染")).toBeInTheDocument();
  });

  it("hides the SDS size-image toggle in pure SDS mode", () => {
    renderPanel({
      imageStrategy: "sds_official",
      availableSdsImages: [sizeReferenceImage],
    });

    expect(screen.queryByText("尺寸图也使用 SDS 渲染")).not.toBeInTheDocument();
  });

  it("hides the SDS size-image toggle when no SDS size references exist", () => {
    renderPanel({
      imageStrategy: "hybrid",
      availableSdsImages: [],
    });

    expect(screen.queryByText("尺寸图也使用 SDS 渲染")).not.toBeInTheDocument();
  });

  it("shows a single next-step callout when product selection is still missing", () => {
    render(
      <SheinStudioGenerationPanel
        availableSdsImages={[]}
        artworkModel="nanobanana"
        createdTasks={[]}
        creatingError=""
        creatingMessage=""
        generationError=""
        imageStrategy="ai_generated"
        isCreatingTasks={false}
        isGenerating={false}
        onCreateTasks={() => undefined}
        onDeleteBatch={() => undefined}
        onGenerate={() => undefined}
        onLoadBatch={() => undefined}
        onSaveBatch={() => undefined}
        productImageCount="5"
        productImagePrompt=""
        productImagePrompts={[]}
        prompt=""
        promptInputRef={{ current: null }}
        renderSizeImagesWithSds={true}
        saveMessage=""
        savedBatches={[]}
        selectedSdsImages={[]}
        selectedStyleCount={0}
        selectionReady={false}
        setArtworkModel={() => undefined}
        setImageStrategy={() => undefined}
        setProductImageCount={() => undefined}
        setProductImagePrompt={() => undefined}
        setProductImagePrompts={() => undefined}
        setPrompt={() => undefined}
        setRenderSizeImagesWithSds={() => undefined}
        setSelectedSdsImages={() => undefined}
        setSheinStoreId={() => undefined}
        setStyleCount={() => undefined}
        setTransparentBackground={() => undefined}
        sheinStoreId="869"
        styleCount="1"
        transparentBackground={false}
      />,
    );

    expect(screen.getByText("请先选择商品")).toBeInTheDocument();
    expect(
      screen.getByText("当前还不能生成或创建任务，请先回到第 1 步完成 SDS 商品选择。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "先选择商品" })).toBeDisabled();
  });

  it("hides variation intensity when generating one style", () => {
    renderPanel({ styleCount: "1" });

    expect(screen.queryByText("变化强度")).not.toBeInTheDocument();
  });

  it("shows variation intensity when generating multiple styles", () => {
    renderPanel({ styleCount: "2" });

    expect(screen.getByText("变化强度")).toBeInTheDocument();
  });
});
