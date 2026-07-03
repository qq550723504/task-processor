import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import { parsePositiveInt } from "@/lib/shein-studio/create-review-tasks";
import {
  buildGroupedGenerationTargets,
  type SheinStudioGroupedGenerationTarget,
} from "@/lib/shein-studio/grouped-image-mode";
import { buildSDSProductReferenceImageUrls } from "@/lib/shein-studio/sds-reference-images";
import type {
  SheinStudioArtworkModel,
  SheinStudioGenerateRequest,
  SheinStudioGenerateResponse,
  SheinStudioGenerationJob,
  SDSGroupedPromptHistoryEntry,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioGroupedWorkspace,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

type GenerationField =
  | "createdTasks"
  | "designs"
  | "groups"
  | "generationJobs"
  | "generationWarning"
  | "selectedIds";

type GenerationSetField = (field: GenerationField, value: unknown) => void;

type GenerationPersistDraft = (
  overrides?: Partial<{
    designs: SheinStudioGeneratedDesign[];
    groups: SheinStudioGroupedWorkspace[];
    selectedIds: string[];
    createdTasks: [];
    generationJobs: SheinStudioGenerationJob[];
  }>,
  options?: {
    navigationTriggered?: boolean;
    source?: string;
    warnOnFailure?: boolean;
  },
) => Promise<unknown>;

type ExecuteStandaloneGenerationInput = {
  activeGroupId: string;
  activeSelection: SDSProductVariantSelection;
  artworkModel: SheinStudioArtworkModel;
  generateDesigns: (
    request: SheinStudioGenerateRequest,
    options?: { onJobStarted?: (jobId: string) => void },
  ) => Promise<SheinStudioGenerateResponse>;
  generationJobs: SheinStudioGenerationJob[];
  groupedImageMode: SheinStudioGroupedImageMode;
  groupedSelections: GroupedSDSSelectionEligibility[];
  groups: SheinStudioGroupedWorkspace[];
  hasLocalWorkflowStateRef: { current: boolean };
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferencePrompt?: string;
  navigateToStep: (step: "review") => void;
  persistDraft: GenerationPersistDraft;
  prompt: string;
  promptMode?: SheinStudioGenerateRequest["promptMode"];
  setField: GenerationSetField;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
  formatError?: (error: unknown) => string;
  onJobStarted?: (input: {
    jobId: string;
    target: SheinStudioGroupedGenerationTarget;
  }) => void;
};

type ExecuteStandaloneGenerationResult = {
  designs: SheinStudioGeneratedDesign[];
  groups: SheinStudioGroupedWorkspace[];
  selectedIds: string[];
  warnings: string[];
  targetErrors: string[];
};

export function resolveGenerationStartValidation({
  activeSelection,
  prompt,
  sheinStoreId,
}: {
  activeSelection?: SDSProductVariantSelection;
  prompt: string;
  sheinStoreId: string;
}) {
  if (!activeSelection?.variantId) {
    return { error: "请先选择 SDS 变体。" };
  }
  if (!sheinStoreId.trim()) {
    return { error: "请先选择批次店铺。" };
  }
  if (!prompt.trim()) {
    return { error: "请先填写主题提示词。", focusPrompt: true };
  }
  return null;
}

export function resolveRegenerationStartValidation({
  activeSelection,
  prompt,
}: {
  activeSelection?: SDSProductVariantSelection;
  prompt: string;
}) {
  if (!activeSelection?.variantId) {
    return { error: "请先选择 SDS 变体。" };
  }
  if (!prompt.trim()) {
    return { error: "请先填写主题提示词。", focusPrompt: true };
  }
  return null;
}

export function buildGenerationPromptHistoryGroups({
  activeGroupId,
  groupedImageMode,
  groups,
  prompt,
}: {
  activeGroupId: string;
  groupedImageMode: SheinStudioGroupedImageMode;
  groups: SheinStudioGroupedWorkspace[];
  prompt: string;
}) {
  const trimmedPrompt = prompt.trim();
  if (!trimmedPrompt || !activeGroupId) {
    return groups;
  }
  const historyEntry: SDSGroupedPromptHistoryEntry = {
    prompt: trimmedPrompt,
    groupedImageMode,
    createdAt: new Date().toISOString(),
  };
  let changed = false;
  const nextGroups = groups.map((group) => {
    if (group.id !== activeGroupId) {
      return group;
    }
    const newest = group.promptHistory[0];
    const promptHistory =
      newest?.prompt === historyEntry.prompt &&
      newest?.groupedImageMode === historyEntry.groupedImageMode
        ? group.promptHistory
        : [historyEntry, ...group.promptHistory].slice(0, 5);
    if (
      promptHistory === group.promptHistory &&
      group.currentPrompt === trimmedPrompt
    ) {
      return group;
    }
    changed = true;
    return {
      ...group,
      currentPrompt: trimmedPrompt,
      promptHistory,
      updatedAt: historyEntry.createdAt,
    };
  });
  return changed ? nextGroups : groups;
}

export function withGenerationTargetMetadata(
  images: SheinStudioGeneratedDesign[],
  target: { key: string; label?: string },
) {
  return images.map((image) => ({
    ...image,
    targetGroupKey: target.key,
    targetGroupLabel: target.label,
  }));
}

export function mergeGeneratedDesignCollections(
  currentDesigns: SheinStudioGeneratedDesign[],
  incomingDesigns: SheinStudioGeneratedDesign[],
) {
  const nextByID = new Map(currentDesigns.map((design) => [design.id, design]));
  for (const design of incomingDesigns) {
    nextByID.set(design.id, design);
  }
  return Array.from(nextByID.values());
}

export function mergeGeneratedSelectedIds(
  currentIDs: string[],
  designs: SheinStudioGeneratedDesign[],
) {
  const next = new Set(currentIDs);
  for (const design of designs) {
    next.add(design.id);
  }
  return Array.from(next);
}

export function replaceRegeneratedDesign({
  designId,
  designs,
  replacement,
}: {
  designId: string;
  designs: SheinStudioGeneratedDesign[];
  replacement: SheinStudioGeneratedDesign;
}) {
  return designs.map((design) =>
    design.id === designId
      ? {
          ...replacement,
          id: designId,
          targetGroupKey: design.targetGroupKey,
          targetGroupLabel: design.targetGroupLabel,
        }
      : design,
  );
}

export function buildHotStyleReferenceGenerationInput(input: {
  prompt: string;
  hotStyleReferencePrompt?: string;
  productReferenceImageUrls?: string[];
  hotStyleReferenceImageUrls?: string[];
}) {
  const referencePrompt = input.hotStyleReferencePrompt?.trim();
  const promptParts = [input.prompt.trim()];
  if (referencePrompt) {
    promptParts.push(
      "Hot-selling reference direction for original artwork:",
      referencePrompt,
    );
  }
  const normalizedProductReferences = normalizeReferenceImageUrls(
    input.productReferenceImageUrls,
  );
  const normalizedHotStyleReferences = referencePrompt
    ? normalizeReferenceImageUrls(input.hotStyleReferenceImageUrls)
    : [];
  return {
    prompt: promptParts.filter(Boolean).join("\n"),
    productReferenceImageUrls: prioritizeReferenceImageUrls({
      existing: normalizedProductReferences,
      prioritized: normalizedHotStyleReferences,
      limit: 5,
    }),
  };
}

function normalizeReferenceImageUrls(values?: string[]) {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const value of values ?? []) {
    const normalized = value.trim();
    if (!normalized || seen.has(normalized)) {
      continue;
    }
    seen.add(normalized);
    result.push(normalized);
  }
  return result;
}

function prioritizeReferenceImageUrls(input: {
  existing: string[];
  prioritized: string[];
  limit: number;
}) {
  const seen = new Set<string>();
  const result: string[] = [];
  const append = (value: string) => {
    if (!value || seen.has(value) || result.length >= input.limit) {
      return;
    }
    seen.add(value);
    result.push(value);
  };
  for (const value of input.prioritized) {
    append(value);
  }
  for (const value of input.existing) {
    append(value);
  }
  return result;
}

export async function executeStandaloneGeneration({
  activeGroupId,
  activeSelection,
  artworkModel,
  generateDesigns,
  generationJobs,
  groupedImageMode,
  groupedSelections,
  groups,
  hasLocalWorkflowStateRef,
  hotStyleReferenceImageUrls,
  hotStyleReferencePrompt,
  navigateToStep,
  persistDraft,
  prompt,
  promptMode,
  setField,
  styleCount,
  transparentBackground,
  variationIntensity,
  formatError = (error) =>
    error instanceof Error ? error.message : "款式图生成失败。",
  onJobStarted,
}: ExecuteStandaloneGenerationInput): Promise<ExecuteStandaloneGenerationResult> {
  const nextGroups = buildGenerationPromptHistoryGroups({
    activeGroupId,
    groupedImageMode,
    groups,
    prompt,
  });
  if (nextGroups !== groups) {
    setField("groups", nextGroups);
  }

  const targets = buildGroupedGenerationTargets({
    activeSelection,
    groupedSelections: groupedSelections
      .filter((item) => item.eligible)
      .map((item) => item.selection),
    groupedImageMode,
  });
  let nextGenerationJobs = generationJobs.filter(
    (job) => job.status === "running" && job.jobId.trim(),
  );
  let accumulatedDesigns: SheinStudioGeneratedDesign[] = [];
  let accumulatedSelectedIDs: string[] = [];
  const aggregatedWarnings: string[] = [];
  const targetErrors: string[] = [];

  const syncGenerationJobs = (jobs: SheinStudioGenerationJob[]) => {
    nextGenerationJobs = jobs;
    setField("generationJobs", jobs);
  };

  const persistProgress = async (
    nextSelectedIds: string[],
    jobs: SheinStudioGenerationJob[],
  ) => {
    await persistDraft(
      {
        designs: accumulatedDesigns,
        groups: nextGroups,
        selectedIds: nextSelectedIds,
        createdTasks: [],
        generationJobs: jobs,
      },
      {
        source: "generate_progress",
        warnOnFailure: false,
      },
    ).catch(() => undefined);
  };

  syncGenerationJobs([]);

  const settled = await Promise.allSettled(
    targets.map(async (target) => {
      const generationInput = buildHotStyleReferenceGenerationInput({
        prompt,
        hotStyleReferencePrompt,
        productReferenceImageUrls:
          buildSDSProductReferenceImageUrls(target.selection),
        hotStyleReferenceImageUrls,
      });
      const response = await generateDesigns(
        buildSheinStudioGenerateRequest({
          prompt: generationInput.prompt,
          promptMode,
          variationIntensity,
          printableWidth: target.selection.printableWidth,
          printableHeight: target.selection.printableHeight,
          productReferenceImageUrls: generationInput.productReferenceImageUrls,
          styleCount: parsePositiveInt(styleCount) ?? 1,
          artworkModel,
          transparentBackground,
        }),
        {
          onJobStarted: (jobId) => {
            const existingIndex = nextGenerationJobs.findIndex(
              (job) => job.jobId === jobId,
            );
            const nextJob: SheinStudioGenerationJob = {
              jobId,
              targetGroupKey: target.key,
              targetGroupLabel: target.label,
              status: "running",
            };
            const jobs =
              existingIndex >= 0
                ? nextGenerationJobs.map((job, index) =>
                    index === existingIndex ? nextJob : job,
                  )
                : [...nextGenerationJobs, nextJob];
            onJobStarted?.({ jobId, target });
            syncGenerationJobs(jobs);
          },
        },
      );
      const nextImages = withGenerationTargetMetadata(response.images, target);
      const targetJobIndex = nextGenerationJobs.findIndex(
        (job) => job.targetGroupKey === target.key,
      );
      if (targetJobIndex >= 0) {
        syncGenerationJobs(
          nextGenerationJobs.map((job, index) =>
            index === targetJobIndex
              ? { ...job, status: "succeeded" as const }
              : job,
          ),
        );
      }
      if (response.warnings?.length) {
        aggregatedWarnings.push(...response.warnings);
      }
      if (nextImages.length > 0) {
        accumulatedDesigns = mergeGeneratedDesignCollections(
          accumulatedDesigns,
          nextImages,
        );
        accumulatedSelectedIDs = mergeGeneratedSelectedIds(
          accumulatedSelectedIDs,
          nextImages,
        );
        hasLocalWorkflowStateRef.current = true;
        setField("designs", accumulatedDesigns);
        setField("selectedIds", accumulatedSelectedIDs);
        setField("createdTasks", []);
        navigateToStep("review");
        await persistProgress(
          accumulatedSelectedIDs,
          nextGenerationJobs.filter((job) => job.status === "running"),
        );
      }
      return nextImages;
    }),
  );

  for (let index = 0; index < settled.length; index += 1) {
    const result = settled[index];
    if (result.status === "fulfilled") {
      continue;
    }
    const target = targets[index];
    const message = formatError(result.reason);
    targetErrors.push(target?.label ? `${target.label}: ${message}` : message);
    const targetJobIndex = nextGenerationJobs.findIndex(
      (job) => job.targetGroupKey === target?.key,
    );
    if (targetJobIndex >= 0) {
      syncGenerationJobs(
        nextGenerationJobs.map((job, jobIndex) =>
          jobIndex === targetJobIndex
            ? { ...job, status: "failed" as const }
            : job,
        ),
      );
    }
  }

  if (!accumulatedDesigns.length) {
    throw new Error(
      "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
    );
  }
  if (aggregatedWarnings.length > 0 || targetErrors.length > 0) {
    setField(
      "generationWarning",
      [...aggregatedWarnings, ...targetErrors].join(" "),
    );
  }
  hasLocalWorkflowStateRef.current = true;
  setField("designs", accumulatedDesigns);
  setField("selectedIds", accumulatedSelectedIDs);
  setField("generationJobs", []);
  navigateToStep("review");
  await persistDraft(
    {
      designs: accumulatedDesigns,
      groups: nextGroups,
      selectedIds: accumulatedSelectedIDs,
      createdTasks: [],
      generationJobs: [],
    },
    {
      navigationTriggered: true,
      source: "generate_success",
      warnOnFailure: false,
    },
  ).catch(() => undefined);

  return {
    designs: accumulatedDesigns,
    groups: nextGroups,
    selectedIds: accumulatedSelectedIDs,
    warnings: aggregatedWarnings,
    targetErrors,
  };
}

export function buildSheinStudioGenerateRequest({
  artworkModel,
  prompt,
  promptMode,
  printableHeight,
  printableWidth,
  productReferenceImageUrls,
  styleCount,
  transparentBackground,
  variationIntensity,
}: {
  artworkModel: SheinStudioArtworkModel;
  prompt: string;
  promptMode?: SheinStudioGenerateRequest["promptMode"];
  printableHeight?: number;
  printableWidth?: number;
  productReferenceImageUrls?: string[];
  styleCount: number;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
}): SheinStudioGenerateRequest {
  const trimmedModel = artworkModel.trim();
  const normalizedPrompt = normalizeSheinStudioPromptWithSDSSize({
    prompt,
    printableWidth,
    printableHeight,
  });
  return {
    prompt: normalizedPrompt,
    promptMode,
    count: styleCount,
    variationIntensity,
    printableWidth,
    printableHeight,
    productReferenceImageUrls,
    imageModel: transparentBackground ? "gpt-image-2" : trimmedModel || undefined,
    transparentBackground,
  };
}

function normalizeSheinStudioPromptWithSDSSize({
  prompt,
  printableWidth,
  printableHeight,
}: {
  prompt: string;
  printableWidth?: number;
  printableHeight?: number;
}) {
  const trimmedPrompt = prompt.trim();
  if (!trimmedPrompt) {
    return trimmedPrompt;
  }
  if (!printableWidth || !printableHeight) {
    return trimmedPrompt;
  }
  const sizeSuffix = `printable size: ${printableWidth}x${printableHeight}px.`;
  const normalizedPrompt = trimmedPrompt.replace(/\s+/g, " ");
  const lowerPrompt = normalizedPrompt.toLowerCase();
  const lowerSuffix = sizeSuffix.toLowerCase();
  const compactSizeToken = `${printableWidth}x${printableHeight}px`.toLowerCase();
  const spacedSizeToken = `${printableWidth} × ${printableHeight}px`.toLowerCase();

  if (
    lowerPrompt.includes(lowerSuffix) ||
    lowerPrompt.includes(compactSizeToken) ||
    lowerPrompt.includes(spacedSizeToken)
  ) {
    return normalizedPrompt;
  }
  return `${normalizedPrompt}\n\n${sizeSuffix}`;
}
