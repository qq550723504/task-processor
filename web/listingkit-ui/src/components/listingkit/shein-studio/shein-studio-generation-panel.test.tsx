import type { ImgHTMLAttributes } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioBatchStatusGroups,
} from "@/lib/types/shein-studio";

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
  storeRequiredMessage?: string;
  subscriptionBlockedMessage?: string;
  statusGroups?: SheinStudioBatchStatusGroups;
}) {
  return render(
    <SheinStudioGenerationPanel
      availableSdsImages={options?.availableSdsImages ?? []}
      artworkModel="nanobanana"
      batchProductCount={4}
      batchStoreLabel={
        options?.storeRequiredMessage ? "未设置" : "SHEIN US 1 (shein-us-1 / NA / US)"
      }
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
      statusGroups={options?.statusGroups}
      storeRequiredMessage={options?.storeRequiredMessage ?? ""}
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
      setStyleCount={() => undefined}
      setVariationIntensity={() => undefined}
      setTransparentBackground={() => undefined}
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
    expect(screen.queryByLabelText("商品图数量")).not.toBeInTheDocument();
  });

  it("keeps product image count visible when the strategy still needs AI image generation", () => {
    renderPanel({
      imageStrategy: "hybrid",
    });

    expect(screen.getByLabelText("商品图数量")).toBeInTheDocument();
  });

  it("shows per-image product prompts only in AI image mode", () => {
    const { rerender } = renderPanel({
      imageStrategy: "ai_generated",
    });

    expect(screen.getByText("全局商品图提示词")).toBeInTheDocument();
    expect(screen.getByText("每张商品图提示词")).toBeInTheDocument();

    rerender(
      <SheinStudioGenerationPanel
        availableSdsImages={[]}
        artworkModel="nanobanana"
        createdTasks={[]}
        creatingError=""
        creatingMessage=""
        generationError=""
        groupedImageMode="shared_by_size"
        imageStrategy="sds_official"
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
        selectionReady={true}
        storeRequiredMessage=""
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
        setStyleCount={() => undefined}
        setVariationIntensity={() => undefined}
        setTransparentBackground={() => undefined}
        styleCount="1"
        transparentBackground={false}
      />,
    );

    expect(screen.queryByText("全局商品图提示词")).not.toBeInTheDocument();
    expect(screen.queryByText("每张商品图提示词")).not.toBeInTheDocument();
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
        storeRequiredMessage=""
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
        setStyleCount={() => undefined}
        setVariationIntensity={() => undefined}
        setTransparentBackground={() => undefined}
        styleCount="1"
        transparentBackground={false}
      />,
    );

    expect(screen.getAllByText("请先选择商品")).toHaveLength(2);
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
        storeRequiredMessage=""
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
        setStyleCount={() => undefined}
        setVariationIntensity={() => undefined}
        setTransparentBackground={() => undefined}
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

  it("blocks generation and task creation when the batch store is still missing", () => {
    renderPanel({
      storeRequiredMessage: "请先选择批次店铺，再生成款式图或创建 SHEIN 资料。",
    });

    expect(screen.getByText("批次店铺")).toBeInTheDocument();
    expect(screen.getByText("未设置")).toBeInTheDocument();
    expect(screen.getByText("需先设置批次店铺")).toBeInTheDocument();
    expect(screen.getByText("先回到上方选择批次店铺，再继续生成。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "生成款式图" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "生成 SHEIN 资料" })).toBeDisabled();
  });

  it("summarizes itemized batch status groups for mixed batch results", () => {
    renderPanel({
      statusGroups: {
        items: [
          { key: "submittable", label: "可提交", count: 2, ids: ["item-1", "item-2"] },
          { key: "needs_fix", label: "需修复", count: 1, ids: ["item-3"] },
          { key: "processing", label: "处理中", count: 1, ids: ["item-4"] },
          { key: "generation_failed", label: "生成失败", count: 1, ids: ["item-5"] },
          { key: "submission_failed", label: "提交失败", count: 1, ids: ["design-6"] },
          { key: "draft_saved", label: "已保存草稿", count: 3, ids: ["task-1", "task-2", "task-3"] },
          { key: "published", label: "已发布", count: 1, ids: ["task-4"] },
        ],
        byKey: {},
      },
    });

    expect(screen.getByText("批量状态分组")).toBeInTheDocument();
    expect(screen.getByText("可提交")).toBeInTheDocument();
    expect(screen.getByText("2 项")).toBeInTheDocument();
    expect(screen.getByText("需修复")).toBeInTheDocument();
    expect(screen.getAllByText("1 项")).toHaveLength(5);
    expect(screen.getByText("已保存草稿")).toBeInTheDocument();
    expect(screen.getByText("3 项")).toBeInTheDocument();
    expect(screen.getByText("已发布")).toBeInTheDocument();
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
        storeRequiredMessage=""
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
        setStyleCount={() => undefined}
        setVariationIntensity={() => undefined}
        setTransparentBackground={() => undefined}
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
  });

  it("uses mobile-first metrics and action groups", () => {
    renderPanel();

    const metricsGrid = screen
      .getByText("批次店铺")
      .closest("div")?.parentElement as HTMLDivElement | null;
    expect(metricsGrid).not.toBeNull();
    expect(metricsGrid?.className).not.toContain("sm:grid-cols-3");

    const actionGroup = screen
      .getByRole("button", { name: "生成 SHEIN 资料" })
      .parentElement as HTMLDivElement | null;
    expect(actionGroup).not.toBeNull();
    expect(actionGroup?.className).toContain("flex-col");
  });
});
