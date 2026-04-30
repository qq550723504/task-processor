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
  default: ({
    fill: _fill,
    alt,
    src,
    ...rest
  }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean; src: string }) => (
    <img alt={alt} src={src} {...rest} />
  ),
}));

function renderPanel(options?: {
  imageStrategy?: "ai_generated" | "sds_official" | "hybrid";
  availableSdsImages?: SheinStudioSelectableSDSImage[];
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
      styleCount="1"
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
});
