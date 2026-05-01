import {
  ensureSheinStudioSession,
  getCachedStudioSessionId,
  mapStudioSessionDetailToDraft,
  replaceSheinStudioSessionDesigns,
  updateSheinStudioSession,
} from "@/lib/api/shein-studio-sessions";
import {
  normalizeBatch,
} from "@/lib/shein-studio/storage-shared";
import {
  parseJsonResponse,
  ResponseJsonParseError,
} from "@/lib/api/response-json";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioArtworkModel,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioSavedBatch,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

export type SheinStudioSaveInput = {
  id?: string;
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
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds?: boolean;
  selection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
};

type SaveDraftOptions = {
  navigationTriggered?: boolean;
  source?: string;
  signal?: AbortSignal;
};

async function parseJSON<T>(response: Response) {
  let payload: (T & { message?: string }) | undefined;
  try {
    payload = await parseJsonResponse<T & { message?: string }>(response);
  } catch (error) {
    if (error instanceof ResponseJsonParseError) {
      throw new Error(error.message);
    }
    throw error;
  }
  if (!response.ok) {
    throw new Error(payload?.message || "SHEIN Studio storage request failed");
  }
  if (!payload) {
    throw new Error(`SHEIN Studio storage returned empty response: ${response.status}`);
  }
  return payload;
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
      input.createdTasks.length > 0
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
      selectedSdsImages: input.selectedSdsImages,
      transparentBackground: input.transparentBackground,
      renderSizeImagesWithSds: input.renderSizeImagesWithSds,
      sheinStoreId: input.sheinStoreId,
      approvedDesignIds: input.selectedIds,
      createdTasks: input.createdTasks,
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
  const response = await fetch("/api/shein-studio/batches", {
    cache: "no-store",
  });
  const payload = await parseJSON<{ batches: SheinStudioSavedBatch[] }>(response);
  return payload.batches
    .map((item) => normalizeBatch(item))
    .filter((item): item is NonNullable<typeof item> => Boolean(item));
}

export async function getSheinStudioBatch(batchID: string) {
  const response = await fetch(`/api/shein-studio/batches/${batchID}`, {
    cache: "no-store",
  });
  if (response.status === 404) {
    return null;
  }

  const payload = await parseJSON<{ batch: SheinStudioSavedBatch | null }>(response);
  return normalizeBatch(payload.batch);
}

export async function saveSheinStudioBatch(input: SheinStudioSaveInput) {
  const response = await fetch("/api/shein-studio/batches", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
    cache: "no-store",
  });
  const payload = await parseJSON<{ batch: SheinStudioSavedBatch | null }>(response);
  return normalizeBatch(payload.batch);
}

export async function deleteSheinStudioBatch(batchID: string) {
  const response = await fetch(`/api/shein-studio/batches/${batchID}`, {
    method: "DELETE",
    cache: "no-store",
  });
  await parseJSON<{ ok: boolean }>(response);
}
