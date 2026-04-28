import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import path from "node:path";

import {
  buildSelectionSummary,
  deriveBatchName,
  MAX_SHEIN_STUDIO_BATCHES,
  normalizeBatch,
  normalizeDraft,
  normalizeStorageData,
} from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioStorageData,
} from "@/lib/types/shein-studio";

const STORAGE_DIR = path.join(process.cwd(), ".data");
const STORAGE_PATH = path.join(STORAGE_DIR, "shein-studio-storage.json");
let storageQueue: Promise<unknown> = Promise.resolve();

type SaveDraftInput = {
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

type SaveBatchInput = SaveDraftInput & {
  id?: string;
};

async function ensureStorageDir() {
  await mkdir(STORAGE_DIR, { recursive: true });
}

async function writeStorage(data: SheinStudioStorageData) {
  const payload = JSON.stringify(data, null, 2);

  await ensureStorageDir();
  const tempPath = `${STORAGE_PATH}.${process.pid}.${Date.now()}.${Math.random()
    .toString(16)
    .slice(2)}.tmp`;
  await writeFile(tempPath, payload, "utf8");
  await rename(tempPath, STORAGE_PATH);
}

function updateStorage<T>(
  mutate: (storage: SheinStudioStorageData) => Promise<{ next: SheinStudioStorageData; result: T }>,
) {
  const operation = storageQueue
    .catch(() => undefined)
    .then(async () => {
      const storage = await readSheinStudioStorage();
      const { next, result } = await mutate(storage);
      await writeStorage(next);
      return result;
    });

  storageQueue = operation.then(
    () => undefined,
    () => undefined,
  );
  return operation;
}

export async function readSheinStudioStorage() {
  try {
    const raw = await readFile(STORAGE_PATH, "utf8");
    return normalizeStorageData(JSON.parse(raw));
  } catch {
    return { draft: null, batches: [] } satisfies SheinStudioStorageData;
  }
}

export async function getSheinStudioDraft(selection?: SDSProductVariantSelection) {
  const storage = await readSheinStudioStorage();
  const draft = storage.draft;
  if (!draft) {
    return null;
  }

  if (
    selection?.variantId &&
    draft.selectionVariantId &&
    draft.selectionVariantId !== selection.variantId
  ) {
    return null;
  }

  return draft;
}

export async function saveSheinStudioDraft(input: SaveDraftInput) {
  return updateStorage(async (storage) => {
    const draft = normalizeDraft({
      prompt: input.prompt,
      styleCount: input.styleCount,
      productImageCount: input.productImageCount,
      productImagePrompt: input.productImagePrompt,
      productImagePrompts: input.productImagePrompts,
      sheinStoreId: input.sheinStoreId,
      imageStrategy: input.imageStrategy,
      renderSizeImagesWithSds: input.renderSizeImagesWithSds,
      selectionVariantId: input.selection?.variantId,
      selection: buildSelectionSummary(input.selection),
      designs: input.designs,
      selectedIds: input.selectedIds,
      createdTasks: input.createdTasks,
      updatedAt: new Date().toISOString(),
    });

    return {
      next: {
        ...storage,
        draft,
      },
      result: draft,
    };
  });
}

export async function listSheinStudioBatches() {
  const storage = await readSheinStudioStorage();
  return storage.batches;
}

export async function getSheinStudioBatch(batchId: string) {
  const storage = await readSheinStudioStorage();
  return storage.batches.find((item) => item.id === batchId) ?? null;
}

export async function saveSheinStudioBatch(input: SaveBatchInput) {
  return updateStorage(async (storage) => {
    const batch = normalizeBatch({
      id: input.id ?? crypto.randomUUID(),
      name: deriveBatchName(input.prompt),
      prompt: input.prompt,
      styleCount: input.styleCount,
      productImageCount: input.productImageCount,
      productImagePrompt: input.productImagePrompt,
      productImagePrompts: input.productImagePrompts,
      sheinStoreId: input.sheinStoreId,
      imageStrategy: input.imageStrategy,
      renderSizeImagesWithSds: input.renderSizeImagesWithSds,
      selectionVariantId: input.selection?.variantId,
      selection: buildSelectionSummary(input.selection),
      designs: input.designs,
      selectedIds: input.selectedIds,
      createdTasks: input.createdTasks,
      updatedAt: new Date().toISOString(),
    });

    if (!batch) {
      return { next: storage, result: null };
    }

    const nextBatches = [batch, ...storage.batches.filter((item) => item.id !== batch.id)].slice(
      0,
      MAX_SHEIN_STUDIO_BATCHES,
    );

    return {
      next: {
        ...storage,
        batches: nextBatches,
      },
      result: batch,
    };
  });
}

export async function deleteSheinStudioBatch(batchId: string) {
  await updateStorage(async (storage) => {
    return {
      next: {
        ...storage,
        batches: storage.batches.filter((item) => item.id !== batchId),
      },
      result: undefined,
    };
  });
}
