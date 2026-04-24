import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioDraft,
  SheinStudioGeneratedDesign,
  SheinStudioSavedBatch,
  SheinStudioStorageData,
} from "@/lib/types/shein-studio";

export const MAX_SHEIN_STUDIO_BATCHES = 12;

export function isGeneratedDesign(item: unknown): item is SheinStudioGeneratedDesign {
  return (
    !!item &&
    typeof item === "object" &&
    typeof (item as SheinStudioGeneratedDesign).id === "string" &&
    (typeof (item as SheinStudioGeneratedDesign).dataUrl === "string" ||
      typeof (item as SheinStudioGeneratedDesign).imageUrl === "string")
  );
}

export function isCreatedTask(item: unknown): item is SheinStudioCreatedTask {
  return (
    !!item &&
    typeof item === "object" &&
    typeof (item as SheinStudioCreatedTask).id === "string" &&
    typeof (item as SheinStudioCreatedTask).title === "string" &&
    typeof (item as SheinStudioCreatedTask).designId === "string"
  );
}

function isSelection(item: unknown): item is SDSProductVariantSelection {
  return (
    !!item &&
    typeof item === "object" &&
    typeof (item as SDSProductVariantSelection).variantId === "number" &&
    typeof (item as SDSProductVariantSelection).productId === "number" &&
    typeof (item as SDSProductVariantSelection).parentProductId === "number" &&
    typeof (item as SDSProductVariantSelection).prototypeGroupId === "number" &&
    typeof (item as SDSProductVariantSelection).layerId === "string" &&
    typeof (item as SDSProductVariantSelection).productName === "string"
  );
}

export function normalizeSelection(selection: unknown) {
  return isSelection(selection) ? selection : undefined;
}

export function normalizeDraft(raw: Partial<SheinStudioDraft> | null | undefined) {
  if (!raw?.prompt) {
    return null;
  }

  return {
    prompt: raw.prompt,
    styleCount: raw.styleCount ?? "4",
    sheinStoreId: raw.sheinStoreId ?? "",
    selectionVariantId: raw.selectionVariantId,
    selection: normalizeSelection(raw.selection),
    designs: Array.isArray(raw.designs) ? raw.designs.filter(isGeneratedDesign) : [],
    selectedIds: Array.isArray(raw.selectedIds)
      ? raw.selectedIds.filter((item): item is string => typeof item === "string")
      : [],
    createdTasks: Array.isArray(raw.createdTasks)
      ? raw.createdTasks.filter(isCreatedTask)
      : [],
    updatedAt: raw.updatedAt ?? new Date().toISOString(),
  } satisfies SheinStudioDraft;
}

export function normalizeBatch(raw: Partial<SheinStudioSavedBatch> | null | undefined) {
  if (!raw?.id || !raw.name || !raw.prompt) {
    return null;
  }

  return {
    id: raw.id,
    name: raw.name,
    prompt: raw.prompt,
    styleCount: raw.styleCount ?? "4",
    sheinStoreId: raw.sheinStoreId ?? "",
    selectionVariantId: raw.selectionVariantId,
    selection: normalizeSelection(raw.selection),
    designs: Array.isArray(raw.designs) ? raw.designs.filter(isGeneratedDesign) : [],
    selectedIds: Array.isArray(raw.selectedIds)
      ? raw.selectedIds.filter((item): item is string => typeof item === "string")
      : [],
    createdTasks: Array.isArray(raw.createdTasks)
      ? raw.createdTasks.filter(isCreatedTask)
      : [],
    updatedAt: raw.updatedAt ?? new Date().toISOString(),
  } satisfies SheinStudioSavedBatch;
}

export function normalizeStorageData(raw: unknown): SheinStudioStorageData {
  if (!raw || typeof raw !== "object") {
    return { draft: null, batches: [] };
  }

  const parsed = raw as Partial<SheinStudioStorageData>;
  const batches = Array.isArray(parsed.batches)
    ? parsed.batches
        .map((item) => normalizeBatch(item as Partial<SheinStudioSavedBatch>))
        .filter((item): item is NonNullable<typeof item> => Boolean(item))
        .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))
    : [];

  return {
    draft: normalizeDraft(parsed.draft),
    batches,
  };
}

export function buildSelectionSummary(selection?: SDSProductVariantSelection) {
  return selection ? selection : undefined;
}

export function deriveBatchName(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return "Untitled batch";
  }
  return trimmed.length > 36 ? `${trimmed.slice(0, 36)}...` : trimmed;
}
