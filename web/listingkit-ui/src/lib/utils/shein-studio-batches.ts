import {
  normalizeBatch,
  normalizeDraft,
} from "@/lib/shein-studio/storage-shared";
import {
  parseJsonResponse,
  ResponseJsonParseError,
} from "@/lib/api/response-json";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioArtworkModel,
  SheinStudioDraft,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

export type SheinStudioSaveInput = {
  id?: string;
  prompt: string;
  styleCount: string;
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  artworkModel?: SheinStudioArtworkModel;
  transparentBackground?: boolean;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  renderSizeImagesWithSds?: boolean;
  selection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
};

type SaveDraftOptions = {
  navigationTriggered?: boolean;
  source?: string;
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

function buildDraftURL(selection?: SDSProductVariantSelection) {
  const params = new URLSearchParams();
  if (!selection) {
    return "/api/shein-studio/draft";
  }

  params.set("productId", String(selection.productId));
  params.set("parentProductId", String(selection.parentProductId));
  params.set("variantId", String(selection.variantId));
  params.set("prototypeGroupId", String(selection.prototypeGroupId));
  params.set("layerId", selection.layerId);
  if (selection.printableWidth) {
    params.set("printWidth", String(selection.printableWidth));
  }
  if (selection.printableHeight) {
    params.set("printHeight", String(selection.printableHeight));
  }
  if (selection.selectedVariantIds?.length) {
    params.set("variantIds", selection.selectedVariantIds.join(","));
  }

  return `/api/shein-studio/draft?${params.toString()}`;
}

export async function loadSheinStudioDraft(selection?: SDSProductVariantSelection) {
  const response = await fetch(buildDraftURL(selection), {
    cache: "no-store",
  });
  const payload = await parseJSON<{ draft: SheinStudioDraft | null }>(response);
  return normalizeDraft(payload.draft);
}

export async function saveSheinStudioDraft(input: SheinStudioSaveInput) {
  return saveSheinStudioDraftWithOptions(input);
}

export async function saveSheinStudioDraftWithOptions(
  input: SheinStudioSaveInput,
  options?: SaveDraftOptions,
) {
  const body = JSON.stringify(input);
  const startedAt = performance.now();
  const bodyBytes = new TextEncoder().encode(body).byteLength;
  const source = options?.source ?? "unknown";

  console.info("[shein-studio-draft] client save started", {
    bodyBytes,
    designCount: input.designs.length,
    navigationTriggered: options?.navigationTriggered ?? false,
    selectionVariantId: input.selection?.variantId ?? null,
    source,
  });

  const response = await fetch("/api/shein-studio/draft", {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body,
    cache: "no-store",
  });
  try {
    const payload = await parseJSON<{ draft: SheinStudioDraft | null }>(response);
    console.info("[shein-studio-draft] client save completed", {
      bodyBytes,
      designCount: input.designs.length,
      draftSaveDurationMs: Math.round(performance.now() - startedAt),
      draftSaveStatus: "succeeded",
      navigationTriggered: options?.navigationTriggered ?? false,
      selectionVariantId: input.selection?.variantId ?? null,
      source,
    });
    return normalizeDraft(payload.draft);
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
