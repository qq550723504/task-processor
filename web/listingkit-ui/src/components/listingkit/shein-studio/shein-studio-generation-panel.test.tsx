import type { ImgHTMLAttributes } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";
import type { SDSGroupedPromptHistoryEntry } from "@/lib/types/shein-studio";

vi.mock("@/components/listingkit/shein-studio/shein-created-tasks-list", () => ({
  SheinCreatedTasksList: () => <div>created tasks</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-saved-batches-panel", () => ({
  SheinSavedBatchesPanel: () => <div>saved batches</div>,
}));

vi.mock("@/lib/query/use-shein-store-selector", () => ({
  useSheinStoreSelector: () => ({
    enabledProfiles: [],
    profiles: { isError: false },
    routing: { isError: false },
    recommendedStoreId: "",
  }),
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
  promptHistory?: SDSGroupedPromptHistoryEntry[];
  styleCount?: string;
  subscriptionBlockedMessage?: string;
}) {
  return render(
    <SheinStudioGenerationPanel
      availableSdsImages={options?.availableSdsImages ?? []}
      artworkModel="nanobanana"
      createdTasks={[]}
      creatingError=""
      creatingMessage=""
      generationError=""
      groupedImageMode="shared_by_size"
      imageStrategy={options?.imageStrategy ?? "ai_generated"}
      isCreatingTasks={false}
      isGenerating={false}
      onCreateTasks={() => undefined}
      onDeleteBatch={() => undefined}
      onGenerate={() => undefined}
      onLoadBatch={() => undefined}
      onRestorePrompt={() => undefined}
      onSaveBatch={() => undefined}
      productImageCount="5"
      productImagePrompt=""
      productImagePrompts={[]}
      prompt=""
      promptHistory={options?.promptHistory ?? []}
      promptInputRef={{ current: null }}
      renderSizeImagesWithSds={true}
      saveMessage=""
      savedBatches={[]}
      selectedSdsImages={[]}
      selectedStyleCount={0}
      selectionReady={true}
      subscriptionBlockedMessage={options?.subscriptionBlockedMessage ?? ""}
      variationIntensity="medium"
      setArtworkModel={() => undefined}
      setGroupedImageMode={() => undefined}
      setImageStrategy={() => undefined}
      setProductImageCount={() => undefined}
      setProductImagePrompt={() => undefined}
      setProductImagePrompts={() => undefined}
      setPrompt={() => undefined}
      setRenderSizeImagesWithSds={() => undefined}
      setSelectedSdsImages={() => undefined}
      setSheinStoreId={() => undefined}
      setStyleCount={() => undefined}
      setVariationIntensity={() => undefined}
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
        groupedImageMode="shared_by_size"
        imageStrategy="ai_generated"
        isCreatingTasks={false}
        isGenerating={false}
        onCreateTasks={() => undefined}
        onDeleteBatch={() => undefined}
        onGenerate={() => undefined}
        onLoadBatch={() => undefined}
        onRestorePrompt={() => undefined}
        onSaveBatch={() => undefined}
        productImageCount="5"
        productImagePrompt=""
        productImagePrompts={[]}
        prompt=""
        promptHistory={[]}
        promptInputRef={{ current: null }}
        renderSizeImagesWithSds={true}
        saveMessage=""
        savedBatches={[]}
        selectedSdsImages={[]}
        selectedStyleCount={0}
        selectionReady={false}
        subscriptionBlockedMessage=""
        variationIntensity="medium"
        setArtworkModel={() => undefined}
        setGroupedImageMode={() => undefined}
        setImageStrategy={() => undefined}
        setProductImageCount={() => undefined}
        setProductImagePrompt={() => undefined}
        setProductImagePrompts={() => undefined}
        setPrompt={() => undefined}
        setRenderSizeImagesWithSds={() => undefined}
        setSelectedSdsImages={() => undefined}
        setSheinStoreId={() => undefined}
        setStyleCount={() => undefined}
        setVariationIntensity={() => undefined}
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

  it("renders prompt history entries and restores one back into the editor", () => {
    const restorePrompt = vi.fn();

    render(
      <SheinStudioGenerationPanel
        availableSdsImages={[]}
        artworkModel="nanobanana"
        createdTasks={[]}
        creatingError=""
        creatingMessage=""
        generationError=""
        groupedImageMode="shared_by_size"
        imageStrategy="ai_generated"
        isCreatingTasks={false}
        isGenerating={false}
        onCreateTasks={() => undefined}
        onDeleteBatch={() => undefined}
        onGenerate={() => undefined}
        onLoadBatch={() => undefined}
        onRestorePrompt={restorePrompt}
        onSaveBatch={() => undefined}
        productImageCount="5"
        productImagePrompt=""
        productImagePrompts={[]}
        prompt="current prompt"
        promptHistory={[
          {
            prompt: "prompt old",
            groupedImageMode: "shared_by_size",
            createdAt: "2026-05-26T01:00:00.000Z",
          },
        ]}
        promptInputRef={{ current: null }}
        renderSizeImagesWithSds={true}
        saveMessage=""
        savedBatches={[]}
        selectedSdsImages={[]}
        selectedStyleCount={0}
        selectionReady={true}
        subscriptionBlockedMessage=""
        variationIntensity="medium"
        setArtworkModel={() => undefined}
        setGroupedImageMode={() => undefined}
        setImageStrategy={() => undefined}
        setProductImageCount={() => undefined}
        setProductImagePrompt={() => undefined}
        setProductImagePrompts={() => undefined}
        setPrompt={() => undefined}
        setRenderSizeImagesWithSds={() => undefined}
        setSelectedSdsImages={() => undefined}
        setSheinStoreId={() => undefined}
        setStyleCount={() => undefined}
        setVariationIntensity={() => undefined}
        setTransparentBackground={() => undefined}
        sheinStoreId="869"
        styleCount="1"
        transparentBackground={false}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "prompt old" }));
    expect(restorePrompt).toHaveBeenCalledWith("prompt old");
  });

  it("blocks generation and task creation when the current tenant has no Studio entitlement", () => {
    renderPanel({
      subscriptionBlockedMessage:
        "当前租户未开通 Studio 模块。请在“当前租户订阅”里开通 Studio，或切换到已开通的租户后再生成款式图。",
    });

    expect(
      screen.getByText(
        "当前租户未开通 Studio 模块。请在“当前租户订阅”里开通 Studio，或切换到已开通的租户后再生成款式图。",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "生成款式图" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "生成 SHEIN 资料" })).toBeDisabled();
  });

  it("locks only artwork-generation fields while a style generation is in progress", () => {
    render(
      <SheinStudioGenerationPanel
        availableSdsImages={[]}
        artworkModel="nanobanana"
        createdTasks={[]}
        creatingError=""
        creatingMessage=""
        generationError=""
        groupedImageMode="shared_by_size"
        imageStrategy="ai_generated"
        isCreatingTasks={false}
        isGenerating={true}
        onCreateTasks={() => undefined}
        onDeleteBatch={() => undefined}
        onGenerate={() => undefined}
        onLoadBatch={() => undefined}
        onRestorePrompt={() => undefined}
        onSaveBatch={() => undefined}
        productImageCount="5"
        productImagePrompt="暖色背景"
        productImagePrompts={[]}
        prompt="美国国旗主题"
        promptHistory={[]}
        promptInputRef={{ current: null }}
        renderSizeImagesWithSds={true}
        saveMessage=""
        savedBatches={[]}
        selectedSdsImages={[]}
        selectedStyleCount={0}
        selectionReady={true}
        subscriptionBlockedMessage=""
        variationIntensity="medium"
        setArtworkModel={() => undefined}
        setGroupedImageMode={() => undefined}
        setImageStrategy={() => undefined}
        setProductImageCount={() => undefined}
        setProductImagePrompt={() => undefined}
        setProductImagePrompts={() => undefined}
        setPrompt={() => undefined}
        setRenderSizeImagesWithSds={() => undefined}
        setSelectedSdsImages={() => undefined}
        setSheinStoreId={() => undefined}
        setStyleCount={() => undefined}
        setVariationIntensity={() => undefined}
        setTransparentBackground={() => undefined}
        sheinStoreId="869"
        styleCount="2"
        transparentBackground={true}
      />,
    );

    expect(screen.getByRole("button", { name: "生成中..." })).toBeDisabled();
    expect(
      screen.getByPlaceholderText("例如：美国国旗主题，复古学院风，线条清晰，适合印刷。"),
    ).toBeDisabled();
    expect(screen.getByLabelText("款式数量")).toBeDisabled();
    expect(screen.getByDisplayValue("中变化")).toBeDisabled();
    expect(screen.getByDisplayValue("gpt-image-2")).toBeDisabled();
    expect(screen.getByRole("checkbox")).toBeDisabled();

    expect(screen.getByLabelText("商品图数量")).toBeEnabled();
    expect(
      screen.getByPlaceholderText("可选。会应用到每一张商品图，例如：背景保持暖色、简洁。"),
    ).toBeEnabled();
    expect(screen.getByLabelText("SHEIN 店铺")).toBeEnabled();
  });
});
