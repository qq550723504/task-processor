import { buildRecentBatchSummaries } from "@/lib/shein-studio/recent-batch-summaries";
import { buildDuplicatedSheinStudioBatchInput } from "@/lib/shein-studio/duplicate-batch";
import type {
  SheinStudioDraft,
  SheinStudioRecentBatchSummary,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";

type RecentBatchSelectionSummary = {
  id: string;
  source: "batch" | "local_draft";
};

type RecentBatchAction = "generate" | "review" | "tasks";

type RecentBatchSelectionProjectionParams = {
  rawSelectedRecentBatchSummaryIds: string[];
  validRecentBatchSummaryKeys: Set<string>;
};

type RecentBatchStep = "generate" | "review" | "tasks";

type RecentBatchSelectionUpdateParams = {
  current: string[];
  value: string[] | ((current: string[]) => string[]);
  validRecentBatchSummaryKeys: Set<string>;
};

type RecentBatchSummariesProjectionParams = {
  draft?: SheinStudioDraft | null;
  draftBatchId?: string;
  savedBatches: SheinStudioSavedBatch[];
  selectedRecentBatchHydrations: RecentBatchHydrationMap;
};

type FreshRecentBatchHydrationParams = {
  cachedHydratedBatch?: SheinStudioWorkbenchHydratedBatch | null;
  savedBatch: SheinStudioSavedBatch;
};

type RecentBatchHydrationMap = Record<string, SheinStudioWorkbenchHydratedBatch>;

type RecentBatchHydrationRequestMap = Map<
  string,
  Promise<SheinStudioWorkbenchHydratedBatch | null>
>;

type ResolveRecentBatchHydrationEntriesParams = {
  batchIds: string[];
  loadHydratedBatch: (
    batchId: string,
  ) => Promise<SheinStudioWorkbenchHydratedBatch | null>;
  pendingHydrationRequests: RecentBatchHydrationRequestMap;
  savedBatches: SheinStudioSavedBatch[];
  selectedRecentBatchHydrations: RecentBatchHydrationMap;
};

type ResolveRecentBatchForMutationParams = {
  batchId: string;
  cacheHydratedBatch: (
    batchId: string,
    hydratedBatch: SheinStudioWorkbenchHydratedBatch,
  ) => void;
  loadHydratedBatch: (
    batchId: string,
  ) => Promise<SheinStudioWorkbenchHydratedBatch | null>;
  savedBatches: SheinStudioSavedBatch[];
  selectedRecentBatchHydrations: Record<string, SheinStudioWorkbenchHydratedBatch>;
};

type ResolveRecentBatchSelectionTargetParams = {
  loadHydratedBatch: (
    batchId: string,
  ) => Promise<SheinStudioWorkbenchHydratedBatch | null>;
  savedBatches: SheinStudioSavedBatch[];
  summary: RecentBatchSelectionSummary;
};

type RunRecentBatchSummarySelectionParams = {
  action?: RecentBatchAction;
  advanceRequestVersion: () => number;
  getCurrentRequestVersion: () => number;
  hasLocalDraft: boolean;
  loadHydratedBatch: (
    batchId: string,
  ) => Promise<SheinStudioWorkbenchHydratedBatch | null>;
  openHydratedBatch: (hydratedBatch: SheinStudioWorkbenchHydratedBatch) => void;
  openLocalDraft: (targetStep: RecentBatchStep) => void;
  openSavedBatch: (batch: SheinStudioSavedBatch) => void;
  savedBatches: SheinStudioSavedBatch[];
  setEffectiveStep: (step: RecentBatchStep) => void;
  summary: RecentBatchSelectionSummary;
};

type ResolveRecentBatchMutationTargetsParams = {
  batchIds: string[];
  resolveBatch: (batchId: string) => Promise<SheinStudioSavedBatch | null>;
};

type RecentBatchDeleteRunner = (batchId: string) => Promise<unknown>;

type RecentBatchMutationSave = (
  input: SheinStudioSaveInput,
  options?: { makeActive?: boolean },
) => Promise<unknown>;

type RenameRecentBatchSummaryParams = {
  name: string;
  refreshSavedBatches: () => Promise<unknown>;
  resolveBatch: (batchId: string) => Promise<SheinStudioSavedBatch | null>;
  saveBatch: RecentBatchMutationSave;
  summary: RecentBatchSelectionSummary;
};

type DuplicateRecentBatchSummaryParams = {
  refreshSavedBatches: () => Promise<unknown>;
  resolveBatch: (batchId: string) => Promise<SheinStudioSavedBatch | null>;
  saveBatch: RecentBatchMutationSave;
  summary: RecentBatchSelectionSummary;
};

type DeleteRecentBatchSummaryParams = {
  clearLocalDraft: () => void;
  deleteBatch: RecentBatchDeleteRunner;
  removeSelection: (summary: RecentBatchSelectionSummary) => void;
  summary: RecentBatchSelectionSummary;
};

type RunRecentBatchBulkStoreUpdateParams = {
  batchIds: string[];
  refreshSavedBatches: () => Promise<unknown>;
  resolveBatch: (batchId: string) => Promise<SheinStudioSavedBatch | null>;
  saveBatch: RecentBatchMutationSave;
  storeId: string;
};

type RecentBatchSelectionTarget =
  | {
      hydratedBatch: SheinStudioWorkbenchHydratedBatch;
      kind: "hydrated";
    }
  | {
      batch: SheinStudioSavedBatch;
      kind: "saved";
    };

export function buildRecentBatchSummaryKey(
  summary: RecentBatchSelectionSummary,
): string {
  return `${summary.source}:${summary.id}`;
}

export function buildRecentBatchSummaryKeys(
  recentBatchSummaries: RecentBatchSelectionSummary[],
): Set<string> {
  return new Set(
    recentBatchSummaries.map((summary) => buildRecentBatchSummaryKey(summary)),
  );
}

export function upsertRecentSavedBatch(
  batches: SheinStudioSavedBatch[],
  nextBatch: SheinStudioSavedBatch,
) {
  return [nextBatch, ...batches.filter((batch) => batch.id !== nextBatch.id)].sort(
    (left, right) => right.updatedAt.localeCompare(left.updatedAt),
  );
}

export function removeRecentBatchSummarySelection(
  current: string[],
  summary: RecentBatchSelectionSummary,
): string[] {
  const key = buildRecentBatchSummaryKey(summary);
  return current.filter((item) => item !== key);
}

export function projectRecentBatchSelectionUpdate({
  current,
  value,
  validRecentBatchSummaryKeys,
}: RecentBatchSelectionUpdateParams): string[] {
  const next = typeof value === "function" ? value(current) : value;
  return next.filter((key) => validRecentBatchSummaryKeys.has(key));
}

export function projectRecentBatchTargetStep(
  action?: RecentBatchAction,
): RecentBatchStep {
  if (action === "tasks") {
    return "tasks";
  }
  if (action === "review") {
    return "review";
  }
  return "generate";
}

export function selectFreshRecentBatchHydration({
  cachedHydratedBatch,
  savedBatch,
}: FreshRecentBatchHydrationParams): SheinStudioWorkbenchHydratedBatch | null {
  if (
    cachedHydratedBatch &&
    Date.parse(cachedHydratedBatch.savedBatch.updatedAt) >=
      Date.parse(savedBatch.updatedAt)
  ) {
    return cachedHydratedBatch;
  }
  return null;
}

export function mergeRecentBatchHydrations(
  current: RecentBatchHydrationMap,
  entries: Iterable<readonly [string, SheinStudioWorkbenchHydratedBatch]>,
): RecentBatchHydrationMap {
  return {
    ...current,
    ...Object.fromEntries(entries),
  };
}

export async function resolveRecentBatchHydrationEntries({
  batchIds,
  loadHydratedBatch,
  pendingHydrationRequests,
  savedBatches,
  selectedRecentBatchHydrations,
}: ResolveRecentBatchHydrationEntriesParams): Promise<
  Array<readonly [string, SheinStudioWorkbenchHydratedBatch]>
> {
  const nextEntries = await Promise.all(
    batchIds.map(async (batchId) => {
      const batch = savedBatches.find((item) => item.id === batchId);
      if (!batch) {
        return null;
      }
      const cached = selectedRecentBatchHydrations[batchId];
      if (cached && cached.savedBatch.updatedAt === batch.updatedAt) {
        return [batchId, cached] as const;
      }
      let pending = pendingHydrationRequests.get(batchId);
      if (!pending) {
        pending = loadHydratedBatch(batchId)
          .catch(() => null)
          .finally(() => {
            pendingHydrationRequests.delete(batchId);
          });
        pendingHydrationRequests.set(batchId, pending);
      }
      const hydratedBatch = await pending;
      return hydratedBatch ? ([batchId, hydratedBatch] as const) : null;
    }),
  );
  return nextEntries.filter(
    (entry): entry is readonly [string, SheinStudioWorkbenchHydratedBatch] =>
      entry != null,
  );
}

export function projectRecentBatchSummaries({
  draft,
  draftBatchId,
  savedBatches,
  selectedRecentBatchHydrations,
}: RecentBatchSummariesProjectionParams): SheinStudioRecentBatchSummary[] {
  const baseSummaries = buildRecentBatchSummaries(savedBatches, {
    draft,
    draftBatchId,
  });
  return baseSummaries.map((summary) => {
    if (summary.source !== "batch") {
      return summary;
    }
    const hydratedBatch = selectedRecentBatchHydrations[summary.id];
    if (!hydratedBatch) {
      return summary;
    }
    return buildRecentBatchSummaries([hydratedBatch.savedBatch])[0] ?? summary;
  });
}

export async function resolveRecentBatchForMutation({
  batchId,
  cacheHydratedBatch,
  loadHydratedBatch,
  savedBatches,
  selectedRecentBatchHydrations,
}: ResolveRecentBatchForMutationParams): Promise<SheinStudioSavedBatch | null> {
  const savedBatch = savedBatches.find((item) => item.id === batchId);
  if (!savedBatch) {
    return null;
  }
  const freshHydratedBatch = selectFreshRecentBatchHydration({
    cachedHydratedBatch: selectedRecentBatchHydrations[batchId],
    savedBatch,
  });
  if (freshHydratedBatch) {
    return freshHydratedBatch.savedBatch;
  }
  try {
    const hydratedBatch = await loadHydratedBatch(batchId);
    if (!hydratedBatch) {
      return savedBatch;
    }
    cacheHydratedBatch(batchId, hydratedBatch);
    return hydratedBatch.savedBatch;
  } catch {
    return savedBatch;
  }
}

export async function resolveRecentBatchMutationTargets({
  batchIds,
  resolveBatch,
}: ResolveRecentBatchMutationTargetsParams): Promise<SheinStudioSavedBatch[]> {
  const targets = await Promise.all(batchIds.map((batchId) => resolveBatch(batchId)));
  return targets.filter((batch): batch is SheinStudioSavedBatch => batch != null);
}

export async function resolveRecentBatchSelectionTarget({
  loadHydratedBatch,
  savedBatches,
  summary,
}: ResolveRecentBatchSelectionTargetParams): Promise<RecentBatchSelectionTarget | null> {
  if (summary.source !== "batch") {
    return null;
  }
  const batch = savedBatches.find((item) => item.id === summary.id);
  if (!batch) {
    return null;
  }
  try {
    const hydratedBatch = await loadHydratedBatch(summary.id);
    if (!hydratedBatch) {
      return {
        batch,
        kind: "saved",
      };
    }
    return {
      hydratedBatch,
      kind: "hydrated",
    };
  } catch {
    return {
      batch,
      kind: "saved",
    };
  }
}

export async function runRecentBatchSummarySelection({
  action,
  advanceRequestVersion,
  getCurrentRequestVersion,
  hasLocalDraft,
  loadHydratedBatch,
  openHydratedBatch,
  openLocalDraft,
  openSavedBatch,
  savedBatches,
  setEffectiveStep,
  summary,
}: RunRecentBatchSummarySelectionParams) {
  const requestVersion = advanceRequestVersion();
  const targetStep = projectRecentBatchTargetStep(action);
  if (summary.source === "local_draft" && hasLocalDraft) {
    openLocalDraft(targetStep);
    return;
  }
  const target = await resolveRecentBatchSelectionTarget({
    loadHydratedBatch,
    savedBatches,
    summary,
  });
  if (!target || getCurrentRequestVersion() !== requestVersion) {
    return;
  }
  if (target.kind === "hydrated") {
    openHydratedBatch(target.hydratedBatch);
  } else {
    openSavedBatch(target.batch);
  }
  if (getCurrentRequestVersion() !== requestVersion) {
    return;
  }
  setEffectiveStep(targetStep);
}

export function projectRecentBatchSelectionState({
  rawSelectedRecentBatchSummaryIds,
  validRecentBatchSummaryKeys,
}: RecentBatchSelectionProjectionParams) {
  const selectedRecentBatchSummaryIds =
    rawSelectedRecentBatchSummaryIds.filter((key) =>
      validRecentBatchSummaryKeys.has(key),
    );
  const selectedPersistedRecentBatchIds =
    selectedRecentBatchSummaryIds.flatMap((key) => {
      const batchPrefix = "batch:";
      return key.startsWith(batchPrefix)
        ? [key.slice(batchPrefix.length)]
        : [];
    });

  return {
    selectedPersistedRecentBatchIds,
    selectedRecentBatchSummaryIds,
    validRecentBatchSummaryKeys,
  };
}

export function buildRecentBatchSaveInput(
  batch: SheinStudioSavedBatch,
  overrides?: Partial<SheinStudioSavedBatch>,
) {
  return {
    id: overrides?.id ?? batch.id,
    updatedAt:
      overrides?.draftUpdatedAt ??
      overrides?.updatedAt ??
      batch.draftUpdatedAt ??
      batch.updatedAt,
    name: overrides?.name ?? batch.name,
    prompt: overrides?.prompt ?? batch.prompt,
    promptMode: overrides?.promptMode ?? batch.promptMode,
    styleCount: overrides?.styleCount ?? batch.styleCount,
    variationIntensity:
      overrides?.variationIntensity ?? batch.variationIntensity,
    productImageCount: overrides?.productImageCount ?? batch.productImageCount,
    productImagePrompt:
      overrides?.productImagePrompt ?? batch.productImagePrompt,
    productImagePrompts:
      overrides?.productImagePrompts ?? batch.productImagePrompts,
    artworkModel: overrides?.artworkModel ?? batch.artworkModel,
    transparentBackground:
      overrides?.transparentBackground ?? batch.transparentBackground,
    sheinStoreId: overrides?.sheinStoreId ?? batch.sheinStoreId,
    imageStrategy: overrides?.imageStrategy ?? batch.imageStrategy,
    groupedImageMode:
      overrides?.groupedImageMode ?? batch.groupedImageMode,
    selectedSdsImages:
      overrides?.selectedSdsImages ?? batch.selectedSdsImages,
    renderSizeImagesWithSds:
      overrides?.renderSizeImagesWithSds ?? batch.renderSizeImagesWithSds,
    selection: overrides?.selection ?? batch.selection,
    groupedSelections:
      overrides?.groupedSelections ?? batch.groupedSelections,
    groups: overrides?.groups ?? batch.groups,
    designs: overrides?.designs ?? batch.designs,
    selectedIds: overrides?.selectedIds ?? batch.selectedIds,
    createdTasks: overrides?.createdTasks ?? batch.createdTasks,
    generationJobs: overrides?.generationJobs ?? batch.generationJobs,
    generationError: overrides?.generationError ?? batch.generationError,
    generationJobId: overrides?.generationJobId ?? batch.generationJobId,
  };
}

export async function renameRecentBatchSummary({
  name,
  refreshSavedBatches,
  resolveBatch,
  saveBatch,
  summary,
}: RenameRecentBatchSummaryParams) {
  if (summary.source !== "batch") {
    return;
  }
  const batch = await resolveBatch(summary.id);
  if (!batch) {
    return;
  }
  await saveBatch(buildRecentBatchSaveInput(batch, { name }), {
    makeActive: false,
  });
  await refreshSavedBatches();
}

export async function duplicateRecentBatchSummary({
  refreshSavedBatches,
  resolveBatch,
  saveBatch,
  summary,
}: DuplicateRecentBatchSummaryParams) {
  if (summary.source !== "batch") {
    return;
  }
  const batch = await resolveBatch(summary.id);
  if (!batch) {
    return;
  }
  await saveBatch(buildDuplicatedSheinStudioBatchInput(batch), {
    makeActive: false,
  });
  await refreshSavedBatches();
}

export async function deleteRecentBatchSummary({
  clearLocalDraft,
  deleteBatch,
  removeSelection,
  summary,
}: DeleteRecentBatchSummaryParams) {
  if (summary.source === "local_draft") {
    clearLocalDraft();
    removeSelection(summary);
    return;
  }
  if (summary.source !== "batch") {
    return;
  }
  await deleteBatch(summary.id);
}

export function buildRecentBatchStoreUpdateInput(
  batch: SheinStudioSavedBatch,
  storeId: string,
) {
  return buildRecentBatchSaveInput(batch, {
    sheinStoreId: storeId,
    groupedSelections: (batch.groupedSelections ?? []).map((item) => ({
      ...item,
      sheinStoreId: storeId,
    })),
    groups: (batch.groups ?? []).map((group) => ({
      ...group,
      sheinStoreId: storeId,
      groupedSelections: group.groupedSelections.map((item) => ({
        ...item,
        sheinStoreId: storeId,
      })),
    })),
  });
}

export function buildRecentBatchBulkStoreUpdateInputs(
  batches: SheinStudioSavedBatch[],
  storeId: string,
) {
  return batches.map((batch) => buildRecentBatchStoreUpdateInput(batch, storeId));
}

export async function runRecentBatchBulkStoreUpdate({
  batchIds,
  refreshSavedBatches,
  resolveBatch,
  saveBatch,
  storeId,
}: RunRecentBatchBulkStoreUpdateParams) {
  const targets = await resolveRecentBatchMutationTargets({
    batchIds,
    resolveBatch,
  });
  if (targets.length === 0) {
    return;
  }
  await Promise.all(
    buildRecentBatchBulkStoreUpdateInputs(targets, storeId).map((input) =>
      saveBatch(input, { makeActive: false }),
    ),
  );
  await refreshSavedBatches();
}

function isMissingRecentBatchDeleteError(error: unknown) {
  return error instanceof Error && /studio session not found/i.test(error.message);
}

export function selectRecentBatchBulkDeleteFailure(
  results: PromiseSettledResult<unknown>[],
): unknown | null {
  const failed = results.find(
    (result) =>
      result.status === "rejected" &&
      !isMissingRecentBatchDeleteError(result.reason),
  );
  return failed?.status === "rejected" ? failed.reason : null;
}

export async function runRecentBatchBulkDelete(
  summaryIds: string[],
  deleteBatch: RecentBatchDeleteRunner,
) {
  if (summaryIds.length === 0) {
    return;
  }
  const results = await Promise.allSettled(
    summaryIds.map((summaryId) => deleteBatch(summaryId)),
  );
  const failure = selectRecentBatchBulkDeleteFailure(results);
  if (failure) {
    throw failure;
  }
}
