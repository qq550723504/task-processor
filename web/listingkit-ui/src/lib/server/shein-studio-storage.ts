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
  SheinStudioStorageData,
} from "@/lib/types/shein-studio";

const STORAGE_DIR = path.join(process.cwd(), ".data");
const STORAGE_PATH = path.join(STORAGE_DIR, "shein-studio-storage.json");
let writeQueue: Promise<void> = Promise.resolve();

type SaveDraftInput = {
  prompt: string;
  styleCount: string;
  sheinStoreId: string;
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

  writeQueue = writeQueue
    .catch(() => undefined)
    .then(async () => {
      await ensureStorageDir();
      const tempPath = `${STORAGE_PATH}.${process.pid}.${Date.now()}.${Math.random()
        .toString(16)
        .slice(2)}.tmp`;
      await writeFile(tempPath, payload, "utf8");
      await rename(tempPath, STORAGE_PATH);
    });

  await writeQueue;
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
  const storage = await readSheinStudioStorage();
  const draft = normalizeDraft({
    prompt: input.prompt,
    styleCount: input.styleCount,
    sheinStoreId: input.sheinStoreId,
    selectionVariantId: input.selection?.variantId,
    selection: buildSelectionSummary(input.selection),
    designs: input.designs,
    selectedIds: input.selectedIds,
    createdTasks: input.createdTasks,
    updatedAt: new Date().toISOString(),
  });

  const next = {
    ...storage,
    draft,
  } satisfies SheinStudioStorageData;

  await writeStorage(next);
  return draft;
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
  const storage = await readSheinStudioStorage();
  const batch = normalizeBatch({
    id: input.id ?? crypto.randomUUID(),
    name: deriveBatchName(input.prompt),
    prompt: input.prompt,
    styleCount: input.styleCount,
    sheinStoreId: input.sheinStoreId,
    selectionVariantId: input.selection?.variantId,
    selection: buildSelectionSummary(input.selection),
    designs: input.designs,
    selectedIds: input.selectedIds,
    createdTasks: input.createdTasks,
    updatedAt: new Date().toISOString(),
  });

  if (!batch) {
    return null;
  }

  const nextBatches = [batch, ...storage.batches.filter((item) => item.id !== batch.id)].slice(
    0,
    MAX_SHEIN_STUDIO_BATCHES,
  );

  await writeStorage({
    ...storage,
    batches: nextBatches,
  });

  return batch;
}

export async function deleteSheinStudioBatch(batchId: string) {
  const storage = await readSheinStudioStorage();
  await writeStorage({
    ...storage,
    batches: storage.batches.filter((item) => item.id !== batchId),
  });
}
