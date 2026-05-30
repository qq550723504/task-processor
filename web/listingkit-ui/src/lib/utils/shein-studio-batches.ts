import {
  deleteSheinStudioSessionBatch,
  ensureSheinStudioSession,
  getCachedStudioSessionId,
  getSheinStudioSessionBatch,
  listSheinStudioSessionBatches,
  mapStudioSessionDetailToDraft,
  replaceSheinStudioSessionDesigns,
  upsertSheinStudioSessionBatch,
  updateSheinStudioSession,
} from "@/lib/api/shein-studio-sessions";
import {
  normalizeBatch,
} from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioGroupedWorkspace,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioArtworkModel,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

const ACTIVE_BATCH_STORAGE_KEY = "listingkit:shein-studio:active-batch-id";

export type SheinStudioSaveInput = {
  id?: string;
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
  const startedAt = performance.now();
  const body = JSON.stringify(input);
  const bodyBytes = new TextEncoder().encode(body).byteLength;
  const source = options?.source ?? "unknown";

  console.info("[shein-studio-draft] client save started", {
    bodyBytes,
    designCount: input.designs.length,
    navigationTriggered: options?.navigationTriggered ?? false,
    selectionVariantId: input.selection?.variantId ?? null,
    source,
  });

  if (!input.selection?.variantId) {
    return null;
  }
  try {
    const sessionId =
      getCachedStudioSessionId(input.selection) ??
      (await ensureSheinStudioSession(input.selection, {
        signal: options?.signal,
      }))?.session?.id;
    if (!sessionId) {
      throw new Error("SHEIN Studio session was not initialized");
    }

    const status =
      (input.generationJobs?.length ?? 0) > 0
        ? "generating"
        : input.createdTasks.length > 0
        ? "tasks_created"
        : input.designs.length > 0
          ? "reviewing"
          : "selecting";

    const detail = await updateSheinStudioSession(sessionId, {
      status,
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
      approvedDesignIds: input.selectedIds,
      createdTasks: input.createdTasks,
      generationJobs: input.generationJobs,
      groups: input.groups,
      groupedSelections: input.groupedSelections,
    }, {
      signal: options?.signal,
    });

    const synced =
      input.designs.length > 0 || input.selectedIds.length > 0 || input.createdTasks.length > 0
        ? await replaceSheinStudioSessionDesigns(sessionId, {
            status,
            approvedDesignIds: input.selectedIds,
            designs: input.designs,
          }, {
            signal: options?.signal,
          })
        : detail;
    console.info("[shein-studio-draft] client save completed", {
      bodyBytes,
      designCount: input.designs.length,
      draftSaveDurationMs: Math.round(performance.now() - startedAt),
      draftSaveStatus: "succeeded",
      navigationTriggered: options?.navigationTriggered ?? false,
      selectionVariantId: input.selection?.variantId ?? null,
      source,
    });
    return mapStudioSessionDetailToDraft(synced);
  } catch (error) {
    console.warn("[shein-studio-draft] client save failed", {
      bodyBytes,
      designCount: input.designs.length,
      draftSaveDurationMs: Math.round(performance.now() - startedAt),
      draftSaveStatus: "failed",
      error: error instanceof Error ? error.message : String(error),
      navigationTriggered: options?.navigationTriggered ?? false,
      selectionVariantId: input.selection?.variantId ?? null,
      source,
    });
    throw error;
  }
}

export async function listSheinStudioBatches() {
  return (await listSheinStudioSessionBatches())
    .map((item) => normalizeBatch(item))
    .filter((item): item is NonNullable<typeof item> => Boolean(item));
}

export async function getSheinStudioBatch(batchID: string) {
  return normalizeBatch(await getSheinStudioSessionBatch(batchID));
}

export async function saveSheinStudioBatch(
  input: SheinStudioSaveInput,
  options?: SaveBatchOptions,
) {
  const saved = normalizeBatch(
    await upsertSheinStudioSessionBatch({
      id: input.id,
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
      designs: input.designs,
    }),
  );
  if (saved?.id && options?.makeActive !== false) {
    setActiveSheinStudioBatchId(saved.id);
  }
  return saved;
}

export async function deleteSheinStudioBatch(batchID: string) {
  await deleteSheinStudioSessionBatch(batchID);
  if (getActiveSheinStudioBatchId() === batchID) {
    setActiveSheinStudioBatchId("");
  }
}
