import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  resetDedicatedBatchPromptOverrides,
  SheinStudioWorkbench,
} from "@/components/listingkit/shein-studio/shein-studio-workbench";
import {
  loadLocalSheinStudioDraftSnapshotDetail,
  saveLocalSheinStudioDraftSnapshot,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import { saveSheinStudioGalleryHandoff } from "@/lib/shein-studio/gallery-handoff";
import { saveSDSGroupedCandidateHandoff } from "@/lib/utils/sds-grouped-candidate-handoff";

const useQuery = vi.fn();
const generateSheinStudioDesigns = vi.fn();
const resumeSheinStudioDesignGeneration = vi.fn();
const createSheinReviewTasks = vi.fn();
const getSDSBaselineReadiness = vi.fn();
const warmSDSBaselineForSelection = vi.fn();
const hydrateSDSVariantSelection = vi.fn();
const listSheinStudioBatches = vi.fn();
const getSheinStudioBatch = vi.fn();
const getSheinStudioHydratedBatch = vi.fn();
const loadSheinStudioDraft = vi.fn();
const saveSheinStudioBatch = vi.fn();
const saveSheinStudioDraftWithOptions = vi.fn();
const setActiveSheinStudioBatchId = vi.fn();
const generateSheinStudioBatch = vi.fn();
const retrySheinStudioBatchItems = vi.fn();
const startSheinStudioBatchRun = vi.fn();
const deleteSheinStudioBatch = vi.fn();
const approveSheinStudioBatchDesigns = vi.fn();
const createSheinStudioBatchTasks = vi.fn();
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

vi.mock("@/components/listingkit/shein-studio/shein-studio-progress-strip", () => ({
  SheinStudioProgressStrip: () => <div>progress strip</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-batch-run-progress", () => ({
  SheinStudioBatchRunProgress: ({
    onBack,
    runId,
  }: {
    onBack: () => void;
    runId: string;
  }) => (
    <div>
      <div>batch run progress: {runId}</div>
      <button onClick={onBack} type="button">
        back from batch run
      </button>
    </div>
  ),
}));

vi.mock("@/components/listingkit/shein-studio/shein-created-tasks-list", () => ({
  SheinCreatedTasksList: ({ tasks }: { tasks: Array<{ id: string }> }) => (
    <div>created tasks: {tasks.length}</div>
  ),
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-preview-grid", () => ({
  SheinDesignPreviewGrid: ({
    designs,
    selectedIds,
    onToggle,
    onNoteChange,
    onCreateReviewTasks,
  }: {
    designs: Array<{ id: string }>;
    selectedIds?: string[];
    onToggle?: (designId: string) => void;
    onNoteChange?: (designId: string, note: string) => void;
    onCreateReviewTasks?: () => void;
  }) => (
    <div>
      <div>review grid: {designs.length}</div>
      <div>approved styles: {Array.isArray(selectedIds) ? selectedIds.length : 0}</div>
      {designs.map((design) => (
        <div key={design.id}>
          {onToggle ? (
            <button onClick={() => onToggle(design.id)} type="button">
              toggle-{design.id}
            </button>
          ) : null}
          {onNoteChange ? (
            <button
              onClick={() => onNoteChange(design.id, `note-${design.id}`)}
              type="button"
            >
              note-{design.id}
            </button>
          ) : null}
        </div>
      ))}
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
    failedBatchItems?: Array<{
      id: string;
      lastError?: string;
      targetGroupLabel?: string;
      targetGroupKey: string;
    }>;
    generateButtonLabel?: string;
    generationNotice?: string;
    groupedImageMode?: string;
    generationError?: string;
    isGenerating?: boolean;
    isRetryingFailedItems?: boolean;
    onCreateTasks?: () => void;
    onGenerate: () => void;
    onRetryFailedItem?: (itemId: string) => void;
    onRestorePrompt?: (value: string) => void;
    prompt: string;
    promptHistory?: Array<{ prompt: string; createdAt: string }>;
    retryingFailedItemId?: string;
    showSavedBatches?: boolean;
    subscriptionBlockedMessage?: string;
    storeRequiredMessage?: string;
    setPrompt: (value: string) => void;
    selectedSdsImages?: Array<{
      color?: string;
      imageUrl: string;
      variantSku?: string;
    }>;
  }) => {
    lastGenerationPanelProps = props as Record<string, unknown>;
    const actionLabel =
      props.generateButtonLabel === "重试失败批次"
        ? "重试失败批次"
        : "generate styles";
    return (
      <div id="shein-studio-generator">
        <label htmlFor="prompt">prompt</label>
        <input
          id="prompt"
          onChange={(event) => props.setPrompt(event.target.value)}
          value={props.prompt}
        />
        <button disabled={props.isGenerating} onClick={props.onGenerate} type="button">
          {actionLabel}
        </button>
        {props.onCreateTasks ? (
          <button onClick={props.onCreateTasks} type="button">
            create tasks from panel
          </button>
        ) : null}
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
        <div>saved batches visible: {props.showSavedBatches === false ? "no" : "yes"}</div>
        {props.subscriptionBlockedMessage ? (
          <div>{props.subscriptionBlockedMessage}</div>
        ) : null}
        {props.storeRequiredMessage ? <div>{props.storeRequiredMessage}</div> : null}
        {props.generationNotice ? <div>{props.generationNotice}</div> : null}
        {props.generationError ? <div>{props.generationError}</div> : null}
        {(props.failedBatchItems ?? []).map((item) => (
          <div key={item.id}>
            <span>{item.targetGroupLabel ?? item.targetGroupKey}</span>
            {item.lastError ? <span>{item.lastError}</span> : null}
            {props.onRetryFailedItem ? (
              <button
                disabled={Boolean(props.retryingFailedItemId)}
                onClick={() => props.onRetryFailedItem?.(item.id)}
                type="button"
              >
                retry-item-{item.id}
              </button>
            ) : null}
          </div>
        ))}
        {props.retryingFailedItemId ? <div>retrying-item: {props.retryingFailedItemId}</div> : null}
      </div>
    );
  },
}));

vi.mock("@/lib/api/shein-studio", () => ({
  generateSheinStudioDesigns: (...args: unknown[]) => generateSheinStudioDesigns(...args),
  resumeSheinStudioDesignGeneration: (...args: unknown[]) =>
    resumeSheinStudioDesignGeneration(...args),
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

vi.mock("@/lib/api/shein-studio-batches", () => ({
  approveSheinStudioBatchDesigns: (...args: unknown[]) =>
    approveSheinStudioBatchDesigns(...args),
  generateSheinStudioBatch: (...args: unknown[]) =>
    generateSheinStudioBatch(...args),
  retrySheinStudioBatchItems: (...args: unknown[]) =>
    retrySheinStudioBatchItems(...args),
  createSheinStudioBatchTasks: (...args: unknown[]) =>
    createSheinStudioBatchTasks(...args),
}));

vi.mock("@/lib/api/shein-studio-batch-runs", () => ({
  startSheinStudioBatchRun: (...args: unknown[]) => startSheinStudioBatchRun(...args),
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  deleteSheinStudioBatch: (...args: unknown[]) => deleteSheinStudioBatch(...args),
  getSheinStudioBatch: (...args: unknown[]) => getSheinStudioBatch(...args),
  getSheinStudioHydratedBatch: (...args: unknown[]) =>
    getSheinStudioHydratedBatch(...args),
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

function buildHydratedBatch(
  savedBatchOverrides: Record<string, unknown> = {},
  detailOverrides: Record<string, unknown> = {},
) {
  return {
    savedBatch: {
      id: "batch-1",
      name: "批次1",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:00:00.000Z",
      ...savedBatchOverrides,
    },
    detail: {
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      items: [],
      ...detailOverrides,
    },
  };
}

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, resolve, reject };
}

describe("SheinStudioWorkbench", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
    resetDedicatedBatchPromptOverrides();
    lastGenerationPanelProps = null;
    useQuery.mockReturnValue({ data: undefined, error: null });
    generateSheinStudioDesigns.mockReset();
    resumeSheinStudioDesignGeneration.mockReset();
    createSheinReviewTasks.mockReset();
    getSDSBaselineReadiness.mockReset();
    getSDSBaselineReadiness.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "ready",
      reason: "",
    });
    hydrateSDSVariantSelection.mockResolvedValue(selection);
    getSheinStudioBatch.mockReset();
    getSheinStudioBatch.mockResolvedValue(null);
    getSheinStudioHydratedBatch.mockReset();
    getSheinStudioHydratedBatch.mockResolvedValue(null);
    listSheinStudioBatches.mockResolvedValue([]);
    loadSheinStudioDraft.mockResolvedValue(null);
    warmSDSBaselineForSelection.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "ready",
      reason: "",
    });
    saveSheinStudioBatch.mockReset();
    saveSheinStudioBatch.mockResolvedValue(null);
    saveSheinStudioDraftWithOptions.mockReset();
    saveSheinStudioDraftWithOptions.mockRejectedValue(new Error("timeout"));
    generateSheinStudioBatch.mockReset();
    retrySheinStudioBatchItems.mockReset();
    startSheinStudioBatchRun.mockReset();
    deleteSheinStudioBatch.mockResolvedValue(undefined);
    approveSheinStudioBatchDesigns.mockReset();
    createSheinStudioBatchTasks.mockReset();
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
    getSheinStudioHydratedBatch.mockResolvedValue({
      savedBatch: {
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
      detail: {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        items: [],
      },
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(getSheinStudioHydratedBatch).toHaveBeenCalledWith("batch-1"),
    );
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    expect(screen.getByText("入口商品")).toBeInTheDocument();
    expect(screen.queryByText("入口商品状态")).not.toBeInTheDocument();
    expect(screen.getByLabelText("批次店铺")).toBeInTheDocument();
    expect(screen.getByText("saved batches visible: no")).toBeInTheDocument();
    expect(screen.queryByText("最近批次")).not.toBeInTheDocument();
  });

  it("shows itemized dedicated-batch designs in review even when the saved batch shell is empty", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue({
      savedBatch: {
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
      detail: {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        items: [
          {
            item: {
              id: "item-1",
              batchId: "batch-1",
              targetGroupKey: "size:1000x1000",
              status: "review_ready",
              selectionCount: 1,
              createdAt: "2026-05-26T09:59:00.000Z",
              updatedAt: "2026-05-26T10:00:00.000Z",
            },
            designs: [
              {
                id: "design-1",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-1",
                targetGroupKey: "size:1000x1000",
                imageUrl: "https://example.com/design-1.png",
                reviewStatus: "approved",
                createdAt: "2026-05-26T09:59:30.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
              {
                id: "design-2",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-2",
                targetGroupKey: "size:1000x1000",
                imageUrl: "https://example.com/design-2.png",
                reviewStatus: "unreviewed",
                createdAt: "2026-05-26T09:59:40.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
            ],
          },
        ],
      },
    });

    render(<SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />);

    await waitFor(() =>
      expect(screen.getByText("review grid: 2")).toBeInTheDocument(),
    );
    expect(screen.getByText("approved styles: 1")).toBeInTheDocument();
  });

  it("does not let a newer dedicated-batch local snapshot override hydrated detail results", async () => {
    saveLocalSheinStudioDraftSnapshot(
      {
        prompt: "stale local draft",
        styleCount: "1",
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "nanobanana",
        transparentBackground: false,
        sheinStoreId: "869",
        imageStrategy: "ai_generated",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        renderSizeImagesWithSds: true,
        designs: [{ id: "legacy-design", imageUrl: "https://example.com/legacy.png" }],
        selectedIds: ["legacy-design"],
        createdTasks: [],
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      { batchId: "batch-1" },
    );
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          id: "batch-1",
          name: "Hydrated Batch",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:01:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:00:30.000Z",
                  updatedAt: "2026-05-26T10:01:00.000Z",
                },
                {
                  id: "design-2",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-2",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-2.png",
                  reviewStatus: "unreviewed",
                  createdAt: "2026-05-26T10:00:40.000Z",
                  updatedAt: "2026-05-26T10:01:00.000Z",
                },
              ],
            },
          ],
        },
      ),
    );

    render(<SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />);

    await waitFor(() =>
      expect(screen.getByText("review grid: 2")).toBeInTheDocument(),
    );
    expect(screen.getByText("approved styles: 1")).toBeInTheDocument();
    expect(screen.queryByText("review grid: 1")).not.toBeInTheDocument();
  });

  it("creates tasks for a dedicated hydrated batch from approved itemized design ids", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue({
      savedBatch: {
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
      detail: {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        items: [
          {
            item: {
              id: "item-1",
              batchId: "batch-1",
              targetGroupKey: "size:1000x1000",
              status: "review_ready",
              selectionCount: 1,
              createdAt: "2026-05-26T09:59:00.000Z",
              updatedAt: "2026-05-26T10:00:00.000Z",
            },
            designs: [
              {
                id: "design-1",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-1",
                targetGroupKey: "size:1000x1000",
                imageUrl: "https://example.com/design-1.png",
                reviewStatus: "approved",
                createdAt: "2026-05-26T09:59:30.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
            ],
          },
        ],
      },
    });
    createSheinStudioBatchTasks.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "tasks_created",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:06:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T09:59:30.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
          ],
        },
      ],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
    });

    render(<SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />);

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "create review tasks" }));

    await waitFor(() =>
      expect(createSheinStudioBatchTasks).toHaveBeenCalledWith("batch-1", ["design-1"]),
    );
    expect(createSheinReviewTasks).not.toHaveBeenCalled();
  });

  it("keeps dedicated batch task creation on the itemized batch endpoint after local review edits", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue({
      savedBatch: {
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
      detail: {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        items: [
          {
            item: {
              id: "item-1",
              batchId: "batch-1",
              targetGroupKey: "size:1000x1000",
              status: "review_ready",
              selectionCount: 1,
              createdAt: "2026-05-26T09:59:00.000Z",
              updatedAt: "2026-05-26T10:00:00.000Z",
            },
            designs: [
              {
                id: "design-1",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-1",
                targetGroupKey: "size:1000x1000",
                imageUrl: "https://example.com/design-1.png",
                reviewStatus: "approved",
                createdAt: "2026-05-26T09:59:30.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
              {
                id: "design-2",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-2",
                targetGroupKey: "size:1000x1000",
                imageUrl: "https://example.com/design-2.png",
                reviewStatus: "unreviewed",
                createdAt: "2026-05-26T09:59:31.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
            ],
          },
        ],
      },
    });
    createSheinStudioBatchTasks.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "tasks_created",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:06:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:30.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
          ],
        },
      ],
      createdTasks: [
        { id: "task-1", title: "Task 1", designId: "design-1" },
        { id: "task-2", title: "Task 2", designId: "design-2" },
      ],
    });
    approveSheinStudioBatchDesigns.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:05:30.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:30.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:30.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:30.000Z",
              updatedAt: "2026-05-26T10:05:30.000Z",
            },
          ],
        },
      ],
    });
    approveSheinStudioBatchDesigns.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:01:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:01:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T09:59:30.000Z",
              updatedAt: "2026-05-26T10:01:00.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T09:59:31.000Z",
              updatedAt: "2026-05-26T10:01:00.000Z",
            },
          ],
        },
      ],
    });

    render(<SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />);

    await waitFor(() =>
      expect(screen.getByText("approved styles: 1")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "toggle-design-2" }));
    fireEvent.click(screen.getByRole("button", { name: "note-design-1" }));

    await waitFor(() =>
      expect(screen.getByText("approved styles: 2")).toBeInTheDocument(),
    );
    expect(approveSheinStudioBatchDesigns).toHaveBeenCalledWith("batch-1", [
      "design-1",
      "design-2",
    ]);
    fireEvent.click(screen.getByRole("button", { name: "create review tasks" }));

    await waitFor(() =>
      expect(createSheinStudioBatchTasks).toHaveBeenCalledWith("batch-1", [
        "design-1",
        "design-2",
      ]),
    );
    expect(createSheinReviewTasks).not.toHaveBeenCalled();
  });

  it("generates an active homepage batch through the itemized batch endpoint instead of session append", async () => {
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
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      }),
    );
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries updated",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries updated",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
          ],
        },
      ],
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: /Retro Cherries/ }));
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries updated" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          prompt: "retro cherries updated",
        }),
        { makeActive: false },
      ),
    );
    await waitFor(() =>
      expect(generateSheinStudioBatch).toHaveBeenCalledWith("batch-1"),
    );
    expect(generateSheinStudioDesigns).not.toHaveBeenCalled();
  });

  it("keeps homepage-generated batch task creation on batch endpoints after local review edits", async () => {
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
    getSheinStudioHydratedBatch.mockResolvedValue({
      savedBatch: {
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
      detail: {
        batch: {
          id: "batch-1",
          status: "draft",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        items: [],
      },
    });
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "unreviewed",
              createdAt: "2026-05-26T10:04:30.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
          ],
        },
      ],
    });
    createSheinStudioBatchTasks.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "tasks_created",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:06:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:30.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
          ],
        },
      ],
      createdTasks: [
        { id: "task-1", title: "Task 1", designId: "design-1" },
        { id: "task-2", title: "Task 2", designId: "design-2" },
      ],
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: /Retro Cherries/ }));
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(generateSheinStudioBatch).toHaveBeenCalledWith("batch-1"),
    );
    await waitFor(() =>
      expect(screen.getByText("approved styles: 1")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "toggle-design-2" }));
    fireEvent.click(screen.getByRole("button", { name: "note-design-1" }));
    await waitFor(() =>
      expect(screen.getByText("approved styles: 2")).toBeInTheDocument(),
    );
    expect(approveSheinStudioBatchDesigns).toHaveBeenCalledWith("batch-1", [
      "design-1",
      "design-2",
    ]);
    fireEvent.click(screen.getByRole("button", { name: "create review tasks" }));

    await waitFor(() =>
      expect(createSheinStudioBatchTasks).toHaveBeenCalledWith("batch-1", [
        "design-1",
        "design-2",
      ]),
    );
    expect(createSheinReviewTasks).not.toHaveBeenCalled();
  });

  it("does not reload the dedicated batch when editing the prompt", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        name: "Retro Cherries",
        prompt: "",
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    const promptInput = await screen.findByLabelText("prompt");
    const callCountBeforeEdit = getSheinStudioHydratedBatch.mock.calls.length;
    fireEvent.change(promptInput, { target: { value: "vintage botanical clock" } });

    await waitFor(() =>
      expect(screen.getByDisplayValue("vintage botanical clock")).toBeInTheDocument(),
    );
    await waitFor(() =>
      expect(loadLocalSheinStudioDraftSnapshotDetail()).toMatchObject({
        batchId: "batch-1",
        draft: expect.objectContaining({
          prompt: "vintage botanical clock",
        }),
      }),
    );
    expect(getSheinStudioHydratedBatch).toHaveBeenCalledTimes(callCountBeforeEdit);
  });

  it("surfaces dedicated batch hydration failures instead of silently leaving an empty shell", async () => {
    getSheinStudioHydratedBatch.mockRejectedValue(
      new Error("Missing ZITADEL session"),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "当前批次加载失败：Missing ZITADEL session。请重新登录后再继续。",
        ),
      ).toBeInTheDocument(),
    );
  });

  it("keeps dedicated batch prompt edits in local draft without remote autosave", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        prompt: "",
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    const promptInput = await screen.findByLabelText("prompt");
    fireEvent.change(promptInput, { target: { value: "updated prompt" } });

    await waitFor(() =>
      expect(loadLocalSheinStudioDraftSnapshotDetail()).toMatchObject({
        batchId: "batch-1",
        draft: expect.objectContaining({
          prompt: "updated prompt",
        }),
      }),
      { timeout: 3000 },
    );
    expect(saveSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("refreshes the dedicated batch version immediately before generate", async () => {
    getSheinStudioHydratedBatch
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            draftUpdatedAt: "2026-05-26T10:00:00.000Z",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              draftUpdatedAt: "2026-05-26T10:06:00.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            draftUpdatedAt: "2026-05-26T10:00:00.000Z",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              draftUpdatedAt: "2026-05-26T10:07:00.000Z",
              updatedAt: "2026-05-26T10:08:00.000Z",
            },
          },
        ),
      );
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      name: "Retro Cherries",
      draftUpdatedAt: "2026-05-26T10:07:00.000Z",
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        status: "review_ready",
        updatedAt: "2026-05-26T10:07:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:07:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:06:30.000Z",
              updatedAt: "2026-05-26T10:07:00.000Z",
            },
          ],
        },
      ],
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          updatedAt: "2026-05-26T10:07:00.000Z",
        }),
        { makeActive: false },
      ),
    );
    await waitFor(() =>
      expect(generateSheinStudioBatch).toHaveBeenCalledWith("batch-1"),
    );
  });

  it("retries failed dedicated batch items when generate is clicked on a partially failed batch", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          batch: {
            ...buildHydratedBatch().detail.batch,
            status: "partially_failed",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "failed",
                selectionCount: 1,
                lastError: "excessive system load",
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [],
            },
          ],
        },
      ),
    );
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      name: "Retro Cherries",
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    retrySheinStudioBatchItems.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        status: "generating",
        updatedAt: "2026-05-26T10:07:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "generating",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:07:00.000Z",
          },
          designs: [],
        },
      ],
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    expect(screen.getByRole("button", { name: "重试失败批次" })).toBeInTheDocument();
    expect(screen.getByText(/当前批次有 1 个失败项/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重试失败批次" }));

    await waitFor(() =>
      expect(retrySheinStudioBatchItems).toHaveBeenCalledWith("batch-1", [
        "item-1",
      ]),
    );
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("routes completed itemized task creation results back into the task list", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          batch: {
            ...buildHydratedBatch().detail.batch,
            status: "review_ready",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:01:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
              ],
            },
          ],
        },
      ),
    );
    createSheinStudioBatchTasks.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        status: "tasks_created",
        updatedAt: "2026-05-26T10:08:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:08:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:01:00.000Z",
              updatedAt: "2026-05-26T10:08:00.000Z",
            },
          ],
        },
      ],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
      failedTasks: [],
      statusGroups: {
        items: [
          { key: "draft_saved", label: "已保存草稿", count: 1, ids: ["task-1"] },
        ],
        byKey: {
          draft_saved: {
            key: "draft_saved",
            label: "已保存草稿",
            count: 1,
            ids: ["task-1"],
          },
        },
      },
    });

    render(<SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />);

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "create review tasks" }));

    await waitFor(() =>
      expect(createSheinStudioBatchTasks).toHaveBeenCalledWith("batch-1", [
        "design-1",
      ]),
    );
    await waitFor(() =>
      expect(screen.getByText("created tasks: 1")).toBeInTheDocument(),
    );
  });

  it("keeps the dedicated batch page busy when generate times out but the batch is still running", async () => {
    getSheinStudioHydratedBatch
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "draft",
              updatedAt: "2026-05-26T10:00:00.000Z",
            },
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "generating",
              updatedAt: "2026-05-26T10:07:00.000Z",
            },
            items: [
              {
                item: {
                  id: "item-1",
                  batchId: "batch-1",
                  targetGroupKey: "size:1000x1000",
                  status: "generating",
                  selectionCount: 1,
                  createdAt: "2026-05-26T09:59:00.000Z",
                  updatedAt: "2026-05-26T10:07:00.000Z",
                },
                designs: [],
              },
            ],
          },
        ),
      );
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      name: "Retro Cherries",
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    generateSheinStudioBatch.mockRejectedValue(
      new Error("ListingKit API request failed: 504"),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(generateSheinStudioBatch).toHaveBeenCalledWith("batch-1"),
    );
    await waitFor(() =>
      expect(getSheinStudioHydratedBatch).toHaveBeenCalledTimes(3),
    );
    await waitFor(() =>
      expect(screen.getByText("正在生成款式图")).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("button", { name: "generate styles" }),
    ).toBeDisabled();
  });

  it("keeps the dedicated batch page busy when failed-item retry times out but hydration shows retry is still running", async () => {
    getSheinStudioHydratedBatch
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "partially_failed",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
            items: [
              {
                item: {
                  id: "item-1",
                  batchId: "batch-1",
                  targetGroupKey: "size:1000x1000",
                  status: "failed",
                  selectionCount: 1,
                  lastError: "excessive system load",
                  createdAt: "2026-05-26T09:59:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
                designs: [],
              },
            ],
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "partially_failed",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
            items: [
              {
                item: {
                  id: "item-1",
                  batchId: "batch-1",
                  targetGroupKey: "size:1000x1000",
                  status: "failed",
                  selectionCount: 1,
                  lastError: "excessive system load",
                  createdAt: "2026-05-26T09:59:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
                designs: [],
              },
            ],
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "generating",
              updatedAt: "2026-05-26T10:07:00.000Z",
            },
            items: [
              {
                item: {
                  id: "item-1",
                  batchId: "batch-1",
                  targetGroupKey: "size:1000x1000",
                  status: "generating",
                  selectionCount: 1,
                  createdAt: "2026-05-26T09:59:00.000Z",
                  updatedAt: "2026-05-26T10:07:00.000Z",
                },
                designs: [],
              },
            ],
          },
        ),
      );
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      name: "Retro Cherries",
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    retrySheinStudioBatchItems.mockRejectedValue(
      new Error("ListingKit API request failed: 504"),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "重试失败批次" }));

    await waitFor(() =>
      expect(retrySheinStudioBatchItems).toHaveBeenCalledWith("batch-1", [
        "item-1",
      ]),
    );
    await waitFor(() =>
      expect(getSheinStudioHydratedBatch).toHaveBeenCalledTimes(3),
    );
    await waitFor(() =>
      expect(screen.getByText("正在生成款式图")).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("button", { name: "generate styles" }),
    ).toBeDisabled();
  });

  it("retries a single failed batch item without retriggering the whole failed set", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          batch: {
            ...buildHydratedBatch().detail.batch,
            status: "partially_failed",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                targetGroupLabel: "黑色 M",
                status: "failed",
                selectionCount: 1,
                lastError: "upstream timeout",
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [],
            },
            {
              item: {
                id: "item-2",
                batchId: "batch-1",
                targetGroupKey: "size:1200x1200",
                targetGroupLabel: "白色 L",
                status: "failed",
                selectionCount: 1,
                lastError: "too many requests",
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [],
            },
          ],
        },
      ),
    );
    retrySheinStudioBatchItems.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        status: "partially_failed",
        updatedAt: "2026-05-26T10:07:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            targetGroupLabel: "黑色 M",
            status: "generating",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:07:00.000Z",
          },
          designs: [],
        },
        {
          item: {
            id: "item-2",
            batchId: "batch-1",
            targetGroupKey: "size:1200x1200",
            targetGroupLabel: "白色 L",
            status: "failed",
            selectionCount: 1,
            lastError: "too many requests",
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          designs: [],
        },
      ],
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "retry-item-item-1" })).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "retry-item-item-1" }));

    await waitFor(() =>
      expect(retrySheinStudioBatchItems).toHaveBeenCalledWith("batch-1", [
        "item-1",
      ]),
    );
    expect(retrySheinStudioBatchItems).not.toHaveBeenCalledWith("batch-1", [
      "item-1",
      "item-2",
    ]);
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("saves the dedicated batch back into the same batch id from the save button", async () => {
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "批次1",
      prompt: "updated prompt",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        prompt: "updated prompt",
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("updated prompt")).toBeInTheDocument(),
    );

    const onSaveBatch = await waitFor(() => {
      const handler = lastGenerationPanelProps?.onSaveBatch as
        | (() => Promise<void> | void)
        | undefined;
      expect(typeof handler).toBe("function");
      return handler;
    });

    saveSheinStudioBatch.mockClear();

    await act(async () => {
      await onSaveBatch?.();
    });

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          prompt: "updated prompt",
        }),
        { makeActive: false },
      ),
    );
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
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        name: "Retro Cherries",
        prompt: "retro cherries",
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    expect(screen.getByText("入口商品")).toBeInTheDocument();
    expect(
      screen.queryByDisplayValue("legacy local draft"),
    ).not.toBeInTheDocument();
  });

  it("does not let a stale local snapshot hide newer grouped selections from the dedicated batch", async () => {
    saveLocalSheinStudioDraftSnapshot(
      {
        prompt: "stale local draft",
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
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      { batchId: "batch-1" },
    );
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        name: "Retro Cherries",
        prompt: "retro cherries",
        groupedSelections: [
          {
            selectionId: "sel-hoodie",
            sheinStoreId: "869",
            selection: {
              ...selection,
              variantId: 101,
              layerId: "layer-2",
              productName: "hoodie",
              variantLabel: "L / white",
            },
            baselineStatus: "ready",
            baselineReason: "",
            eligible: true,
          },
        ],
        updatedAt: "2026-05-26T10:05:00.000Z",
      }, {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:05:00.000Z",
        },
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByText("已加入 1 款")).toBeInTheDocument(),
    );
    expect(screen.queryByText("已加入 0 款")).not.toBeInTheDocument();
  });

  it("does not let a newer but incomplete local snapshot erase dedicated batch context", async () => {
    saveLocalSheinStudioDraftSnapshot(
      {
        prompt: "",
        styleCount: "1",
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "nanobanana",
        transparentBackground: false,
        sheinStoreId: "",
        imageStrategy: "ai_generated",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        renderSizeImagesWithSds: true,
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:06:00.000Z",
      },
      { batchId: "batch-1" },
    );
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        name: "869全品类",
        prompt: "retro cherries",
        sheinStoreId: "869",
        groupedSelections: [groupedSelection],
        updatedAt: "2026-05-26T10:05:00.000Z",
      }, {
        batch: {
          id: "batch-1",
          status: "draft",
          prompt: "",
          styleCount: "",
          sheinStoreId: 0,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:05:00.000Z",
        },
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    await waitFor(() =>
      expect(screen.getByText("已加入 1 款")).toBeInTheDocument(),
    );
    expect(screen.queryByText("已加入 0 款")).not.toBeInTheDocument();
  });

  it("lets the dedicated batch page rename the current batch without losing current draft state", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(),
    );
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "批次1",
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
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "批次9",
      prompt: "updated prompt",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:10:00.000Z",
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    const promptInput = await screen.findByLabelText("prompt");
    fireEvent.change(promptInput, { target: { value: "updated prompt" } });

    fireEvent.click(await screen.findByRole("button", { name: "重命名当前批次" }));
    const nameInput = await screen.findByLabelText("当前批次名称");
    fireEvent.change(nameInput, { target: { value: "批次9" } });
    fireEvent.click(screen.getByRole("button", { name: "保存名称" }));

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          name: "批次9",
          prompt: "updated prompt",
        }),
        { makeActive: false },
      ),
    );
    expect(await screen.findByText("已重命名为：批次9")).toBeInTheDocument();
  });

  it("uses hydrated batch truth for homepage metadata-only writes", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Stale Batch",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        generationJobs: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          id: "batch-1",
          name: "Fresh Batch",
          prompt: "retro cherries",
          designs: [{ id: "design-1", imageUrl: "https://example.com/design-1.png" }],
          selectedIds: ["design-1"],
          createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
          generationJobs: [
            {
              id: "job-1",
              status: "succeeded",
              requestedCount: 1,
              completedCount: 1,
              createdAt: "2026-05-26T10:02:00.000Z",
              updatedAt: "2026-05-26T10:03:00.000Z",
            },
          ],
          generationError: "legacy warning",
          generationJobId: "job-1",
          updatedAt: "2026-05-26T10:03:00.000Z",
        },
        {
          batch: {
            id: "batch-1",
            status: "tasks_created",
            prompt: "retro cherries",
            styleCount: "1",
            sheinStoreId: 869,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:03:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:03:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:01:00.000Z",
                  updatedAt: "2026-05-26T10:03:00.000Z",
                },
              ],
            },
          ],
        },
      ),
    );
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Renamed Batch",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [{ id: "design-1", imageUrl: "https://example.com/design-1.png" }],
      selectedIds: ["design-1"],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
      generationJobs: [
        {
          id: "job-1",
          status: "succeeded",
          requestedCount: 1,
          completedCount: 1,
          createdAt: "2026-05-26T10:02:00.000Z",
          updatedAt: "2026-05-26T10:03:00.000Z",
        },
      ],
      generationError: "legacy warning",
      generationJobId: "job-1",
      updatedAt: "2026-05-26T10:03:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    await waitFor(() =>
      expect(getSheinStudioHydratedBatch).toHaveBeenCalledWith("batch-1"),
    );

    fireEvent.click(screen.getByRole("button", { name: "重命名" }));
    fireEvent.change(screen.getByLabelText("批次名称"), {
      target: { value: "Renamed Batch" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存名称" }));

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          name: "Renamed Batch",
          updatedAt: "2026-05-26T10:03:00.000Z",
          selectedIds: ["design-1"],
          createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
          generationJobs: [
            expect.objectContaining({
              id: "job-1",
              status: "succeeded",
            }),
          ],
          generationError: "legacy warning",
          generationJobId: "job-1",
          designs: [
            expect.objectContaining({
              id: "design-1",
              imageUrl: "https://example.com/design-1.png",
            }),
          ],
        }),
        { makeActive: false },
      ),
    );
  });

  it("duplicates a recent batch without carrying over in-flight generation state", async () => {
    const sourceBatch = {
      id: "batch-1",
      name: "Fresh Batch",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      groupedSelections: [],
      groups: [],
      designs: [{ id: "design-1", imageUrl: "https://example.com/design-1.png" }],
      selectedIds: ["design-1"],
      createdTasks: [],
      generationJobs: [
        {
          jobId: "job-1",
          targetGroupKey: "primary",
          targetGroupLabel: "当前商品",
          status: "running" as const,
        },
      ],
      generationJobId: "job-1",
      updatedAt: "2026-05-26T10:03:00.000Z",
    };
    const duplicatedBatch = {
      id: "batch-2",
      name: "Fresh Batch 副本",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      generationJobs: [],
      updatedAt: "2026-05-26T10:04:00.000Z",
    };
    listSheinStudioBatches
      .mockResolvedValueOnce([sourceBatch])
      .mockResolvedValue([duplicatedBatch, sourceBatch]);
    getSheinStudioHydratedBatch.mockImplementation(async (batchId: string) => {
      if (batchId === "batch-1") {
        return buildHydratedBatch(sourceBatch);
      }
      return buildHydratedBatch(duplicatedBatch, {
        batch: {
          id: "batch-2",
          status: "draft",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T10:03:30.000Z",
          updatedAt: "2026-05-26T10:04:00.000Z",
        },
      });
    });
    saveSheinStudioBatch.mockResolvedValue({
      ...duplicatedBatch,
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: "复制" }));

    await waitFor(() =>
      expect(getSheinStudioHydratedBatch).toHaveBeenCalledWith("batch-1"),
    );
    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: undefined,
          name: "Fresh Batch 副本",
          prompt: "retro cherries",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          generationJobs: [],
        }),
        { makeActive: false },
      ),
    );

    expect(await screen.findByText("Fresh Batch 副本")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Fresh Batch 副本"));

    await waitFor(() =>
      expect(getSheinStudioHydratedBatch).toHaveBeenCalledWith("batch-2"),
    );
    expect(resumeSheinStudioDesignGeneration).not.toHaveBeenCalled();
    expect(
      screen.queryByText("已恢复之前发起的生成任务，正在继续等待结果。"),
    ).not.toBeInTheDocument();
  });

  it("lets the dedicated batch page delete the current batch and return to the homepage", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(),
    );
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "批次1",
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
    vi.spyOn(window, "confirm").mockReturnValue(true);

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    fireEvent.click(await screen.findByRole("button", { name: "删除当前批次" }));

    await waitFor(() => expect(deleteSheinStudioBatch).toHaveBeenCalledWith("batch-1"));
    expect(push).toHaveBeenCalledWith("/listing-kits/sds");
  });

  it("lets the dedicated batch page jump to SDS selection for the current batch", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(),
    );
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "批次1",
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

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "去 SDS 选品并加入当前批次" }),
    );

    expect(setActiveSheinStudioBatchId).toHaveBeenCalledWith("batch-1");
    expect(push).toHaveBeenCalledWith("/listing-kits/sds/new?targetBatchId=batch-1");
  });

  it("offers a baseline recovery action on the dedicated batch page when the active selection is abnormal", async () => {
    getSDSBaselineReadiness.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "failed",
      reasonCode: "cache_unavailable",
      reason: "",
    });
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByText("当前 SDS 选择还没有可用的 baseline 缓存。")).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("button", { name: "重试 baseline 校验" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重试 baseline 校验" }));

    await waitFor(() =>
      expect(warmSDSBaselineForSelection).toHaveBeenCalledWith(selection),
    );
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
    expect(screen.queryByText("入口商品")).not.toBeInTheDocument();
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

  it("updates the loaded homepage batch instead of creating a duplicate when saving", async () => {
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
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries updated",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:05:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: /Retro Cherries/ }));
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries updated" },
    });

    const onSaveBatch = lastGenerationPanelProps?.onSaveBatch as
      | (() => Promise<void> | void)
      | undefined;
    expect(typeof onSaveBatch).toBe("function");
    await act(async () => {
      await onSaveBatch?.();
    });

    await waitFor(() =>
      expect(
        saveSheinStudioBatch.mock.calls.some(
          ([input]) => {
            const candidate = input as { id?: string; prompt?: string } | undefined;
            return (
              candidate?.id === "batch-1" &&
              candidate.prompt === "retro cherries updated"
            );
          },
        ),
      ).toBe(true),
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

  it("starts a server-side batch run from homepage selection for generate mode", async () => {
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
    startSheinStudioBatchRun.mockResolvedValue({
      run: {
        id: "run-1",
      },
      items: [],
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(["batch-1", "batch-2"]),
    );
    expect(await screen.findByText("batch run progress: run-1")).toBeInTheDocument();
    expect(screen.queryByText("最近批次")).not.toBeInTheDocument();
  });

  it("shows batch run start errors inline on the homepage", async () => {
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
    startSheinStudioBatchRun.mockRejectedValue(new Error("run start failed"));

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));

    await waitFor(() =>
      expect(screen.getByText("run start failed")).toBeInTheDocument(),
    );
    expect(screen.queryByText(/batch run progress:/)).not.toBeInTheDocument();
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
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
          selectedIds: ["design-1"],
        },
        {
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T09:59:30.000Z",
                  updatedAt: "2026-05-26T10:00:00.000Z",
                },
              ],
            },
          ],
        },
      ),
    );

    render(<SheinStudioWorkbench activeStep="generate" />);

    expect(getSheinStudioHydratedBatch).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "批量去创建任务 1 个" }),
    );

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    expect(getSheinStudioHydratedBatch).toHaveBeenCalledWith("batch-1");
    expect(screen.getByText("第 1 / 1 个批次")).toBeInTheDocument();
    expect(
      screen.getByText("已定位到审核区，可直接创建任务或调整款式。"),
    ).toBeInTheDocument();
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled());
  });

  it("ignores stale recent-batch hydration when a newer batch selection wins", async () => {
    Element.prototype.scrollIntoView = vi.fn();
    const batchADeferred = createDeferred<ReturnType<typeof buildHydratedBatch>>();
    const batchBDeferred = createDeferred<ReturnType<typeof buildHydratedBatch>>();
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-a",
        name: "Batch A",
        prompt: "prompt a",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-b",
        name: "Batch B",
        prompt: "prompt b",
        styleCount: "1",
        sheinStoreId: "869",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:01:00.000Z",
      },
    ]);
    getSheinStudioHydratedBatch.mockImplementation((batchId: string) => {
      if (batchId === "batch-a") {
        return batchADeferred.promise;
      }
      if (batchId === "batch-b") {
        return batchBDeferred.promise;
      }
      return Promise.resolve(null);
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    const batchACard = (await screen.findByText("Batch A")).closest('[role="button"]');
    const batchBCard = screen.getByText("Batch B").closest('[role="button"]');
    expect(batchACard).not.toBeNull();
    expect(batchBCard).not.toBeNull();

    fireEvent.click(batchACard!);
    fireEvent.click(batchBCard!);

    await act(async () => {
      batchBDeferred.resolve(
        buildHydratedBatch(
          {
            id: "batch-b",
            name: "Batch B",
            prompt: "prompt b",
            selection: groupedSelection.selection,
            updatedAt: "2026-05-26T10:02:00.000Z",
          },
          {
            batch: {
              id: "batch-b",
              status: "draft",
              prompt: "prompt b",
              styleCount: "1",
              sheinStoreId: 869,
              createdAt: "2026-05-26T10:00:00.000Z",
              updatedAt: "2026-05-26T10:02:00.000Z",
            },
          },
        ),
      );
      await Promise.resolve();
    });

    await waitFor(() =>
      expect(screen.getByDisplayValue("prompt b")).toBeInTheDocument(),
    );

    await act(async () => {
      batchADeferred.resolve(
        buildHydratedBatch(
          {
            id: "batch-a",
            name: "Batch A",
            prompt: "prompt a",
            updatedAt: "2026-05-26T10:02:00.000Z",
          },
          {
            batch: {
              id: "batch-a",
              status: "draft",
              prompt: "prompt a",
              styleCount: "1",
              sheinStoreId: 869,
              createdAt: "2026-05-26T09:59:00.000Z",
              updatedAt: "2026-05-26T10:02:00.000Z",
            },
          },
        ),
      );
      await Promise.resolve();
    });

    expect(screen.getByDisplayValue("prompt b")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("prompt a")).not.toBeInTheDocument();
  });

  it("returns from batch run progress and refreshes saved batches", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Batch 1",
        prompt: "prompt 1",
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
        name: "Batch 2",
        prompt: "prompt 2",
        styleCount: "1",
        sheinStoreId: "869",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:01:00.000Z",
      },
    ]);
    startSheinStudioBatchRun.mockResolvedValue({
      run: {
        id: "run-2",
      },
      items: [],
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));

    await waitFor(() =>
      expect(screen.getByText("batch run progress: run-2")).toBeInTheDocument(),
    );
    listSheinStudioBatches.mockResolvedValueOnce([
      {
        id: "batch-1",
        name: "Batch 1",
        prompt: "prompt 1",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);
    const callsBeforeBack = listSheinStudioBatches.mock.calls.length;

    fireEvent.click(screen.getByRole("button", { name: "back from batch run" }));

    await waitFor(() =>
      expect(listSheinStudioBatches.mock.calls.length).toBeGreaterThan(callsBeforeBack),
    );
    expect(await screen.findByText("最近批次")).toBeInTheDocument();
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
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
          ],
        },
      ],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    expect(
      screen.queryByText(
        "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
      ),
    ).not.toBeInTheDocument();
  });

  it("creates a brand-new homepage batch before generation and skips legacy session sync", async () => {
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
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      id: "batch-new",
      name: "新批次",
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        id: "batch-new",
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-new",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-new",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
          ],
        },
      ],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    expect(saveSheinStudioBatch).toHaveBeenCalledWith(
      expect.not.objectContaining({ id: expect.anything() }),
      undefined,
    );
    expect(generateSheinStudioBatch).toHaveBeenCalledWith("batch-new");
  });

  it("guards against leaving the page while style generation is running", async () => {
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
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    generateSheinStudioDesigns.mockImplementation(
      () =>
        new Promise(() => {
          return undefined;
        }),
    );

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
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

  it("does not auto-resume an in-flight generation job after returning to the page", async () => {
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
      batchStatus: "generating",
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "generate styles" })).toBeInTheDocument(),
    );
    expect(resumeSheinStudioDesignGeneration).not.toHaveBeenCalled();
    expect(screen.queryByText("review grid: 1")).not.toBeInTheDocument();
    expect(saveSheinStudioDraftWithOptions).not.toHaveBeenCalled();
    expect(saveSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("does not auto-resume multiple in-flight generation jobs after returning to the page", async () => {
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
      generationJobs: [
        {
          jobId: "job-123",
          targetGroupKey: "primary",
          targetGroupLabel: "当前商品",
          status: "running",
        },
        {
          jobId: "job-456",
          targetGroupKey: "group-1",
          targetGroupLabel: "分组商品 1",
          status: "running",
        },
      ],
      batchStatus: "generating",
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "generate styles" })).toBeInTheDocument(),
    );
    expect(resumeSheinStudioDesignGeneration).not.toHaveBeenCalled();
    expect(screen.queryByText("review grid: 2")).not.toBeInTheDocument();
    expect(saveSheinStudioDraftWithOptions).not.toHaveBeenCalled();
    expect(saveSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("does not use legacy session sync when an unsaved homepage workspace generates through batch APIs", async () => {
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
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      id: "batch-unsaved",
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        id: "batch-unsaved",
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-unsaved",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-unsaved",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
          ],
        },
      ],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() => expect(loadSheinStudioDraft).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
  });

  it("keeps generated designs when parent step changes to review", async () => {
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
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
          ],
        },
      ],
    });

    const rendered = render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
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
    loadSheinStudioDraft.mockResolvedValue({
      prompt: "",
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
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [],
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

  it("does not surface a draft-save warning after successful batch generation", async () => {
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
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
          ],
        },
      ],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    expect(
      screen.queryByText(
        "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
      ),
    ).not.toBeInTheDocument();
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

  it("requires choosing a batch store before generating styles", async () => {
    loadSheinStudioDraft.mockResolvedValue({
      prompt: "retro cherries",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "",
      imageStrategy: "ai_generated",
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
      expect(
        screen.getByText("请先选择批次店铺，再生成款式图或创建 SHEIN 资料。"),
      ).toBeInTheDocument(),
    );
    expect(lastGenerationPanelProps?.storeRequiredMessage).toBe(
      "请先选择批次店铺，再生成款式图或创建 SHEIN 资料。",
    );

    const onGenerate = lastGenerationPanelProps?.onGenerate as
      | (() => Promise<void> | void)
      | undefined;
    await act(async () => {
      await onGenerate?.();
    });

    expect(generateSheinStudioDesigns).not.toHaveBeenCalled();
    expect(screen.getByText("请先选择批次店铺。")).toBeInTheDocument();
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

  it("shows grouped-candidate recovery guidance after returning from candidate pool", async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    saveSDSGroupedCandidateHandoff({
      action: "focus_generate",
      actionLabel: "去生成并继续校验",
      message:
        "这款候选商品还没有 baseline 缓存。先在当前工作台完成一次生成或预热，再回来继续校验并加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(
        screen.getByText(
          "这款候选商品还没有 baseline 缓存。先在当前工作台完成一次生成或预热，再回来继续校验并加入 grouped 批量上品。",
        ),
      ).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "去生成并继续校验" }));
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled());
  });

  it("routes login-blocked grouped guidance to the SDS login page", async () => {
    saveSDSGroupedCandidateHandoff({
      action: "open_sds_login",
      actionLabel: "去处理 SDS 登录",
      message: "当前 SDS 登录缺少 access token。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "去处理 SDS 登录" })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "去处理 SDS 登录" }));
    expect(push).toHaveBeenCalledWith("/listing-kits/sds-login");
  });

  it("routes the active-selection baseline action to the SDS login page when login is blocked", async () => {
    getSDSBaselineReadiness.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "blocked",
      reasonCode: "login_missing_credentials",
      reason: "",
    });
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    const actionButton = await screen.findByRole("button", {
      name: "去处理 SDS 登录",
    });
    fireEvent.click(actionButton);

    expect(push).toHaveBeenCalledWith("/listing-kits/sds-login");
  });

  it("warms baseline directly from grouped-candidate guidance", async () => {
    saveSDSGroupedCandidateHandoff({
      action: "warm_baseline",
      actionLabel: "一键预热并校验 baseline",
      message:
        "这款候选商品还没有 baseline 缓存。先预热并校验 baseline，再回来加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "一键预热并校验 baseline" })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "一键预热并校验 baseline" }));

    await waitFor(() => expect(warmSDSBaselineForSelection).toHaveBeenCalledWith(selection));
    await waitFor(() =>
      expect(
        screen.getByText("这款 SDS 商品的 baseline 已通过校验，现在可以继续加入 grouped 批量上品。"),
      ).toBeInTheDocument(),
    );
  });

  it("keeps the baseline recovery action available when warmup still needs more validation", async () => {
    warmSDSBaselineForSelection.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "baseline_cached",
      reasonCode: "cache_unavailable",
      reason: "",
    });
    saveSDSGroupedCandidateHandoff({
      action: "warm_baseline",
      actionLabel: "一键预热并校验 baseline",
      message:
        "这款候选商品还没有 baseline 缓存。先预热并校验 baseline，再回来加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "一键预热并校验 baseline" })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "一键预热并校验 baseline" }));

    await waitFor(() =>
      expect(screen.getAllByRole("button", { name: "继续 baseline 校验" }).length).toBeGreaterThan(0),
    );
    expect(
      screen.getAllByText("当前 SDS 选择还没有可用的 baseline 缓存。").length,
    ).toBeGreaterThan(0);
  });

  it("shows a direct fallback message when warmup returns cached baseline without a reason", async () => {
    warmSDSBaselineForSelection.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "baseline_cached",
      reasonCode: "",
      reason: "",
    });
    saveSDSGroupedCandidateHandoff({
      action: "warm_baseline",
      actionLabel: "一键预热并校验 baseline",
      message:
        "这款候选商品还没有 baseline 缓存。先预热并校验 baseline，再回来加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "一键预热并校验 baseline" })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "一键预热并校验 baseline" }));

    await waitFor(() =>
      expect(
        screen.getByText("这款 SDS 商品已经完成 baseline 缓存，当前没有更多校验结果。可以继续使用，必要时再手动复查。"),
      ).toBeInTheDocument(),
    );
  });
});
