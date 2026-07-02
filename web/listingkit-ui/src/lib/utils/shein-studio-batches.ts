import {
  buildStudioBatchDraftSelectionKey,
  deleteSheinStudioBatchDraft,
  listSheinStudioBatchDrafts,
  upsertSheinStudioBatchDraft,
} from "@/lib/api/shein-studio-batch-drafts";
import { ApiError } from "@/lib/api/client";
import { getSheinStudioBatchDetail } from "@/lib/api/shein-studio-batches";
import {
  normalizeBatch,
  normalizeDraft,
} from "@/lib/shein-studio/storage-shared";
import { enqueueSheinStudioSave } from "@/lib/shein-studio/save-queue";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioBatchDetail,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioGeneratedDesign,
  SheinStudioLegacyCompatibilitySnapshot,
  SheinStudioPersistedBatchView,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

const ACTIVE_BATCH_STORAGE_KEY = "listingkit:shein-studio:active-batch-id";
const LOCAL_DRAFT_SNAPSHOT_KEY = "listingkit:shein-studio:recent-draft";

export type SheinStudioSaveInput = Omit<
  SheinStudioPersistedBatchView,
  "updatedAt"
> & {
  id?: string;
  name?: string;
  updatedAt?: string;
  designs?: SheinStudioGeneratedDesign[];
  selectedIds?: string[];
  createdTasks?: SheinStudioCreatedTask[];
  generationJobs?: SheinStudioGenerationJob[];
};

type SaveBatchOptions = {
  makeActive?: boolean;
};

export type SheinStudioHydratedBatch = {
  savedBatch: SheinStudioSavedBatch;
  detail: SheinStudioBatchDetail;
};

type SheinStudioBatchListOptions = {
  limit?: number;
};

const inFlightBatchListPromises = new Map<string, Promise<SheinStudioSavedBatch[]>>();
const inFlightHydratedBatchPromises = new Map<
  string,
  Promise<SheinStudioHydratedBatch>
>();

function buildSheinStudioSaveQueueKey(input: SheinStudioSaveInput) {
  const batchID = input.id?.trim();
  if (batchID) {
    return `batch:${batchID}`;
  }
  const selectionKey = buildStudioBatchDraftSelectionKey(input.selection);
  if (selectionKey) {
    return `selection:${selectionKey}`;
  }
  return "studio:default";
}

function canUseBatchStorage() {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

function loadLocalDraftSnapshot() {
  if (!canUseBatchStorage()) {
    return null;
  }
  const raw = window.localStorage.getItem(LOCAL_DRAFT_SNAPSHOT_KEY);
  if (!raw) {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as {
      batchId?: unknown;
      draft?: unknown;
    };
    const draft = normalizeDraft(
      parsed && typeof parsed === "object" && "draft" in parsed
        ? (parsed.draft as Record<string, unknown> | null | undefined)
        : undefined,
    );
    if (!draft) {
      return null;
    }
    return {
      batchId: typeof parsed?.batchId === "string" ? parsed.batchId : "",
      draft,
    };
  } catch {
    return null;
  }
}

function saveLocalDraftSnapshot(input: SheinStudioSaveInput, batchId?: string) {
  if (!canUseBatchStorage()) {
    return null;
  }
  const draft = normalizeDraft({
    ...input,
    updatedAt: input.updatedAt || new Date().toISOString(),
  } as Partial<import("@/lib/types/shein-studio").SheinStudioDraft>);
  if (!draft) {
    return null;
  }
  window.localStorage.setItem(
    LOCAL_DRAFT_SNAPSHOT_KEY,
    JSON.stringify({
      batchId: batchId?.trim() || undefined,
      draft,
    }),
  );
  return draft;
}

function resolveLegacyCompatibilitySnapshot(
  input: SheinStudioSaveInput,
): SheinStudioLegacyCompatibilitySnapshot | undefined {
  if (input.legacyCompatibilitySnapshot) {
    return input.legacyCompatibilitySnapshot;
  }

  const hasDesigns = (input.designs?.length ?? 0) > 0;
  const hasSelectedIds = (input.selectedIds?.length ?? 0) > 0;
  const hasCreatedTasks = (input.createdTasks?.length ?? 0) > 0;
  const hasGenerationJobs = (input.generationJobs?.length ?? 0) > 0;
  if (
    !hasDesigns &&
    !hasSelectedIds &&
    !hasCreatedTasks &&
    !hasGenerationJobs &&
    !input.generationError &&
    !input.generationJobId
  ) {
    return undefined;
  }

  return {
    designs: input.designs ?? [],
    selectedIds: input.selectedIds ?? [],
    createdTasks: input.createdTasks ?? [],
    generationJobs: input.generationJobs ?? [],
    generationError: input.generationError,
    generationJobId: input.generationJobId,
  };
}

export function getActiveSheinStudioBatchId() {
  if (!canUseBatchStorage()) {
    return "";
  }
  return window.localStorage.getItem(ACTIVE_BATCH_STORAGE_KEY) ?? "";
}

export function setActiveSheinStudioBatchId(batchId: string) {
  if (!canUseBatchStorage()) {
    return;
  }
  if (!batchId.trim()) {
    window.localStorage.removeItem(ACTIVE_BATCH_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(ACTIVE_BATCH_STORAGE_KEY, batchId);
}

export async function loadSheinStudioDraft(selection?: SDSProductVariantSelection) {
  if (!selection?.variantId) {
    return null;
  }
  const snapshot = loadLocalDraftSnapshot();
  if (!snapshot?.draft?.selection?.variantId) {
    return null;
  }
  return buildStudioBatchDraftSelectionKey(snapshot.draft.selection) ===
    buildStudioBatchDraftSelectionKey(selection)
    ? snapshot.draft
    : null;
}

export async function saveSheinStudioDraft(input: SheinStudioSaveInput) {
  return saveSheinStudioDraftWithOptions(input);
}

export async function saveSheinStudioDraftWithOptions(input: SheinStudioSaveInput) {
  return enqueueSheinStudioSave(buildSheinStudioSaveQueueKey(input), async () => {
    if (!input.selection?.variantId) {
      return null;
    }
    return saveLocalDraftSnapshot(input) ?? null;
  });
}

function buildBatchListCacheKey(options?: SheinStudioBatchListOptions) {
  const limit =
    typeof options?.limit === "number" && options.limit > 0
      ? Math.floor(options.limit)
      : 0;
  return `limit:${limit}`;
}

export async function listSheinStudioBatches(options?: SheinStudioBatchListOptions) {
  const cacheKey = buildBatchListCacheKey(options);
  let pending = inFlightBatchListPromises.get(cacheKey);
  if (!pending) {
    pending = listSheinStudioBatchDrafts({
      limit: options?.limit,
    })
      .then((items) =>
        items
          .map((item) => normalizeBatch(item))
          .filter((item): item is NonNullable<typeof item> => Boolean(item)),
      )
      .finally(() => {
        inFlightBatchListPromises.delete(cacheKey);
      });
    inFlightBatchListPromises.set(cacheKey, pending);
  }
  return pending;
}

export async function getSheinStudioBatch(batchID: string) {
  return (await getSheinStudioHydratedBatch(batchID)).savedBatch;
}

export async function getSheinStudioHydratedBatch(
  batchID: string,
): Promise<SheinStudioHydratedBatch> {
  const normalizedBatchID = batchID.trim();
  if (!normalizedBatchID) {
    throw new Error("Saved batch context unavailable for an empty batch id.");
  }
  const inFlight = inFlightHydratedBatchPromises.get(normalizedBatchID);
  if (inFlight) {
    return inFlight;
  }
  const pending = (async () => {
    const savedBatch = (await listSheinStudioBatches()).find(
      (item) => item.id === normalizedBatchID,
    );
    if (!savedBatch) {
      throw new Error(
        `Saved batch context unavailable for ${normalizedBatchID}.`,
      );
    }
    const detail = await getSheinStudioBatchDetail(normalizedBatchID, {
      tenantId: savedBatch.tenantId,
    });
    return {
      savedBatch: normalizeBatch(
        mergeBatchDetailWithSavedBatchContext(detail, savedBatch),
      )!,
      detail,
    };
  })().finally(() => {
    inFlightHydratedBatchPromises.delete(normalizedBatchID);
  });
  inFlightHydratedBatchPromises.set(normalizedBatchID, pending);
  return pending;
}

export async function saveSheinStudioBatch(
  input: SheinStudioSaveInput,
  options?: SaveBatchOptions,
) {
  return enqueueSheinStudioSave(buildSheinStudioSaveQueueKey(input), async () => {
    const buildUpsertInput = (expectedUpdatedAt?: string) => ({
      id: input.id,
      expectedUpdatedAt,
      name: input.name?.trim() || undefined,
      prompt: input.prompt,
      promptMode: input.promptMode,
      styleCount: input.styleCount,
      variationIntensity: input.variationIntensity,
      productImageCount: input.productImageCount,
      productImagePrompt: input.productImagePrompt,
      productImagePrompts: input.productImagePrompts,
      artworkModel: input.artworkModel,
      imageStrategy: input.imageStrategy,
      groupedImageMode: input.groupedImageMode,
      selectedSdsImages: input.selectedSdsImages,
      transparentBackground: input.transparentBackground,
      renderSizeImagesWithSds: input.renderSizeImagesWithSds,
      hotStyleReferenceImageUrls: input.hotStyleReferenceImageUrls,
      hotStyleReferenceBrief: input.hotStyleReferenceBrief,
      hotStyleReferencePrompt: input.hotStyleReferencePrompt,
      sheinStoreId: input.sheinStoreId,
      selection: input.selection,
      legacyCompatibilitySnapshot: resolveLegacyCompatibilitySnapshot(input),
      groups: input.groups,
      groupedSelections: input.groupedSelections,
    });
    const saveBatch = async (expectedUpdatedAt?: string) =>
      normalizeBatch(await upsertSheinStudioBatchDraft(buildUpsertInput(expectedUpdatedAt)));

    let saved: ReturnType<typeof normalizeBatch>;
    try {
      saved = await saveBatch(input.updatedAt);
    } catch (error) {
      const batchID = input.id?.trim();
      if (!batchID || !shouldRetryStudioBatchSaveConflict(error, input)) {
        throw error;
      }
      const latestBatch = await getSheinStudioHydratedBatch(batchID);
      saved = await saveBatch(
        latestBatch.detail.batch.draftUpdatedAt ||
          latestBatch.savedBatch.draftUpdatedAt ||
          latestBatch.savedBatch.updatedAt,
      );
    }
    if (saved?.id && options?.makeActive !== false) {
      setActiveSheinStudioBatchId(saved.id);
    }
    return saved;
  });
}

function shouldRetryStudioBatchSaveConflict(
  error: unknown,
  input: SheinStudioSaveInput,
) {
  const batchID = input.id?.trim();
  if (!batchID) {
    return false;
  }
  return error instanceof ApiError && error.status === 409;
}

export async function deleteSheinStudioBatch(batchID: string) {
  await deleteSheinStudioBatchDraft(batchID);
  if (getActiveSheinStudioBatchId() === batchID) {
    setActiveSheinStudioBatchId("");
  }
}

export function flattenSheinStudioBatchDetailDesigns(
  detail: SheinStudioBatchDetail,
): SheinStudioGeneratedDesign[] {
  return detail.items.flatMap((entry) =>
    entry.designs.map(
      (design) =>
        ({
          id: design.id,
          imageUrl: design.imageUrl,
          prompt: detail.batch.prompt,
          reviewNote: design.reviewNote,
          role: design.role,
          roleLabel: design.roleLabel,
          targetGroupKey: design.targetGroupKey,
          targetGroupLabel: design.targetGroupLabel,
          productImageUrls: design.productImageUrls,
        }) satisfies SheinStudioGeneratedDesign,
    ),
  );
}

export function getApprovedSheinStudioBatchDesignIDs(
  detail: SheinStudioBatchDetail,
) {
  return detail.items.flatMap((entry) =>
    entry.designs
      .filter((design) => design.reviewStatus === "approved")
      .map((design) => design.id),
  );
}

export function updateSheinStudioBatchDetailReviewNote(
  detail: SheinStudioBatchDetail,
  designID: string,
  note: string,
): SheinStudioBatchDetail {
  return {
    ...detail,
    items: detail.items.map((entry) => ({
      ...entry,
      designs: entry.designs.map((design) =>
        design.id === designID ? { ...design, reviewNote: note } : design,
      ),
    })),
  };
}

function projectItemizedBatchSavedBatchCompatibility(
  detail: SheinStudioBatchDetail,
) {
  return {
    prompt: detail.batch.prompt.trim(),
    promptMode: detail.batch.promptMode ?? "managed",
    styleCount: detail.batch.styleCount.trim(),
    variationIntensity: detail.batch.variationIntensity,
    artworkModel: detail.batch.artworkModel,
    transparentBackground: detail.batch.transparentBackground,
    sheinStoreId:
      detail.batch.sheinStoreId > 0 ? String(detail.batch.sheinStoreId) : "",
    groupedImageMode: detail.batch.groupedImageMode,
    selectionVariantId: detail.batch.selectionVariantId,
    selection: detail.batch.selection,
    groupedSelections: detail.batch.groupedSelections ?? [],
    selectedSdsImages: detail.batch.selectedSdsImages ?? [],
    designs: flattenSheinStudioBatchDetailDesigns(detail),
    selectedIds: getApprovedSheinStudioBatchDesignIDs(detail),
    batchStatus: detail.batch.status,
    draftUpdatedAt: detail.batch.draftUpdatedAt,
    updatedAt: detail.batch.updatedAt,
  };
}

function mergeBatchDetailWithSavedBatchContext(
  detail: SheinStudioBatchDetail,
  savedBatch?: SheinStudioSavedBatch,
): SheinStudioSavedBatch {
  const itemized = projectItemizedBatchSavedBatchCompatibility(detail);
  const detailCreatedTasks = detail.createdTasks ?? [];
  return {
    id: detail.batch.id,
    tenantId: detail.batch.tenantId ?? savedBatch?.tenantId,
    name: savedBatch?.name ?? deriveBatchName(detail.batch.prompt),
    prompt: itemized.prompt || savedBatch?.prompt || "",
    styleCount: itemized.styleCount || savedBatch?.styleCount || "1",
    variationIntensity: itemized.variationIntensity ?? savedBatch?.variationIntensity,
    productImageCount: savedBatch?.productImageCount,
    productImagePrompt: savedBatch?.productImagePrompt,
    productImagePrompts: savedBatch?.productImagePrompts,
    artworkModel: itemized.artworkModel || savedBatch?.artworkModel,
    transparentBackground: itemized.transparentBackground ?? savedBatch?.transparentBackground,
    sheinStoreId: itemized.sheinStoreId || savedBatch?.sheinStoreId || "",
    imageStrategy: savedBatch?.imageStrategy,
    groupedImageMode: itemized.groupedImageMode ?? savedBatch?.groupedImageMode,
    selectedSdsImages:
      itemized.selectedSdsImages.length > 0
        ? itemized.selectedSdsImages
        : (savedBatch?.selectedSdsImages ?? []),
    renderSizeImagesWithSds: savedBatch?.renderSizeImagesWithSds,
    selectionVariantId:
      itemized.selectionVariantId ??
      itemized.selection?.variantId ??
      savedBatch?.selectionVariantId ??
      savedBatch?.selection?.variantId,
    selection: itemized.selection ?? savedBatch?.selection,
    groupedSelections:
      itemized.groupedSelections.length > 0
        ? itemized.groupedSelections
        : (savedBatch?.groupedSelections ?? []),
    groups: savedBatch?.groups ?? [],
    designs: itemized.designs,
    selectedIds: itemized.selectedIds,
    createdTasks:
      detailCreatedTasks.length > 0
        ? detailCreatedTasks
        : (savedBatch?.createdTasks ?? []),
    generationJobs: savedBatch?.generationJobs ?? [],
    generationError: savedBatch?.generationError,
    generationJobId: savedBatch?.generationJobId,
    batchStatus: itemized.batchStatus,
    draftUpdatedAt:
      itemized.draftUpdatedAt ||
      savedBatch?.draftUpdatedAt ||
      savedBatch?.updatedAt ||
      itemized.updatedAt,
    updatedAt: itemized.updatedAt || savedBatch?.updatedAt || new Date().toISOString(),
  };
}

function deriveBatchName(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return "未命名批次";
  }
  return trimmed.length > 36 ? `${trimmed.slice(0, 36)}...` : trimmed;
}
