import { vi } from "vitest";

export const useQuery = vi.fn();
export const generateSheinStudioDesigns = vi.fn();
export const analyzeSheinStudioReferenceStyle = vi.fn();
export const createSheinReviewTasks = vi.fn();
export const getSDSBaselineReadiness = vi.fn();
export const warmSDSBaselineForSelection = vi.fn();
export const hydrateSDSVariantSelection = vi.fn();
export const listSheinStudioBatches = vi.fn();
export const getSheinStudioBatch = vi.fn();
export const getSheinStudioHydratedBatch = vi.fn();
export const loadSheinStudioDraft = vi.fn();
export const saveSheinStudioBatch = vi.fn();
export const saveSheinStudioDraftWithOptions = vi.fn();
export const setActiveSheinStudioBatchId = vi.fn();
export const generateSheinStudioBatch = vi.fn();
export const retrySheinStudioBatchItems = vi.fn();
export const startSheinStudioBatchRun = vi.fn();
export const deleteSheinStudioBatch = vi.fn();
export const approveSheinStudioBatchDesigns = vi.fn();
export const createSheinStudioBatchTasks = vi.fn();
export const push = vi.fn();
export let lastGenerationPanelProps: Record<string, unknown> | null = null;

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/sds",
  useRouter: () => ({ push }),
  useSearchParams: () => new URLSearchParams("step=generate"),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (...args: unknown[]) => useQuery(...args),
}));

vi.mock(
  "@/components/listingkit/shein-studio/shein-studio-progress-strip",
  () => ({
    SheinStudioProgressStrip: () => <div>progress strip</div>,
  }),
);

vi.mock(
  "@/components/listingkit/shein-studio/shein-studio-batch-run-progress",
  () => ({
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
  }),
);

vi.mock(
  "@/components/listingkit/shein-studio/shein-created-tasks-list",
  () => ({
    SheinCreatedTasksList: ({
      failedTasks = [],
      rejectedTasks = [],
      reusedTasks = [],
      tasks,
    }: {
      failedTasks?: Array<{ message?: string; reasonCode?: string }>;
      rejectedTasks?: Array<{ message?: string; reasonCode?: string }>;
      reusedTasks?: Array<{ id: string }>;
      tasks: Array<{ id: string }>;
    }) => (
      <div>
        <div>created tasks: {tasks.length}</div>
        <div>reused tasks: {reusedTasks.length}</div>
        <div>rejected tasks: {rejectedTasks.length}</div>
        <div>failed tasks: {failedTasks.length}</div>
        {rejectedTasks.map((task) => (
          <div key={task.reasonCode ?? task.message}>
            rejected reason: {task.reasonCode} {task.message}
          </div>
        ))}
        {failedTasks.map((task) => (
          <div key={task.reasonCode ?? task.message}>
            failed reason: {task.reasonCode} {task.message}
          </div>
        ))}
      </div>
    ),
  }),
);

vi.mock(
  "@/components/listingkit/shein-studio/shein-design-preview-grid",
  () => ({
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
        <div>
          approved styles: {Array.isArray(selectedIds) ? selectedIds.length : 0}
        </div>
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
  }),
);

vi.mock(
  "@/components/listingkit/shein-studio/shein-studio-generation-panel",
  () => ({
    SheinStudioGenerationPanel: (props: {
      actions: {
        onCreateTasks?: () => void;
        onGenerate: () => void;
        onRetryFailedItem?: (itemId: string) => void;
        onRestorePrompt?: (value: string) => void;
        onSaveBatch?: () => void;
        analyzeReferenceStyle?: (input: {
          basePrompt?: string;
          referenceImageUrls: string[];
        }) => Promise<unknown>;
        setPrompt: (value: string) => void;
        setHotStyleReferenceBrief?: (value: string) => void;
        setHotStyleReferenceImageUrls?: (value: string[]) => void;
        setHotStyleReferencePrompt?: (value: string) => void;
      };
      form: {
        groupedImageMode?: string;
        hotStyleReferenceBrief?: string;
        hotStyleReferenceImageUrls?: string[];
        hotStyleReferencePrompt?: string;
        prompt: string;
        promptHistory?: Array<{ prompt: string; createdAt: string }>;
        selectedSdsImages?: Array<{
          color?: string;
          imageUrl: string;
          variantSku?: string;
        }>;
      };
      status: {
        failedBatchItems?: Array<{
          id: string;
          lastError?: string;
          targetGroupLabel?: string;
          targetGroupKey: string;
        }>;
        generateButtonLabel?: string;
        generationNotice?: string;
        generationError?: string;
        isGenerating?: boolean;
        isRetryingFailedItems?: boolean;
        retryingFailedItemId?: string;
        showSavedBatches?: boolean;
        subscriptionBlockedMessage?: string;
        storeRequiredMessage?: string;
      };
    }) => {
      const flatProps = {
        ...props.form,
        ...props.status,
        ...props.actions,
      };
      lastGenerationPanelProps = flatProps as Record<string, unknown>;
      const actionLabel =
        flatProps.generateButtonLabel === "重试失败批次"
          ? "重试失败批次"
          : "generate styles";
      return (
        <div id="shein-studio-generator">
          <label htmlFor="prompt">prompt</label>
          <input
            id="prompt"
            onChange={(event) => flatProps.setPrompt(event.target.value)}
            value={flatProps.prompt}
          />
          <button
            disabled={flatProps.isGenerating}
            onClick={flatProps.onGenerate}
            type="button"
          >
            {actionLabel}
          </button>
          {flatProps.onSaveBatch ? (
            <button onClick={flatProps.onSaveBatch} type="button">
              save batch
            </button>
          ) : null}
          {flatProps.onCreateTasks ? (
            <button onClick={flatProps.onCreateTasks} type="button">
              create tasks from panel
            </button>
          ) : null}
          {(flatProps.promptHistory ?? []).map((entry) => (
            <button
              key={entry.createdAt}
              onClick={() => flatProps.onRestorePrompt?.(entry.prompt)}
              type="button"
            >
              restore-{entry.prompt}
            </button>
          ))}
          <div>
            selected SDS images:{" "}
            {Array.isArray(flatProps.selectedSdsImages)
              ? flatProps.selectedSdsImages.length
              : 0}
          </div>
          <div>
            saved batches visible:{" "}
            {flatProps.showSavedBatches === false ? "no" : "yes"}
          </div>
          {flatProps.subscriptionBlockedMessage ? (
            <div>{flatProps.subscriptionBlockedMessage}</div>
          ) : null}
          {flatProps.storeRequiredMessage ? (
            <div>{flatProps.storeRequiredMessage}</div>
          ) : null}
          {flatProps.generationNotice ? (
            <div>{flatProps.generationNotice}</div>
          ) : null}
          {flatProps.generationError ? (
            <div>{flatProps.generationError}</div>
          ) : null}
          {(flatProps.failedBatchItems ?? []).map((item) => (
            <div key={item.id}>
              <span>{item.targetGroupLabel ?? item.targetGroupKey}</span>
              {item.lastError ? <span>{item.lastError}</span> : null}
              {flatProps.onRetryFailedItem ? (
                <button
                  disabled={Boolean(flatProps.retryingFailedItemId)}
                  onClick={() => flatProps.onRetryFailedItem?.(item.id)}
                  type="button"
                >
                  retry-item-{item.id}
                </button>
              ) : null}
            </div>
          ))}
          {flatProps.retryingFailedItemId ? (
            <div>retrying-item: {flatProps.retryingFailedItemId}</div>
          ) : null}
        </div>
      );
    },
  }),
);

vi.mock("@/lib/api/shein-studio", () => ({
  analyzeSheinStudioReferenceStyle: (...args: unknown[]) =>
    analyzeSheinStudioReferenceStyle(...args),
  generateSheinStudioDesigns: (...args: unknown[]) =>
    generateSheinStudioDesigns(...args),
}));

vi.mock("@/lib/shein-studio/create-review-tasks", async () => {
  const actual = await vi.importActual<
    typeof import("@/lib/shein-studio/create-review-tasks")
  >("@/lib/shein-studio/create-review-tasks");
  return {
    ...actual,
    createSheinReviewTasks: (...args: unknown[]) =>
      createSheinReviewTasks(...args),
  };
});

vi.mock("@/lib/shein-studio/hydrate-sds-selection", () => ({
  hydrateSDSVariantSelection: (...args: unknown[]) =>
    hydrateSDSVariantSelection(...args),
}));

vi.mock("@/lib/api/sds-baseline", () => ({
  getSDSBaselineReadiness: (...args: unknown[]) =>
    getSDSBaselineReadiness(...args),
  warmSDSBaselineForSelection: (...args: unknown[]) =>
    warmSDSBaselineForSelection(...args),
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
  startSheinStudioBatchRun: (...args: unknown[]) =>
    startSheinStudioBatchRun(...args),
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  deleteSheinStudioBatch: (...args: unknown[]) =>
    deleteSheinStudioBatch(...args),
  getSheinStudioBatch: (...args: unknown[]) => getSheinStudioBatch(...args),
  getSheinStudioHydratedBatch: (...args: unknown[]) =>
    getSheinStudioHydratedBatch(...args),
  listSheinStudioBatches: (...args: unknown[]) =>
    listSheinStudioBatches(...args),
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

export const selection = {
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

export const groupedSelection = {
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

export function buildHydratedBatch(
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

export function buildReviewReadyHydratedBatchDetail(
  batchId = "batch-1",
  updatedAt = "2026-05-26T10:05:00.000Z",
) {
  const { detail } = buildHydratedBatch();
  return {
    ...detail,
    batch: {
      ...detail.batch,
      id: batchId,
      updatedAt,
    },
    items: [
      {
        item: {
          id: "item-1",
          batchId,
          targetGroupKey: "size:1000x1000",
          status: "review_ready",
          selectionCount: 1,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt,
        },
        designs: [
          {
            id: "design-1",
            batchId,
            itemId: "item-1",
            sourceAttemptId: "attempt-1",
            targetGroupKey: "size:1000x1000",
            imageUrl: "https://example.com/design.png",
            reviewStatus: "approved",
            createdAt: "2026-05-26T10:04:00.000Z",
            updatedAt,
          },
        ],
      },
    ],
  };
}

export function buildDraft(overrides: Record<string, unknown> = {}) {
  return {
    prompt: "retro cherries",
    styleCount: "1",
    productImageCount: "5",
    productImagePrompt: "",
    productImagePrompts: [],
    hotStyleReferenceImageUrls: [],
    hotStyleReferenceBrief: "",
    hotStyleReferencePrompt: "",
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
    ...overrides,
  };
}

export function createDeferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, resolve, reject };
}

export function resetSheinStudioWorkbenchHarness(
  resetDedicatedBatchPromptOverrides: () => void,
) {
  vi.restoreAllMocks();
  window.localStorage.clear();
  resetDedicatedBatchPromptOverrides();
  lastGenerationPanelProps = null;
  useQuery.mockReturnValue({ data: undefined, error: null });
  analyzeSheinStudioReferenceStyle.mockReset();
  generateSheinStudioDesigns.mockReset();
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
  startSheinStudioBatchRun.mockResolvedValue({
    run: {
      id: "run-default",
    },
    items: [],
  });
  deleteSheinStudioBatch.mockResolvedValue(undefined);
  approveSheinStudioBatchDesigns.mockReset();
  createSheinStudioBatchTasks.mockReset();
  push.mockReset();
}
