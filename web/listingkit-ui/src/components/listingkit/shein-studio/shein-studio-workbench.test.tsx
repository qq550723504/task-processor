import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinStudioWorkbench } from "@/components/listingkit/shein-studio/shein-studio-workbench";
import { saveSheinStudioGalleryHandoff } from "@/lib/shein-studio/gallery-handoff";

const useQuery = vi.fn();
const generateSheinStudioDesigns = vi.fn();
const resumeSheinStudioDesignGeneration = vi.fn();
const createSheinReviewTasks = vi.fn();
const getSDSBaselineReadiness = vi.fn();
const ensureSheinStudioSession = vi.fn();
const hydrateSDSVariantSelection = vi.fn();
const listSheinStudioBatches = vi.fn();
const loadSheinStudioDraft = vi.fn();
const saveSheinStudioBatch = vi.fn();
const saveSheinStudioDraftWithOptions = vi.fn();
const updateSheinStudioSession = vi.fn();
const deleteSheinStudioBatch = vi.fn();
let lastGenerationPanelProps: Record<string, unknown> | null = null;

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/sds",
  useSearchParams: () => new URLSearchParams("step=generate"),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (...args: unknown[]) => useQuery(...args),
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-selection-overview", () => ({
  SheinStudioSelectionOverview: () => <div>selection overview</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-progress-strip", () => ({
  SheinStudioProgressStrip: () => <div>progress strip</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-created-tasks-list", () => ({
  SheinCreatedTasksList: ({ tasks }: { tasks: Array<{ id: string }> }) => (
    <div>created tasks: {tasks.length}</div>
  ),
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-preview-grid", () => ({
  SheinDesignPreviewGrid: ({
    designs,
    onCreateReviewTasks,
  }: {
    designs: Array<{ id: string }>;
    onCreateReviewTasks?: () => void;
  }) => (
    <div>
      <div>review grid: {designs.length}</div>
      {onCreateReviewTasks ? (
        <button onClick={onCreateReviewTasks} type="button">
          create review tasks
        </button>
      ) : null}
    </div>
  ),
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-generation-panel", () => ({
  SheinStudioGenerationPanel: (props: {
    generationError?: string;
    onGenerate: () => void;
    prompt: string;
    subscriptionBlockedMessage?: string;
    setPrompt: (value: string) => void;
    selectedSdsImages?: Array<{
      color?: string;
      imageUrl: string;
      variantSku?: string;
    }>;
  }) => {
    lastGenerationPanelProps = props as Record<string, unknown>;
    return (
      <div>
        <label htmlFor="prompt">prompt</label>
        <input
          id="prompt"
          onChange={(event) => props.setPrompt(event.target.value)}
          value={props.prompt}
        />
        <button onClick={props.onGenerate} type="button">
          generate styles
        </button>
        <div>
          selected SDS images:{" "}
          {Array.isArray(props.selectedSdsImages) ? props.selectedSdsImages.length : 0}
        </div>
        {props.subscriptionBlockedMessage ? (
          <div>{props.subscriptionBlockedMessage}</div>
        ) : null}
        {props.generationError ? <div>{props.generationError}</div> : null}
      </div>
    );
  },
}));

vi.mock("@/lib/api/shein-studio", () => ({
  generateSheinStudioDesigns: (...args: unknown[]) => generateSheinStudioDesigns(...args),
  resumeSheinStudioDesignGeneration: (...args: unknown[]) =>
    resumeSheinStudioDesignGeneration(...args),
}));

vi.mock("@/lib/api/shein-studio-sessions", () => ({
  ensureSheinStudioSession: (...args: unknown[]) => ensureSheinStudioSession(...args),
  updateSheinStudioSession: (...args: unknown[]) => updateSheinStudioSession(...args),
}));

vi.mock("@/lib/shein-studio/create-review-tasks", async () => {
  const actual = await vi.importActual<typeof import("@/lib/shein-studio/create-review-tasks")>(
    "@/lib/shein-studio/create-review-tasks",
  );
  return {
    ...actual,
    createSheinReviewTasks: (...args: unknown[]) => createSheinReviewTasks(...args),
  };
});

vi.mock("@/lib/shein-studio/hydrate-sds-selection", () => ({
  hydrateSDSVariantSelection: (...args: unknown[]) => hydrateSDSVariantSelection(...args),
}));

vi.mock("@/lib/api/sds-baseline", () => ({
  getSDSBaselineReadiness: (...args: unknown[]) => getSDSBaselineReadiness(...args),
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  deleteSheinStudioBatch: (...args: unknown[]) => deleteSheinStudioBatch(...args),
  listSheinStudioBatches: (...args: unknown[]) => listSheinStudioBatches(...args),
  loadSheinStudioDraft: (...args: unknown[]) => loadSheinStudioDraft(...args),
  saveSheinStudioBatch: (...args: unknown[]) => saveSheinStudioBatch(...args),
  saveSheinStudioDraftWithOptions: (...args: unknown[]) =>
    saveSheinStudioDraftWithOptions(...args),
}));

vi.mock("@/lib/query/use-shein-store-selector", () => ({
  useSheinStoreSelector: () => ({
    enabledProfiles: [],
    profiles: { isError: false },
    routing: { isError: false },
    recommendedStoreId: "",
  }),
}));

const selection = {
  layerId: "layer-1",
  parentProductId: 1,
  printableHeight: 1000,
  printableWidth: 1000,
  productId: 1,
  productName: "tee",
  prototypeGroupId: 200,
  variantId: 100,
  variantLabel: "M / black",
};

describe("SheinStudioWorkbench", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
    lastGenerationPanelProps = null;
    useQuery.mockReturnValue({ data: undefined, error: null });
    generateSheinStudioDesigns.mockReset();
    resumeSheinStudioDesignGeneration.mockReset();
    createSheinReviewTasks.mockReset();
    getSDSBaselineReadiness.mockReset();
    ensureSheinStudioSession.mockResolvedValue({ session: { id: "session-1" } });
    getSDSBaselineReadiness.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "ready",
      reason: "",
    });
    hydrateSDSVariantSelection.mockResolvedValue(selection);
    listSheinStudioBatches.mockResolvedValue([]);
    loadSheinStudioDraft.mockResolvedValue(null);
    saveSheinStudioBatch.mockResolvedValue(null);
    saveSheinStudioDraftWithOptions.mockRejectedValue(new Error("timeout"));
    updateSheinStudioSession.mockResolvedValue({ session: { id: "session-1" } });
    deleteSheinStudioBatch.mockResolvedValue(undefined);
  });

  it("defaults to one SDS main image plus size references in hybrid and SDS modes", async () => {
    hydrateSDSVariantSelection.mockResolvedValue({
      ...selection,
      mockupImageUrls: ["https://example.com/main-mockup.jpg"],
      sizeReferenceImageUrls: ["https://example.com/size-reference.jpg"],
    });
    loadSheinStudioDraft.mockResolvedValue({
      prompt: "retro cherries",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "hybrid",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selectionVariantId: 100,
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-04-29T00:00:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByText("selected SDS images: 2")).toBeInTheDocument(),
    );
    expect(lastGenerationPanelProps?.selectedSdsImages).toEqual([
      {
        imageUrl: "https://example.com/main-mockup.jpg",
        color: undefined,
        variantSku: undefined,
      },
      {
        imageUrl: "https://example.com/size-reference.jpg",
        color: undefined,
        variantSku: undefined,
      },
    ]);
  });

  it("keeps generated designs visible when draft save fails", async () => {
    generateSheinStudioDesigns.mockResolvedValue({
      images: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
      ),
    ).toBeInTheDocument();
  });

  it("does not block generation when studio session sync fails", async () => {
    ensureSheinStudioSession.mockRejectedValue(new Error("ListingKit API request failed: 408"));
    generateSheinStudioDesigns.mockResolvedValue({
      images: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    expect(screen.queryByText("ListingKit API request failed: 408")).not.toBeInTheDocument();
  });

  it("guards against leaving the page while style generation is running", async () => {
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    generateSheinStudioDesigns.mockImplementation(
      () =>
        new Promise(() => {
          return undefined;
        }),
    );

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("正在生成款式图")).toBeInTheDocument(),
    );

    const anchor = document.createElement("a");
    anchor.href = "https://example.test/listing-kits";
    document.body.appendChild(anchor);

    const cancelled = !anchor.dispatchEvent(
      new MouseEvent("click", { bubbles: true, cancelable: true }),
    );

    expect(confirmSpy).toHaveBeenCalledWith(
      "当前正在生成款式图或创建 SHEIN 资料。现在离开会中断当前页面上的进度承接，确认还要离开吗？",
    );
    expect(cancelled).toBe(true);

    anchor.remove();
  });

  it("resumes an in-flight generation job after returning to the page", async () => {
    loadSheinStudioDraft.mockResolvedValue({
      prompt: "retro cherries",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      renderSizeImagesWithSds: true,
      selectionVariantId: 100,
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      generationError: "",
      generationJobId: "job-123",
      sessionStatus: "generating",
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    resumeSheinStudioDesignGeneration.mockResolvedValue({
      warnings: [],
      images: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(resumeSheinStudioDesignGeneration).toHaveBeenCalledWith("job-123"),
    );
    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
  });

  it("shows backend generation warnings when only part of the requested styles succeed", async () => {
    generateSheinStudioDesigns.mockResolvedValue({
      warnings: [
        "款式变体提示词生成失败，已回退为基础提示词重复生成。",
        "请求生成 3 款，实际仅成功 1 款，另外 2 款生成失败。 首个失败原因：upstream rate limited",
      ],
      images: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() => expect(loadSheinStudioDraft).toHaveBeenCalled());

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        /款式变体提示词生成失败，已回退为基础提示词重复生成。 请求生成 3 款，实际仅成功 1 款，另外 2 款生成失败。/,
      ),
    ).toBeInTheDocument();
  });

  it("keeps generated designs when parent step changes to review", async () => {
    generateSheinStudioDesigns.mockResolvedValue({
      images: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    });

    const rendered = render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );

    loadSheinStudioDraft.mockResolvedValue({
      prompt: "retro cherries",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      renderSizeImagesWithSds: true,
      selectionVariantId: 100,
      selection,
      designs: [],
      selectedIds: ["design-1"],
      createdTasks: [],
      updatedAt: "2026-04-29T00:00:00.000Z",
    });

    rendered.rerender(
      <SheinStudioWorkbench activeStep="review" selection={selection} />,
    );

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
  });

  it("shows explicit error and stays out of review when generation returns no images", async () => {
    generateSheinStudioDesigns.mockResolvedValue({
      images: [],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() => expect(loadSheinStudioDraft).toHaveBeenCalled());

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(
        screen.getByText(
          "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
        ),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText("review grid: 1")).not.toBeInTheDocument();
  });

  it("enters tasks view after task creation even when draft save fails", async () => {
    loadSheinStudioDraft.mockResolvedValue({
      prompt: "retro cherries",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      renderSizeImagesWithSds: true,
      selectionVariantId: 100,
      selection,
      designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
      selectedIds: ["design-1"],
      createdTasks: [],
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    createSheinReviewTasks.mockResolvedValue([
      { id: "task-1", title: "Task 1", designId: "design-1" },
    ]);

    render(<SheinStudioWorkbench activeStep="review" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "create review tasks" }));

    await waitFor(() =>
      expect(screen.getByText("created tasks: 1")).toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
      ),
    ).toBeInTheDocument();
  });

  it("imports a gallery handoff into review after SDS selection is available", async () => {
    saveSheinStudioGalleryHandoff({
      createdAt: new Date().toISOString(),
      height: 1000,
      id: "gallery-style-1",
      imageUrl: "https://example.com/gallery-style.png",
      prompt: "retro cherries",
      source: "studio_saved",
      title: "Gallery style",
      width: 1000,
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
  });

  it("blocks task creation when an imported gallery image ratio mismatches the SDS ratio", async () => {
    saveSheinStudioGalleryHandoff({
      createdAt: new Date().toISOString(),
      height: 1000,
      id: "gallery-style-1",
      imageUrl: "https://example.com/gallery-style.png",
      source: "studio_saved",
      title: "Gallery style",
      width: 1400,
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "create review tasks" }));

    expect(createSheinReviewTasks).not.toHaveBeenCalled();
    expect(
      screen.getByText("图库图片比例与 SDS 款式比例差异过大，请换图或更换 SDS 款式。"),
    ).toBeInTheDocument();
  });

  it("passes a Studio subscription gate message into the generation panel when the tenant is not entitled", async () => {
    useQuery.mockReturnValue({
      data: {
        entitlements: [
          {
            allowed: false,
            module: { code: "studio", name: "Studio" },
          },
        ],
      },
      error: null,
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(
        screen.getByText(
          "当前租户未开通 Studio 模块。请在“当前租户订阅”里开通 Studio，或切换到已开通的租户后再生成款式图。",
        ),
      ).toBeInTheDocument(),
    );
    expect(lastGenerationPanelProps?.subscriptionBlockedMessage).toBe(
      "当前租户未开通 Studio 模块。请在“当前租户订阅”里开通 Studio，或切换到已开通的租户后再生成款式图。",
    );
  });

  it("restores grouped selections from a saved draft even when they are not in recent variants", async () => {
    loadSheinStudioDraft.mockResolvedValue({
      prompt: "retro cherries",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "sds_official",
      selectedSdsImages: [],
      groupedSelections: [
        {
          selectionId: "1:200:101:layer-2:101",
          selection: {
            productId: 1,
            parentProductId: 1,
            variantId: 101,
            prototypeGroupId: 200,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          baselineStatus: "ready",
          baselineReason: "",
          sheinStoreId: "9",
          eligible: true,
        },
      ],
      renderSizeImagesWithSds: true,
      selectionVariantId: 100,
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-04-29T00:00:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByText(/已加入\s*1\s*款/)).toBeInTheDocument(),
    );
    expect(screen.getByText("hoodie")).toBeInTheDocument();
  });
});
