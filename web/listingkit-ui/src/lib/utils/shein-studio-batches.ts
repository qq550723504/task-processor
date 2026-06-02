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
  dedupeGeneratedDesignsByID,
  normalizeDraft,
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
const LOCAL_DRAFT_SNAPSHOT_KEY = "listingkit:shein-studio:recent-draft";

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

type SaveBatchOptions = {
  makeActive?: boolean;
};

type SaveDraftOptions = {
  navigationTriggered?: boolean;
  source?: string;
  signal?: AbortSignal;
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
  });
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

export async function saveSheinStudioDraftWithOptions(
  input: SheinStudioSaveInput,
  _options?: SaveDraftOptions,
) {
  return enqueueSheinStudioSave(buildSheinStudioSaveQueueKey(input), async () => {
    if (!input.selection?.variantId) {
      return null;
    }
    return saveLocalDraftSnapshot(input) ?? null;
  });
}

export async function listSheinStudioBatches() {
  return (await listSheinStudioBatchDrafts())
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
    const buildUpsertInput = (expectedUpdatedAt?: string) => ({
      id: input.id,
      expectedUpdatedAt,
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

function mergeBatchDetailWithSavedBatchContext(
  detail: SheinStudioBatchDetail,
  savedBatch?: SheinStudioSavedBatch,
): SheinStudioSavedBatch {
  const designs = flattenSheinStudioBatchDetailDesigns(detail);
  const selectedIDs = getApprovedSheinStudioBatchDesignIDs(detail);
  const detailStoreID =
    detail.batch.sheinStoreId > 0 ? String(detail.batch.sheinStoreId) : "";
  const detailPrompt = detail.batch.prompt.trim();
  const detailStyleCount = detail.batch.styleCount.trim();
  const detailSelection = detail.batch.selection;
  const detailGroupedSelections = detail.batch.groupedSelections ?? [];
  const detailSelectedSdsImages = detail.batch.selectedSdsImages ?? [];
  return {
    id: detail.batch.id,
    name: savedBatch?.name ?? deriveBatchName(detail.batch.prompt),
    prompt: detailPrompt || savedBatch?.prompt || "",
    styleCount: detailStyleCount || savedBatch?.styleCount || "1",
    variationIntensity:
      detail.batch.variationIntensity ?? savedBatch?.variationIntensity,
    productImageCount: savedBatch?.productImageCount,
    productImagePrompt: savedBatch?.productImagePrompt,
    productImagePrompts: savedBatch?.productImagePrompts,
    artworkModel: detail.batch.artworkModel || savedBatch?.artworkModel,
    transparentBackground:
      detail.batch.transparentBackground ?? savedBatch?.transparentBackground,
    sheinStoreId: detailStoreID || savedBatch?.sheinStoreId || "",
    imageStrategy: savedBatch?.imageStrategy,
    groupedImageMode:
      detail.batch.groupedImageMode ?? savedBatch?.groupedImageMode,
    selectedSdsImages:
      detailSelectedSdsImages.length > 0
        ? detailSelectedSdsImages
        : (savedBatch?.selectedSdsImages ?? []),
    renderSizeImagesWithSds: savedBatch?.renderSizeImagesWithSds,
    selectionVariantId:
      detail.batch.selectionVariantId ??
      detailSelection?.variantId ??
      savedBatch?.selectionVariantId ??
      savedBatch?.selection?.variantId,
    selection: detailSelection ?? savedBatch?.selection,
    groupedSelections:
      detailGroupedSelections.length > 0
        ? detailGroupedSelections
        : (savedBatch?.groupedSelections ?? []),
    groups: savedBatch?.groups ?? [],
    designs,
    selectedIds: selectedIDs,
    createdTasks: savedBatch?.createdTasks ?? [],
    generationJobs: savedBatch?.generationJobs ?? [],
    generationError: savedBatch?.generationError,
    generationJobId: savedBatch?.generationJobId,
    batchStatus: detail.batch.status,
    draftUpdatedAt:
      detail.batch.draftUpdatedAt ||
      savedBatch?.draftUpdatedAt ||
      savedBatch?.updatedAt ||
      detail.batch.updatedAt,
    updatedAt:
      detail.batch.updatedAt ||
      savedBatch?.updatedAt ||
      new Date().toISOString(),
  };
}

function deriveBatchName(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return "未命名批次";
  }
  return trimmed.length > 36 ? `${trimmed.slice(0, 36)}...` : trimmed;
}

