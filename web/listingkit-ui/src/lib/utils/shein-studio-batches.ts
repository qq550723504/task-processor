import {
  normalizeBatch,
  normalizeDraft,
} from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioDraft,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

type SaveInput = {
  id?: string;
  prompt: string;
  styleCount: string;
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  renderSizeImagesWithSds?: boolean;
  selection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
};

async function parseJSON<T>(response: Response) {
  const payload = (await response.json()) as T & { message?: string };
  if (!response.ok) {
    throw new Error(payload.message || "SHEIN Studio storage request failed");
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
  params.set("productName", selection.productName);
  params.set("variantLabel", selection.variantLabel);
  if (selection.printableWidth) {
    params.set("printWidth", String(selection.printableWidth));
  }
  if (selection.printableHeight) {
    params.set("printHeight", String(selection.printableHeight));
  }
  if (selection.templateImageUrl) {
    params.set("templateImageUrl", selection.templateImageUrl);
  }
  if (selection.mockupImageUrl) {
    params.set("mockupImageUrl", selection.mockupImageUrl);
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

export async function saveSheinStudioDraft(input: SaveInput) {
  const response = await fetch("/api/shein-studio/draft", {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
    cache: "no-store",
  });
  const payload = await parseJSON<{ draft: SheinStudioDraft | null }>(response);
  return normalizeDraft(payload.draft);
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

export async function saveSheinStudioBatch(input: SaveInput) {
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
