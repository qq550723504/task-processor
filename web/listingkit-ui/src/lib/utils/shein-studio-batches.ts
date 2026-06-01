import {
  buildStudioSessionSelectionKey,
  deleteSheinStudioSessionBatch,
  ensureSheinStudioSession,
  getCachedStudioSessionId,
  listSheinStudioSessionBatches,
  mapStudioSessionDetailToDraft,
  replaceSheinStudioSessionDesigns,
  upsertSheinStudioSessionBatch,
  updateSheinStudioSession,
} from "@/lib/api/shein-studio-sessions";
import { getSheinStudioBatchDetail } from "@/lib/api/shein-studio-batches";
import {
  normalizeBatch,
  dedupeGeneratedDesignsByID,
} from "@/lib/shein-studio/storage-shared";
import { enqueueSheinStudioSave } from "@/lib/shein-studio/save-queue";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioGroupedWorkspace,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioArtworkModel,
  SheinStudioGeneratedDesign,
  SheinStudioBatchDetail,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

const ACTIVE_BATCH_STORAGE_KEY = "listingkit:shein-studio:active-batch-id";

export type SheinStudioSaveInput = {
  id?: string;
  updatedAt?: string;
  name?: string;
  prompt: string;
  styleCount: string;
  variationIntensity?: SheinStudioVariationIntensity;
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  artworkModel?: SheinStudioArtworkModel;
  transparentBackground?: boolean;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  groupedImageMode?: SheinStudioGroupedImageMode;
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds?: boolean;
  selection?: SDSProductVariantSelection;
  groupedSelections?: GroupedSDSSelectionEligibility[];
  groups?: SheinStudioGroupedWorkspace[];
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  generationJobs?: SheinStudioGenerationJob[];
};

type SaveDraftOptions = {
  navigationTriggered?: boolean;
  source?: string;
  signal?: AbortSignal;
};

type SaveBatchOptions = {
  makeActive?: boolean;
};

export type SheinStudioHydratedBatch = {
  savedBatch: SheinStudioSavedBatch;
  detail: SheinStudioBatchDetail;
};

function buildSheinStudioSaveQueueKey(input: SheinStudioSaveInput) {
  const batchID = input.id?.trim();
  if (batchID) {
    return `batch:${batchID}`;
  }
  const selectionKey = buildStudioSessionSelectionKey(input.selection);
  if (selectionKey) {
    return `selection:${selectionKey}`;
  }
  return "studio:default";
}

function canUseBatchStorage() {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
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

  const detail = await ensureSheinStudioSession(selection);
  return mapStudioSessionDetailToDraft(detail);
}

export async function saveSheinStudioDraft(input: SheinStudioSaveInput) {
  return saveSheinStudioDraftWithOptions(input);
}

export async function saveSheinStudioDraftWithOptions(
  input: SheinStudioSaveInput,
  options?: SaveDraftOptions,
) {
  return enqueueSheinStudioSave(buildSheinStudioSaveQueueKey(input), async () => {
  const startedAt = performance.now();
  const normalizedDesigns = dedupeGeneratedDesignsByID(input.designs);
  const normalizedInput: SheinStudioSaveInput = {
    ...input,
    designs: normalizedDesigns,
  };
  const body = JSON.stringify(normalizedInput);
  const bodyBytes = new TextEncoder().encode(body).byteLength;
  const source = options?.source ?? "unknown";

  console.info("[shein-studio-draft] client save started", {
    bodyBytes,
    designCount: normalizedInput.designs.length,
    navigationTriggered: options?.navigationTriggered ?? false,
    selectionVariantId: input.selection?.variantId ?? null,
    source,
  });

  if (!normalizedInput.selection?.variantId) {
    return null;
  }
  try {
    const sessionId =
      getCachedStudioSessionId(normalizedInput.selection) ??
      (await ensureSheinStudioSession(normalizedInput.selection, {
        signal: options?.signal,
      }))?.session?.id;
    if (!sessionId) {
      throw new Error("SHEIN Studio session was not initialized");
    }

    const status =
      (normalizedInput.generationJobs?.length ?? 0) > 0
        ? "generating"
        : normalizedInput.createdTasks.length > 0
        ? "tasks_created"
        : normalizedInput.designs.length > 0
          ? "reviewing"
          : "selecting";

    const detail = await updateSheinStudioSession(sessionId, {
      status,
      expectedUpdatedAt: normalizedInput.updatedAt,
      prompt: normalizedInput.prompt,
      styleCount: normalizedInput.styleCount,
      variationIntensity: normalizedInput.variationIntensity,
      productImageCount: normalizedInput.productImageCount,
      productImagePrompt: normalizedInput.productImagePrompt,
      productImagePrompts: normalizedInput.productImagePrompts,
      artworkModel: normalizedInput.artworkModel,
      imageStrategy: normalizedInput.imageStrategy,
      groupedImageMode: normalizedInput.groupedImageMode,
      selectedSdsImages: normalizedInput.selectedSdsImages,
      transparentBackground: normalizedInput.transparentBackground,
      renderSizeImagesWithSds: normalizedInput.renderSizeImagesWithSds,
      sheinStoreId: normalizedInput.sheinStoreId,
      approvedDesignIds: normalizedInput.selectedIds,
      createdTasks: normalizedInput.createdTasks,
      generationJobs: normalizedInput.generationJobs,
      groups: normalizedInput.groups,
      groupedSelections: normalizedInput.groupedSelections,
    }, {
      signal: options?.signal,
    });

    const synced =
      normalizedInput.designs.length > 0 || normalizedInput.selectedIds.length > 0 || normalizedInput.createdTasks.length > 0
        ? await replaceSheinStudioSessionDesigns(sessionId, {
            expectedUpdatedAt: detail.session?.updated_at,
            status,
            approvedDesignIds: normalizedInput.selectedIds,
            designs: normalizedInput.designs,
          }, {
            signal: options?.signal,
          })
        : detail;
    console.info("[shein-studio-draft] client save completed", {
      bodyBytes,
      designCount: normalizedInput.designs.length,
      draftSaveDurationMs: Math.round(performance.now() - startedAt),
      draftSaveStatus: "succeeded",
      navigationTriggered: options?.navigationTriggered ?? false,
      selectionVariantId: normalizedInput.selection?.variantId ?? null,
      source,
    });
    return mapStudioSessionDetailToDraft(synced);
  } catch (error) {
    console.warn("[shein-studio-draft] client save failed", {
      bodyBytes,
      designCount: normalizedInput.designs.length,
      draftSaveDurationMs: Math.round(performance.now() - startedAt),
      draftSaveStatus: "failed",
      error: error instanceof Error ? error.message : String(error),
      navigationTriggered: options?.navigationTriggered ?? false,
      selectionVariantId: normalizedInput.selection?.variantId ?? null,
      source,
    });
    throw error;
  }
  });
}

export async function listSheinStudioBatches() {
  return (await listSheinStudioSessionBatches())
    .map((item) => normalizeBatch(item))
    .filter((item): item is NonNullable<typeof item> => Boolean(item));
}

export async function getSheinStudioBatch(batchID: string) {
  return (await getSheinStudioHydratedBatch(batchID)).savedBatch;
}

export async function getSheinStudioHydratedBatch(
  batchID: string,
): Promise<SheinStudioHydratedBatch> {
  const detail = await getSheinStudioBatchDetail(batchID);
  const savedBatch = (await listSheinStudioBatches()).find(
    (item) => item.id === batchID,
  );
  if (!savedBatch) {
    throw new Error(`Saved batch context unavailable for ${batchID}.`);
  }
  return {
    savedBatch: normalizeBatch(
      mergeBatchDetailWithSavedBatchContext(detail, savedBatch),
    )!,
    detail,
  };
}

export async function saveSheinStudioBatch(
  input: SheinStudioSaveInput,
  options?: SaveBatchOptions,
) {
  return enqueueSheinStudioSave(buildSheinStudioSaveQueueKey(input), async () => {
    const saved = normalizeBatch(
      await upsertSheinStudioSessionBatch({
        id: input.id,
        expectedUpdatedAt: input.updatedAt,
        name: input.name?.trim() || undefined,
        prompt: input.prompt,
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
        sheinStoreId: input.sheinStoreId,
        selection: input.selection,
        groups: input.groups,
        groupedSelections: input.groupedSelections,
        approvedDesignIds: input.selectedIds,
        createdTasks: input.createdTasks,
        generationJobs: input.generationJobs,
        designs: dedupeGeneratedDesignsByID(input.designs),
      }),
    );
    if (saved?.id && options?.makeActive !== false) {
      setActiveSheinStudioBatchId(saved.id);
    }
    return saved;
  });
}

export async function deleteSheinStudioBatch(batchID: string) {
  await deleteSheinStudioSessionBatch(batchID);
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

function mergeBatchDetailWithSavedBatchContext(
  detail: SheinStudioBatchDetail,
  savedBatch?: SheinStudioSavedBatch,
): SheinStudioSavedBatch {
  const designs = flattenSheinStudioBatchDetailDesigns(detail);
  const selectedIDs = getApprovedSheinStudioBatchDesignIDs(detail);
  const detailStoreID =
    detail.batch.sheinStoreId > 0 ? String(detail.batch.sheinStoreId) : "";
  return {
    id: detail.batch.id,
    name: savedBatch?.name ?? deriveBatchName(detail.batch.prompt),
    prompt: detail.batch.prompt,
    styleCount: detail.batch.styleCount,
    variationIntensity: savedBatch?.variationIntensity,
    productImageCount: savedBatch?.productImageCount,
    productImagePrompt: savedBatch?.productImagePrompt,
    productImagePrompts: savedBatch?.productImagePrompts,
    artworkModel: savedBatch?.artworkModel,
    transparentBackground: savedBatch?.transparentBackground,
    sheinStoreId: savedBatch?.sheinStoreId || detailStoreID,
    imageStrategy: savedBatch?.imageStrategy,
    groupedImageMode: savedBatch?.groupedImageMode,
    selectedSdsImages: savedBatch?.selectedSdsImages,
    renderSizeImagesWithSds: savedBatch?.renderSizeImagesWithSds,
    selectionVariantId:
      savedBatch?.selectionVariantId ?? savedBatch?.selection?.variantId,
    selection: savedBatch?.selection,
    groupedSelections: savedBatch?.groupedSelections ?? [],
    groups: savedBatch?.groups ?? [],
    designs,
    selectedIds: selectedIDs,
    createdTasks: savedBatch?.createdTasks ?? [],
    generationJobs: savedBatch?.generationJobs ?? [],
    generationError: savedBatch?.generationError,
    generationJobId: savedBatch?.generationJobId,
    sessionStatus: detail.batch.status,
    updatedAt: detail.batch.updatedAt,
  };
}

function deriveBatchName(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return "未命名批次";
  }
  return trimmed.length > 36 ? `${trimmed.slice(0, 36)}...` : trimmed;
}
