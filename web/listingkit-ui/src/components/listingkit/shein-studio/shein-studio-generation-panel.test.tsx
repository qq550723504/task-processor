import type { ImgHTMLAttributes } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioBatchItem,
  SheinStudioBatchStatusGroups,
  SheinStudioCreatedTask,
} from "@/lib/types/shein-studio";
import type { ComponentProps } from "react";

vi.mock(
  "@/components/listingkit/shein-studio/shein-created-tasks-list",
  () => ({
    SheinCreatedTasksList: () => <div>created tasks</div>,
  }),
);

vi.mock(
  "@/components/listingkit/shein-studio/shein-saved-batches-panel",
  () => ({
    SheinSavedBatchesPanel: () => <div>saved batches</div>,
  }),
);

vi.mock("@/lib/query/use-shein-store-selector", () => ({
  useSheinStoreSelector: () => ({
    enabledProfiles: [],
    profiles: { isError: false },
    routing: { isError: false },
    recommendedStoreId: "",
  }),
}));

vi.mock("next/image", () => ({
  default: (
    props: ImgHTMLAttributes<HTMLImageElement> & {
      fill?: boolean;
      src: string;
    },
  ) => {
    const { fill, alt, src, ...rest } = props;
    void fill;

    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img alt={alt ?? ""} src={src} {...rest} />
    );
  },
}));

function buildPanelProps(options?: {
  imageStrategy?: "ai_generated" | "sds_official" | "hybrid";
  availableSdsImages?: SheinStudioSelectableSDSImage[];
  generationNotice?: string;
  generateButtonLabel?: string;
  createdTasks?: SheinStudioCreatedTask[];
  isGenerating?: boolean;
  onRestorePrompt?: (value: string) => void;
  prompt?: string;
  failedBatchItems?: SheinStudioBatchItem[];
  isRetryingFailedItems?: boolean;
  onRetryFailedItem?: (itemId: string) => void;
  promptHistory?: SDSGroupedPromptHistoryEntry[];
  retryingFailedItemId?: string;
  reusedTasks?: SheinStudioCreatedTask[];
  styleCount?: string;
  storeRequiredMessage?: string;
  subscriptionBlockedMessage?: string;
  statusGroups?: SheinStudioBatchStatusGroups;
  selectionReady?: boolean;
  transparentBackground?: boolean;
  hotStyleReferenceBrief?: string;
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferencePrompt?: string;
}) {
  return {
    form: {
      availableSdsImages: options?.availableSdsImages ?? [],
      artworkModel: "nanobanana",
      groupedImageMode: "shared_by_size",
      hotStyleReferenceBrief: options?.hotStyleReferenceBrief ?? "",
      hotStyleReferenceImageUrls: options?.hotStyleReferenceImageUrls ?? [],
      hotStyleReferencePrompt: options?.hotStyleReferencePrompt ?? "",
      imageStrategy: options?.imageStrategy ?? "ai_generated",
      productImageCount: "5",
      productImagePrompt: options?.isGenerating ? "暖色背景" : "",
      productImagePrompts: [],
      prompt: options?.prompt ?? "",
      promptMode: "managed",
      promptHistory: options?.promptHistory ?? [],
      promptInputRef: { current: null },
      renderSizeImagesWithSds: true,
      selectedSdsImages: [],
      styleCount: options?.styleCount ?? "1",
      transparentBackground: options?.transparentBackground ?? false,
      variationIntensity: "medium",
    },
    status: {
      batchProductCount: 4,
      batchStoreLabel: options?.storeRequiredMessage
        ? "未设置"
        : "SHEIN US 1 (shein-us-1 / NA / US)",
      createdTasks: options?.createdTasks ?? [],
      creatingError: "",
      creatingMessage: "",
      failedBatchItems: options?.failedBatchItems ?? [],
      generationError: "",
      generationNotice: options?.generationNotice ?? "",
      isCreatingTasks: false,
      isGenerating: options?.isGenerating ?? false,
      isRetryingFailedItems: options?.isRetryingFailedItems ?? false,
      retryingFailedItemId: options?.retryingFailedItemId ?? "",
      reusedTasks: options?.reusedTasks ?? [],
      saveMessage: "",
      savedBatches: [],
      selectedStyleCount: 0,
      generateButtonLabel: options?.generateButtonLabel,
      selectionReady: options?.selectionReady ?? true,
      statusGroups: options?.statusGroups,
      storeRequiredMessage: options?.storeRequiredMessage ?? "",
      subscriptionBlockedMessage: options?.subscriptionBlockedMessage ?? "",
    },
    actions: {
      onCreateTasks: () => undefined,
      onDeleteBatch: () => undefined,
      onGenerate: () => undefined,
      onLoadBatch: () => undefined,
      analyzeReferenceStyle: async () => ({
        referenceStyleBrief: "",
        sanitizedPrompt: "",
        warnings: [],
      }),
      onRetryFailedItem: options?.onRetryFailedItem,
      onRestorePrompt: options?.onRestorePrompt ?? (() => undefined),
      onSaveBatch: () => undefined,
      setArtworkModel: () => undefined,
      setGroupedImageMode: () => undefined,
      setHotStyleReferenceBrief: () => undefined,
      setHotStyleReferenceImageUrls: () => undefined,
      setHotStyleReferencePrompt: () => undefined,
      setImageStrategy: () => undefined,
      setProductImageCount: () => undefined,
      setProductImagePrompt: () => undefined,
      setProductImagePrompts: () => undefined,
      setPrompt: () => undefined,
      setPromptMode: () => undefined,
      setRenderSizeImagesWithSds: () => undefined,
      setSelectedSdsImages: () => undefined,
      setStyleCount: () => undefined,
      setTransparentBackground: () => undefined,
      setVariationIntensity: () => undefined,
    },
  } satisfies ComponentProps<typeof SheinStudioGenerationPanel>;
}

function renderPanel(options?: Parameters<typeof buildPanelProps>[0]) {
  return render(<SheinStudioGenerationPanel {...buildPanelProps(options)} />);
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
        {...buildPanelProps({ imageStrategy: "sds_official" })}
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
        {...buildPanelProps({ selectionReady: false })}
      />,
    );

    expect(screen.getAllByText("请先选择商品")).toHaveLength(2);
    expect(
      screen.getByText(
        "当前还不能生成或创建任务，请先回到第 1 步完成 SDS 商品选择。",
      ),
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
        {...buildPanelProps({
          onRestorePrompt: restorePrompt,
          prompt: "current prompt",
          promptHistory: [
            {
              prompt: "prompt old",
              groupedImageMode: "shared_by_size",
              createdAt: "2026-05-26T01:00:00.000Z",
            },
          ],
        })}
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
    expect(
      screen.getByRole("button", { name: "生成 SHEIN 资料" }),
    ).toBeDisabled();
  });

  it("blocks generation and task creation when the batch store is still missing", () => {
    renderPanel({
      storeRequiredMessage: "请先选择批次店铺，再生成款式图或创建 SHEIN 资料。",
    });

    expect(screen.getByText("批次店铺")).toBeInTheDocument();
    expect(screen.getByText("未设置")).toBeInTheDocument();
    expect(screen.getByText("需先设置批次店铺")).toBeInTheDocument();
    expect(
      screen.getByText("先回到上方选择批次店铺，再继续生成。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "生成款式图" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "生成 SHEIN 资料" }),
    ).toBeDisabled();
  });

  it("summarizes itemized batch status groups for mixed batch results", () => {
    renderPanel({
      statusGroups: {
        items: [
          {
            key: "submittable",
            label: "可提交",
            count: 2,
            ids: ["item-1", "item-2"],
          },
          { key: "needs_fix", label: "需修复", count: 1, ids: ["item-3"] },
          { key: "processing", label: "处理中", count: 1, ids: ["item-4"] },
          {
            key: "generation_failed",
            label: "生成失败",
            count: 1,
            ids: ["item-5"],
          },
          {
            key: "submission_failed",
            label: "提交失败",
            count: 1,
            ids: ["design-6"],
          },
          {
            key: "draft_saved",
            label: "已保存草稿",
            count: 3,
            ids: ["task-1", "task-2", "task-3"],
          },
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

  it("surfaces a dedicated retry notice for failed batch items", () => {
    renderPanel({
      generateButtonLabel: "重试失败批次",
      generationNotice:
        "当前批次有 2 个失败项。点击“重试失败批次”只会重试失败部分，不会重复生成已成功内容。",
    });

    expect(
      screen.getByRole("button", { name: "重试失败批次" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "当前批次有 2 个失败项。点击“重试失败批次”只会重试失败部分，不会重复生成已成功内容。",
      ),
    ).toBeInTheDocument();
  });

  it("renders failed batch items and retries one item at a time", () => {
    const onRetryFailedItem = vi.fn();

    renderPanel({
      failedBatchItems: [
        {
          id: "item-1",
          batchId: "batch-1",
          targetGroupKey: "size:1000x1000",
          targetGroupLabel: "黑色 M",
          status: "failed",
          selectionCount: 2,
          lastError: "upstream timeout",
          createdAt: "2026-05-26T10:00:00.000Z",
          updatedAt: "2026-05-26T10:01:00.000Z",
        },
      ],
      onRetryFailedItem,
    });

    expect(screen.getByText("失败项")).toBeInTheDocument();
    expect(screen.getByText("黑色 M")).toBeInTheDocument();
    expect(screen.getByText("upstream timeout")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重试此项" }));
    expect(onRetryFailedItem).toHaveBeenCalledWith("item-1");
  });

  it("blocks item retry when a created ListingKit task already owns the batch item", () => {
    const onRetryFailedItem = vi.fn();

    renderPanel({
      createdTasks: [
        {
          id: "task-created-1",
          title: "黑色 M review task",
          designId: "design-1",
          itemId: "item-1",
          outcome: "created",
        },
      ],
      failedBatchItems: [
        {
          id: "item-1",
          batchId: "batch-1",
          targetGroupKey: "size:1000x1000",
          targetGroupLabel: "黑色 M",
          status: "failed",
          selectionCount: 2,
          lastError: "upstream timeout",
          createdAt: "2026-05-26T10:00:00.000Z",
          updatedAt: "2026-05-26T10:01:00.000Z",
        },
      ],
      onRetryFailedItem,
    });

    expect(
      screen.getByRole("button", { name: "已有 ListingKit 任务" }),
    ).toBeDisabled();
    expect(screen.getByText(/task-created-1/)).toHaveTextContent(
      "已创建 ListingKit 任务 task-created-1，请前往下方已有任务继续处理，避免重新生成覆盖已建任务设计。",
    );

    fireEvent.click(
      screen.getByRole("button", { name: "已有 ListingKit 任务" }),
    );
    expect(onRetryFailedItem).not.toHaveBeenCalled();
  });

  it("blocks item retry when a reused ListingKit task already owns the batch item", () => {
    renderPanel({
      reusedTasks: [
        {
          id: "task-reused-1",
          title: "复用 review task",
          designId: "design-2",
          itemId: "item-2",
          outcome: "reused",
        },
      ],
      failedBatchItems: [
        {
          id: "item-2",
          batchId: "batch-1",
          targetGroupKey: "size:1200x1200",
          targetGroupLabel: "白色 L",
          status: "failed",
          selectionCount: 1,
          lastError: "too many requests",
          createdAt: "2026-05-26T10:00:00.000Z",
          updatedAt: "2026-05-26T10:01:00.000Z",
        },
      ],
      onRetryFailedItem: vi.fn(),
    });

    expect(
      screen.getByRole("button", { name: "已有 ListingKit 任务" }),
    ).toBeDisabled();
    expect(screen.getByText(/task-reused-1/)).toHaveTextContent(
      "已复用 ListingKit 任务 task-reused-1，请前往下方已有任务继续处理，避免重新生成覆盖已建任务设计。",
    );
  });

  it("keeps item retry available when a failed batch item has no linked task", () => {
    const onRetryFailedItem = vi.fn();

    renderPanel({
      createdTasks: [
        {
          id: "task-created-1",
          title: "其他 item review task",
          designId: "design-1",
          itemId: "item-1",
          outcome: "created",
        },
      ],
      failedBatchItems: [
        {
          id: "item-2",
          batchId: "batch-1",
          targetGroupKey: "size:1200x1200",
          targetGroupLabel: "白色 L",
          status: "failed",
          selectionCount: 1,
          lastError: "too many requests",
          createdAt: "2026-05-26T10:00:00.000Z",
          updatedAt: "2026-05-26T10:01:00.000Z",
        },
      ],
      onRetryFailedItem,
    });

    fireEvent.click(screen.getByRole("button", { name: "重试此项" }));
    expect(onRetryFailedItem).toHaveBeenCalledWith("item-2");
  });

  it("shows item-level retry progress and locks sibling retries while one item is retrying", () => {
    renderPanel({
      failedBatchItems: [
        {
          id: "item-1",
          batchId: "batch-1",
          targetGroupKey: "size:1000x1000",
          targetGroupLabel: "黑色 M",
          status: "failed",
          selectionCount: 2,
          lastError: "upstream timeout",
          createdAt: "2026-05-26T10:00:00.000Z",
          updatedAt: "2026-05-26T10:01:00.000Z",
        },
        {
          id: "item-2",
          batchId: "batch-1",
          targetGroupKey: "size:1200x1200",
          targetGroupLabel: "白色 L",
          status: "failed",
          selectionCount: 1,
          lastError: "too many requests",
          createdAt: "2026-05-26T10:00:00.000Z",
          updatedAt: "2026-05-26T10:01:00.000Z",
        },
      ],
      onRetryFailedItem: () => undefined,
      retryingFailedItemId: "item-1",
    });

    expect(screen.getByRole("button", { name: "重试中..." })).toBeDisabled();
    expect(screen.getByRole("button", { name: "重试此项" })).toBeDisabled();
  });

  it("locks only artwork-generation fields while a style generation is in progress", () => {
    render(
      <SheinStudioGenerationPanel
        {...buildPanelProps({
          isGenerating: true,
          prompt: "美国国旗主题",
          styleCount: "2",
          transparentBackground: true,
        })}
      />,
    );

    expect(screen.getByRole("button", { name: "生成中..." })).toBeDisabled();
    expect(
      screen.getByPlaceholderText(
        "例如：美国国旗主题，复古学院风，线条清晰，适合印刷。",
      ),
    ).toBeDisabled();
    expect(screen.getByLabelText("款式数量")).toBeDisabled();
    expect(screen.getByDisplayValue("中变化")).toBeDisabled();
    expect(screen.getByDisplayValue("gpt-image-2")).toBeDisabled();
    expect(screen.getByRole("checkbox")).toBeDisabled();

    expect(screen.getByLabelText("商品图数量")).toBeEnabled();
    expect(
      screen.getByPlaceholderText(
        "可选。会应用到每一张商品图，例如：背景保持暖色、简洁。",
      ),
    ).toBeEnabled();
  });

  it("uses mobile-first metrics and action groups", () => {
    renderPanel();

    const metricsGrid = screen.getByText("批次店铺").closest("div")
      ?.parentElement as HTMLDivElement | null;
    expect(metricsGrid).not.toBeNull();
    expect(metricsGrid?.className).not.toContain("sm:grid-cols-3");

    const actionGroup = screen.getByRole("button", { name: "生成 SHEIN 资料" })
      .parentElement as HTMLDivElement | null;
    expect(actionGroup).not.toBeNull();
    expect(actionGroup?.className).toContain("flex-col");
  });

  it("uses a wide-screen two-column settings layout", () => {
    renderPanel();

    expect(screen.getByTestId("generation-settings-grid").className).toContain(
      "xl:grid-cols-[minmax(0,1.08fr)_minmax(24rem,0.92fr)]",
    );
  });

  it("renders hot style reference controls and applies extracted prompt", async () => {
    const user = userEvent.setup();
    const analyzeReferenceStyle = vi.fn().mockResolvedValue({
      referenceStyleBrief: "retro badge with cream and red palette",
      sanitizedPrompt:
        "Create an original retro badge with cream and red palette.",
      warnings: [],
    });
    const setHotStyleReferenceBrief = vi.fn();
    const setHotStyleReferencePrompt = vi.fn();
    const panelProps = buildPanelProps({ prompt: "retro cherries" });

    render(
      <SheinStudioGenerationPanel
        {...panelProps}
        actions={{
          ...panelProps.actions,
          analyzeReferenceStyle,
          setHotStyleReferenceBrief,
          setHotStyleReferencePrompt,
        }}
        form={{
          ...panelProps.form,
          hotStyleReferenceBrief: "",
          hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
          hotStyleReferencePrompt: "",
        }}
      />,
    );

    await user.click(screen.getByRole("button", { name: "提取热销款风格" }));

    expect(analyzeReferenceStyle).toHaveBeenCalledWith({
      referenceImageUrls: ["https://example.com/ref.png"],
      basePrompt: "retro cherries",
    });
    expect(setHotStyleReferenceBrief).toHaveBeenCalledWith(
      "retro badge with cream and red palette",
    );
    expect(setHotStyleReferencePrompt).toHaveBeenCalledWith(
      expect.stringContaining("original retro badge"),
    );
  });

  it("clears stale hot style prompt when extraction returns an empty sanitized prompt", async () => {
    const user = userEvent.setup();
    const analyzeReferenceStyle = vi.fn().mockResolvedValue({
      referenceStyleBrief: "no reusable visual pattern",
      sanitizedPrompt: "",
      warnings: [],
    });
    const setHotStyleReferenceBrief = vi.fn();
    const setHotStyleReferencePrompt = vi.fn();
    const panelProps = buildPanelProps({ prompt: "retro cherries" });

    render(
      <SheinStudioGenerationPanel
        {...panelProps}
        actions={{
          ...panelProps.actions,
          analyzeReferenceStyle,
          setHotStyleReferenceBrief,
          setHotStyleReferencePrompt,
        }}
        form={{
          ...panelProps.form,
          hotStyleReferenceBrief: "old brief",
          hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
          hotStyleReferencePrompt: "old extracted prompt",
        }}
      />,
    );

    await user.click(screen.getByRole("button", { name: "提取热销款风格" }));

    expect(setHotStyleReferenceBrief).toHaveBeenCalledWith(
      "no reusable visual pattern",
    );
    expect(setHotStyleReferencePrompt).toHaveBeenCalledWith("");
  });

  it("shows analyzer warnings and clears derived hot-style state after URL edits", async () => {
    const user = userEvent.setup();
    const analyzeReferenceStyle = vi.fn().mockResolvedValue({
      referenceStyleBrief: "retro badge with cream and red palette",
      sanitizedPrompt:
        "Create an original retro badge with cream and red palette.",
      warnings: [
        "最多分析 5 张参考图，已忽略多余图片。",
        "已移除品牌、Logo、原文案或过于接近原图的描述。",
      ],
    });
    const setHotStyleReferenceBrief = vi.fn();
    const setHotStyleReferenceImageUrls = vi.fn();
    const setHotStyleReferencePrompt = vi.fn();
    const panelProps = buildPanelProps({
      prompt: "retro cherries",
      hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
    });

    render(
      <SheinStudioGenerationPanel
        {...panelProps}
        actions={{
          ...panelProps.actions,
          analyzeReferenceStyle,
          setHotStyleReferenceBrief,
          setHotStyleReferenceImageUrls,
          setHotStyleReferencePrompt,
        }}
      />,
    );

    await user.click(screen.getByRole("button", { name: "提取热销款风格" }));

    expect(
      screen.getByText("最多分析 5 张参考图，已忽略多余图片。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("已移除品牌、Logo、原文案或过于接近原图的描述。"),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("每行一个热销款参考图 URL。"), {
      target: { value: "https://example.com/new-ref.png" },
    });

    expect(setHotStyleReferenceImageUrls).toHaveBeenLastCalledWith([
      "https://example.com/new-ref.png",
    ]);
    expect(setHotStyleReferenceBrief).toHaveBeenCalledWith("");
    expect(setHotStyleReferencePrompt).toHaveBeenCalledWith("");
    expect(
      screen.queryByText("最多分析 5 张参考图，已忽略多余图片。"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("已移除品牌、Logo、原文案或过于接近原图的描述。"),
    ).not.toBeInTheDocument();
  });
});
