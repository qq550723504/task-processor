import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinStudioWorkbench } from "@/components/listingkit/shein-studio/shein-studio-workbench";
import { saveLocalSheinStudioDraftSnapshot } from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import { saveSheinStudioGalleryHandoff } from "@/lib/shein-studio/gallery-handoff";
import { saveSDSGroupedCandidateHandoff } from "@/lib/utils/sds-grouped-candidate-handoff";

const useQuery = vi.fn();
const generateSheinStudioDesigns = vi.fn();
const resumeSheinStudioDesignGeneration = vi.fn();
const createSheinReviewTasks = vi.fn();
const getSDSBaselineReadiness = vi.fn();
const warmSDSBaselineForSelection = vi.fn();
const ensureSheinStudioSession = vi.fn();
const hydrateSDSVariantSelection = vi.fn();
const listSheinStudioBatches = vi.fn();
const getSheinStudioBatch = vi.fn();
const loadSheinStudioDraft = vi.fn();
const saveSheinStudioBatch = vi.fn();
const saveSheinStudioDraftWithOptions = vi.fn();
const setActiveSheinStudioBatchId = vi.fn();
const updateSheinStudioSession = vi.fn();
const deleteSheinStudioBatch = vi.fn();
const useSDSGroupedCandidates = vi.fn();
const push = vi.fn();
let lastGenerationPanelProps: Record<string, unknown> | null = null;

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/sds",
  useRouter: () => ({ push }),
  useSearchParams: () => new URLSearchParams("step=generate"),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (...args: unknown[]) => useQuery(...args),
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-selection-overview", () => ({
  SheinStudioSelectionOverview: ({
    selection,
  }: {
    selection?: { variantId?: number };
  }) => (
    <div>
      selection overview
      {selection?.variantId ? ` selection variant: ${selection.variantId}` : ""}
    </div>
  ),
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
    groupedImageMode?: string;
    generationError?: string;
    onGenerate: () => void;
    onRestorePrompt?: (value: string) => void;
    prompt: string;
    promptHistory?: Array<{ prompt: string; createdAt: string }>;
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
      <div id="shein-studio-generator">
        <label htmlFor="prompt">prompt</label>
        <input
          id="prompt"
          onChange={(event) => props.setPrompt(event.target.value)}
          value={props.prompt}
        />
        <button onClick={props.onGenerate} type="button">
          generate styles
        </button>
        {(props.promptHistory ?? []).map((entry) => (
          <button
            key={entry.createdAt}
            onClick={() => props.onRestorePrompt?.(entry.prompt)}
            type="button"
          >
            restore-{entry.prompt}
          </button>
        ))}
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
  warmSDSBaselineForSelection: (...args: unknown[]) => warmSDSBaselineForSelection(...args),
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  deleteSheinStudioBatch: (...args: unknown[]) => deleteSheinStudioBatch(...args),
  getSheinStudioBatch: (...args: unknown[]) => getSheinStudioBatch(...args),
  listSheinStudioBatches: (...args: unknown[]) => listSheinStudioBatches(...args),
  loadSheinStudioDraft: (...args: unknown[]) => loadSheinStudioDraft(...args),
  saveSheinStudioBatch: (...args: unknown[]) => saveSheinStudioBatch(...args),
  saveSheinStudioDraftWithOptions: (...args: unknown[]) =>
    saveSheinStudioDraftWithOptions(...args),
  setActiveSheinStudioBatchId: (...args: unknown[]) =>
    setActiveSheinStudioBatchId(...args),
}));

vi.mock("@/lib/query/use-shein-store-selector", () => ({
  useSheinStoreSelector: () => ({
    enabledProfiles: [],
    profiles: { isError: false },
    routing: { isError: false },
    recommendedStoreId: "",
  }),
}));

vi.mock("@/lib/query/use-sds-grouped-candidates", () => ({
  useSDSGroupedCandidates: () => useSDSGroupedCandidates(),
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

const groupedSelection = {
  selectionId: "1:200:101:layer-2:101",
  selection: {
    productId: 1,
    parentProductId: 1,
    variantId: 101,
    prototypeGroupId: 200,
    layerId: "layer-2",
    productName: "hoodie",
    variantLabel: "L / white",
    printableWidth: 1000,
    printableHeight: 1000,
  },
  baselineStatus: "ready" as const,
  baselineReason: "",
  sheinStoreId: "9",
  eligible: true,
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
    getSheinStudioBatch.mockResolvedValue(null);
    listSheinStudioBatches.mockResolvedValue([]);
    loadSheinStudioDraft.mockResolvedValue(null);
    warmSDSBaselineForSelection.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "ready",
      reason: "",
    });
    saveSheinStudioBatch.mockResolvedValue(null);
    saveSheinStudioDraftWithOptions.mockRejectedValue(new Error("timeout"));
    updateSheinStudioSession.mockResolvedValue({ session: { id: "session-1" } });
    deleteSheinStudioBatch.mockResolvedValue(undefined);
    useSDSGroupedCandidates.mockReturnValue([]);
    push.mockReset();
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

  it("loads saved groups on page entry without requiring reselecting the original product", async () => {
    saveLocalSheinStudioDraftSnapshot({
      prompt: "legacy top-level",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          primarySelection: selection,
          groupedSelections: [],
          styleCount: "1",
          sheinStoreId: "1",
          imageStrategy: "ai_generated",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          currentPrompt: "prompt a",
          promptHistory: [],
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "nanobanana",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T00:00:00.000Z",
        },
        {
          id: "group-2",
          name: "Group 2",
          primarySelection: {
            ...selection,
            layerId: "layer-3",
            productName: "hoodie",
            variantId: 102,
            variantLabel: "L / white",
          },
          groupedSelections: [groupedSelection],
          styleCount: "2",
          sheinStoreId: "9",
          imageStrategy: "sds_official",
          groupedImageMode: "per_product",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          currentPrompt: "prompt b",
          promptHistory: [],
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "nanobanana",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T01:00:00.000Z",
        },
      ],
      updatedAt: "2026-05-26T01:00:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    expect(await screen.findByText("Group 2")).toBeInTheDocument();
    expect(screen.getByDisplayValue("prompt b")).toBeInTheDocument();
  });

  it("shows recent batch cards before any explicit product reselection", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        groupedSelections: [
          {
            selectionId: "sel-1",
            selection: groupedSelection.selection,
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "869",
            eligible: true,
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    expect(await screen.findByText("最近批次")).toBeInTheDocument();
    expect(screen.getByText("Retro Cherries")).toBeInTheDocument();
    expect(screen.getByText("2 款商品")).toBeInTheDocument();
  });

  it("loads a batch by id when mounted from the dedicated batch route", async () => {
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:00:00.000Z",
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(getSheinStudioBatch).toHaveBeenCalledWith("batch-1"),
    );
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    expect(screen.getByText("selection overview selection variant: 100")).toBeInTheDocument();
    expect(screen.queryByText("最近批次")).not.toBeInTheDocument();
  });

  it("does not let a local draft override the dedicated batch selection", async () => {
    saveLocalSheinStudioDraftSnapshot({
      prompt: "legacy local draft",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:00:00.000Z",
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    expect(
      screen.getByText("selection overview selection variant: 100"),
    ).toBeInTheDocument();
    expect(
      screen.queryByDisplayValue("legacy local draft"),
    ).not.toBeInTheDocument();
  });

  it("shows the recent batch homepage before a selection is chosen", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="select" />);

    expect(await screen.findByText("最近批次")).toBeInTheDocument();
    expect(screen.getByText("Retro Cherries")).toBeInTheDocument();
    expect(screen.queryByText("selection overview")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "generate styles" })).not.toBeInTheDocument();
  });

  it("loads the selected batch into the editor when clicking a recent batch card", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: /Retro Cherries/ }));
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
  });

  it("routes a recent review-ready batch to the dedicated batch page", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
        selectedIds: ["design-1"],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: "去创建任务" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");
  });

  it("routes a recent batch with tasks to the dedicated batch page", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
        selectedIds: ["design-1"],
        createdTasks: [{ id: "task-1", title: "Task 1" }],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: "查看任务" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");
  });

  it("starts queue mode from homepage selection and loads the first batch", async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));

    expect(await screen.findByText("第 1 / 2 个批次")).toBeInTheDocument();
    expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument();
    expect(
      screen.getByText("已定位到生成区，可直接修改提示词或继续生成。"),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByLabelText("prompt")),
    );
    expect(scrollIntoView).toHaveBeenCalled();
  });

  it("moves to the next batch when clicking next", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));
    fireEvent.click(await screen.findByRole("button", { name: "下一批次" }));

    await waitFor(() =>
      expect(screen.getByDisplayValue("second prompt")).toBeInTheDocument(),
    );
    expect(screen.getByText("第 2 / 2 个批次")).toBeInTheDocument();
  });

  it("starts create-task queue mode at the review step for batches with designs", async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
        selectedIds: ["design-1"],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("button", { name: "批量去创建任务 1 个" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    expect(screen.getByText("第 1 / 1 个批次")).toBeInTheDocument();
    expect(
      screen.getByText("已定位到审核区，可直接创建任务或调整款式。"),
    ).toBeInTheDocument();
    expect(scrollIntoView).toHaveBeenCalled();
  });

  it("starts task-view queue mode from batches that already have created tasks", async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
        selectedIds: ["design-1"],
        createdTasks: [{ id: "task-1", title: "Task 1" }],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("button", { name: "批量查看任务 1 个" }));

    await waitFor(() =>
      expect(screen.getByText("created tasks: 1")).toBeInTheDocument(),
    );
    expect(screen.getByText("第 1 / 1 个批次")).toBeInTheDocument();
    expect(
      screen.getByText("已定位到任务区，可继续查看已创建的任务。"),
    ).toBeInTheDocument();
    expect(scrollIntoView).toHaveBeenCalled();
  });

  it("skips missing batch ids and continues to the next available batch", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 1 个" }));

    await waitFor(() =>
      expect(screen.getByDisplayValue("second prompt")).toBeInTheDocument(),
    );
  });

  it("clears queue mode when exiting", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));
    fireEvent.click(await screen.findByRole("button", { name: "退出批量处理" }));

    await waitFor(() =>
      expect(screen.queryByText("第 1 / 2 个批次")).not.toBeInTheDocument(),
    );
    expect(screen.getByText(/已停在第 1 \/ 2 个批次/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "继续本轮处理" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "清除这轮选择" })).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "select batch-1" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "select batch-2" })).toBeChecked();
  });

  it("resumes the queued homepage selection from the saved queue position", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));
    fireEvent.click(await screen.findByRole("button", { name: "退出批量处理" }));
    fireEvent.click(screen.getByRole("button", { name: "继续本轮处理" }));

    expect(await screen.findByText("第 1 / 2 个批次")).toBeInTheDocument();
    expect(screen.queryByText(/已停在第 1 \/ 2 个批次/)).not.toBeInTheDocument();
  });

  it("clears the queued homepage selection context on demand", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));
    fireEvent.click(await screen.findByRole("button", { name: "退出批量处理" }));
    fireEvent.click(screen.getByRole("button", { name: "清除这轮选择" }));

    await waitFor(() =>
      expect(screen.queryByText(/已停在第 1 \/ 2 个批次/)).not.toBeInTheDocument(),
    );
    expect(screen.getByRole("checkbox", { name: "select batch-1" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "select batch-2" })).not.toBeChecked();
  });

  it("keeps homepage selection context and shows a completion message after finishing the queue", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));
    fireEvent.click(await screen.findByRole("button", { name: "下一批次" }));
    fireEvent.click(await screen.findByRole("button", { name: "下一批次" }));

    await waitFor(() =>
      expect(screen.queryByText("第 2 / 2 个批次")).not.toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        "已完成这轮继续生成处理，共处理 2 个已保存批次。首页勾选已保留，可继续调整或再次发起批量处理。",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "select batch-1" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "select batch-2" })).toBeChecked();
  });

  it("appends the active prompt to the selected group's history when generating", async () => {
    loadSheinStudioDraft.mockResolvedValue({
      prompt: "prompt old",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selectionVariantId: 100,
      selection,
      groupedSelections: [],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          primarySelection: selection,
          groupedSelections: [],
          styleCount: "1",
          sheinStoreId: "1",
          imageStrategy: "ai_generated",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          currentPrompt: "prompt old",
          promptHistory: [],
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "nanobanana",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T00:00:00.000Z",
        },
      ],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    generateSheinStudioDesigns.mockImplementation(
      () =>
        new Promise(() => {
          return undefined;
        }),
    );

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() => expect(screen.getByDisplayValue("prompt old")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "new prompt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "restore-new prompt" })).toBeInTheDocument(),
    );
  });

  it("restores a historic group prompt into the current prompt field", async () => {
    saveLocalSheinStudioDraftSnapshot({
      prompt: "legacy top-level",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          primarySelection: selection,
          groupedSelections: [],
          styleCount: "1",
          sheinStoreId: "1",
          imageStrategy: "ai_generated",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          currentPrompt: "prompt a",
          promptHistory: [
            {
              prompt: "prompt old",
              groupedImageMode: "shared_by_size",
              createdAt: "2026-05-26T00:00:00.000Z",
            },
          ],
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "nanobanana",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T01:00:00.000Z",
        },
      ],
      updatedAt: "2026-05-26T01:00:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    expect(await screen.findByDisplayValue("prompt a")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "restore-prompt old" }));
    await waitFor(() =>
      expect(screen.getByDisplayValue("prompt old")).toBeInTheDocument(),
    );
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

  it("shows candidate-pool items even when there are no recent variants", async () => {
    useSDSGroupedCandidates.mockReturnValue([
      {
        productId: 1,
        parentProductId: 1,
        variantId: 101,
        prototypeGroupId: 200,
        layerId: "layer-2",
        productName: "hoodie",
        variantLabel: "L / white",
        printableWidth: 1000,
        printableHeight: 1000,
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByText("批量候选池")).toBeInTheDocument(),
    );
    expect(screen.getByText("hoodie")).toBeInTheDocument();
  });

  it("passes the grouped image mode into the generation panel", async () => {
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
      groupedImageMode: "per_product",
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

    await waitFor(() => expect(loadSheinStudioDraft).toHaveBeenCalled());
    expect(lastGenerationPanelProps?.groupedImageMode).toBe("per_product");
  });

  it("allows a ready candidate with a different printable size to join grouped creation", async () => {
    useSDSGroupedCandidates.mockReturnValue([
      {
        productId: 1,
        parentProductId: 1,
        variantId: 101,
        prototypeGroupId: 200,
        layerId: "layer-2",
        productName: "hoodie",
        variantLabel: "L / white",
        printableWidth: 1400,
        printableHeight: 1000,
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    const addButton = await screen.findByRole("button", { name: "加入分组" });
    expect(addButton).toBeEnabled();
    expect(
      screen.queryByText("印刷宽度与当前主商品不一致，先不要混在同一批创建。"),
    ).not.toBeInTheDocument();
  });

  it("shows grouped-candidate recovery guidance after returning from candidate pool", async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    saveSDSGroupedCandidateHandoff({
      action: "focus_generate",
      actionLabel: "去生成并预热",
      message:
        "这款候选商品还没有 baseline 缓存。先在当前工作台完成一次生成或预热，再回来加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(
        screen.getByText(
          "这款候选商品还没有 baseline 缓存。先在当前工作台完成一次生成或预热，再回来加入 grouped 批量上品。",
        ),
      ).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "去生成并预热" }));
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled());
  });

  it("warms baseline directly from grouped-candidate guidance", async () => {
    saveSDSGroupedCandidateHandoff({
      action: "warm_baseline",
      actionLabel: "一键预热 baseline",
      message:
        "这款候选商品还没有 baseline 缓存。先预热 baseline，再回来加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "一键预热 baseline" })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "一键预热 baseline" }));

    await waitFor(() => expect(warmSDSBaselineForSelection).toHaveBeenCalledWith(selection));
    await waitFor(() =>
      expect(
        screen.getByText("这款 SDS 商品的 baseline 已预热完成，现在可以继续加入 grouped 批量上品。"),
      ).toBeInTheDocument(),
    );
  });
});
