import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  normalizeGroupedSDSSelectionEligibility,
  removePrimarySelectionFromGroupedSelections,
  type GroupedSDSSelectionEligibility,
} from "@/lib/types/sds-baseline";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioCreatedTask,
  SheinStudioArtworkModel,
  SheinStudioArtworkGenerationMode,
  SheinStudioDraft,
  SheinStudioGenerationJob,
  SheinStudioGeneratedDesign,
  SheinStudioLegacyCompatibilitySnapshot,
  SheinStudioGroupedWorkspace,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioSavedBatch,
  SheinStudioStorageData,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import { normalizeSelectedSDSImages } from "@/lib/shein-studio/sds-selectable-images";

type LegacyBatchStatusCarrier = {
  sessionStatus?: unknown;
};

export const MAX_SHEIN_STUDIO_BATCHES = 12;
export const DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY: SheinStudioImageStrategy =
  "sds_official";
export const DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT = "5";
export const DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL: SheinStudioArtworkModel =
  "";
export const DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE: SheinStudioGroupedImageMode =
  "shared_by_size";
export const DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY: SheinStudioVariationIntensity =
  "medium";
export const SHEIN_STUDIO_PRODUCT_IMAGE_ROLES = [
  {
    role: "main",
    label: "主图",
    hint: "亚马逊标准主图，纯白背景，商品居中完整展示，无文字、Logo、阴影。",
    defaultPrompt:
      "Amazon primary image standard, 1:1, high-definition, pure white background RGB(255,255,255), product centered, fully displayed, no crop, no shadow, no reflection, no text, no watermark, no logo, soft even lighting, clear material texture, clean and simple, ready to upload.",
  },
  {
    role: "scene",
    label: "场景图",
    hint: "基于真实产品类型生成自然使用场景，画面干净，不加杂乱文字。",
    defaultPrompt:
      "Realistic natural usage scene based on the actual product, product as visual center, no clutter, soft lighting, unified color tone, clear material texture, no text, no watermark, no brand, no logo, shows product scene compatibility.",
  },
  {
    role: "selling_point",
    label: "卖点图",
    hint: "展示 3-4 个简短卖点，可带简洁图标，商品清晰可见，不写夸大承诺。",
    defaultPrompt:
      "White or light gray clean background, product centered or left, 3-4 key selling points on right or bottom, each with minimal icon and short text, clear font, high contrast, text/icons do not block product, unified tone, no watermark, no brand, no logo.",
  },
  {
    role: "detail",
    label: "细节图",
    hint: "展示印刷质量、材质纹理或做工细节，可加少量说明。",
    defaultPrompt:
      "Focus on key product details, magnified and sharp, visible material texture, white or light gray background, optional short text 1-2 lines plus small minimal icon, icon does not cover details, unified tone, no clutter, no watermark, no brand, no logo.",
  },
  {
    role: "dimension",
    label: "尺寸图",
    hint: "优先使用 SDS 尺寸参考图；如展示尺寸，只允许准确英寸单位。",
    defaultPrompt:
      "Use SDS size reference mockup or accurate SDS dimensions only. Product fully displayed, pure white background, clean high-contrast dimension lines and numbers, no crop, no shadow, no reflection, no clutter, no watermark, no logo. If dimensions are displayed, show inches only and do not invent measurements.",
  },
  {
    role: "angle",
    label: "角度图",
    hint: "从另一个角度展示形状、厚度或结构。",
    defaultPrompt:
      "Second useful product angle showing shape, thickness, or construction, high-definition product photography, clean background, product sharp and central, no text, no watermark, no brand, no logo.",
  },
  {
    role: "multi_scene",
    label: "多场景图",
    hint: "最多展示 3 个使用场景；必要时可加短描述或图标。",
    defaultPrompt:
      "Multiple usage scenarios combined in one image, clean layout with 3 clear areas, each shows one real usage scenario, product clearly visible, optional minimal icon plus one short description text per scenario, unified color, text/icons do not cover product, clean background, no clutter, no watermark.",
  },
  {
    role: "material",
    label: "材质图",
    hint: "展示表面质感、面料、印刷区域或触感特点。",
    defaultPrompt:
      "Highlight material, surface finish, fabric, print area, or tactile quality, macro or close detail, realistic lighting, clean background, optional minimal callout only if useful, no clutter, no watermark, no brand, no logo.",
  },
  {
    role: "character_scene",
    label: "人物场景图",
    hint: "可选。只有产品适合穿戴、手持或真人演示时才使用人物。",
    defaultPrompt:
      "Optional human usage scene. Use a real person only if the product is naturally worn, held, used, or demonstrated by a person; otherwise create a normal product scene. Natural expression, normal posture, product clear, optional simple 2-3 lines text plus minimal icon, text/icon do not cover product or person, no clutter, no watermark, no brand, no logo.",
  },
] as const;

export function normalizeImageStrategy(value: unknown): SheinStudioImageStrategy {
  return value === "sds_official" || value === "hybrid" || value === "ai_generated"
    ? value
    : "sds_official";
}

export function normalizeArtworkModel(value: unknown): SheinStudioArtworkModel {
  return typeof value === "string"
    ? value.trim()
    : DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL;
}

export function normalizeGroupedImageMode(
  value: unknown,
): SheinStudioGroupedImageMode {
  return value === "per_product" || value === "shared_by_size"
    ? value
    : DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE;
}

export function normalizeVariationIntensity(
  value: unknown,
): SheinStudioVariationIntensity {
  return value === "light" || value === "medium" || value === "strong"
    ? value
    : DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY;
}

export function normalizePromptMode(value: unknown) {
  return value === "raw" ? "raw" : "managed";
}

export function normalizeArtworkGenerationMode(
  value: unknown,
): SheinStudioArtworkGenerationMode | undefined {
  return value === "hot_reference" || value === "theme_prompt"
    ? value
    : undefined;
}

function normalizeHotStyleReferenceImageUrls(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of value) {
    if (typeof item !== "string") {
      continue;
    }
    const trimmed = item.trim();
    if (!trimmed || seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    result.push(trimmed);
    if (result.length === 1) {
      break;
    }
  }
  return result;
}

function normalizeHotStyleReferenceText(value: unknown) {
  return typeof value === "string" ? value : "";
}

export function isGeneratedDesign(item: unknown): item is SheinStudioGeneratedDesign {
  return (
    !!item &&
    typeof item === "object" &&
    typeof (item as SheinStudioGeneratedDesign).id === "string" &&
    (typeof (item as SheinStudioGeneratedDesign).dataUrl === "string" ||
      typeof (item as SheinStudioGeneratedDesign).imageUrl === "string")
  );
}

function normalizeGeneratedDesign(item: SheinStudioGeneratedDesign) {
  return {
    ...item,
    targetGroupKey:
      typeof item.targetGroupKey === "string" ? item.targetGroupKey.trim() : undefined,
    targetGroupLabel:
      typeof item.targetGroupLabel === "string"
        ? item.targetGroupLabel.trim()
        : undefined,
  } satisfies SheinStudioGeneratedDesign;
}

export function dedupeGeneratedDesignsByID(
  designs: SheinStudioGeneratedDesign[],
): SheinStudioGeneratedDesign[] {
  const nextByID = new Map<string, SheinStudioGeneratedDesign>();
  for (const design of designs) {
    if (!design?.id?.trim()) {
      continue;
    }
    nextByID.set(design.id, normalizeGeneratedDesign(design));
  }
  return Array.from(nextByID.values());
}

export function isCreatedTask(item: unknown): item is SheinStudioCreatedTask {
  const raw = item as
    | (SheinStudioCreatedTask & { design_id?: string })
    | undefined;
  return (
    !!item &&
    typeof item === "object" &&
    typeof raw?.id === "string" &&
    typeof raw?.title === "string" &&
    (typeof raw?.designId === "string" || typeof raw?.design_id === "string")
  );
}

function normalizeCreatedTasks(input: unknown): SheinStudioCreatedTask[] {
  if (!Array.isArray(input)) {
    return [];
  }
  return input
    .map((item) => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const raw = item as SheinStudioCreatedTask & { design_id?: string };
      if (typeof raw.id !== "string" || typeof raw.title !== "string") {
        return null;
      }
      const designId =
        typeof raw.designId === "string"
          ? raw.designId
          : typeof raw.design_id === "string"
            ? raw.design_id
            : "";
      return { id: raw.id, title: raw.title, designId };
    })
    .filter((item): item is SheinStudioCreatedTask => Boolean(item));
}

function normalizeGenerationJobs(input: unknown): SheinStudioGenerationJob[] {
  if (!Array.isArray(input)) {
    return [];
  }
  return input.reduce<SheinStudioGenerationJob[]>((jobs, item) => {
    if (!item || typeof item !== "object") {
      return jobs;
    }
    const raw = item as Partial<SheinStudioGenerationJob> & {
      job_id?: string;
      target_group_key?: string;
      target_group_label?: string;
    };
    const jobId =
      typeof raw.jobId === "string"
        ? raw.jobId.trim()
        : typeof raw.job_id === "string"
          ? raw.job_id.trim()
          : "";
    if (!jobId) {
      return jobs;
    }
    const status =
      raw.status === "succeeded" || raw.status === "failed"
        ? raw.status
        : "running";
    jobs.push({
      jobId,
      targetGroupKey:
        typeof raw.targetGroupKey === "string"
          ? raw.targetGroupKey.trim()
          : typeof raw.target_group_key === "string"
            ? raw.target_group_key.trim()
            : undefined,
      targetGroupLabel:
        typeof raw.targetGroupLabel === "string"
          ? raw.targetGroupLabel.trim()
          : typeof raw.target_group_label === "string"
            ? raw.target_group_label.trim()
            : undefined,
      status,
    });
    return jobs;
  }, []);
}

function normalizeProductImagePrompts(input: unknown): SheinStudioProductImagePrompt[] {
  if (!Array.isArray(input)) {
    return [];
  }
  return input
    .map((item) => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const prompt = item as Partial<SheinStudioProductImagePrompt>;
      if (typeof prompt.role !== "string") {
        return null;
      }
      return {
        role: prompt.role,
        label: typeof prompt.label === "string" ? prompt.label : prompt.role,
        prompt: typeof prompt.prompt === "string" ? prompt.prompt : "",
      };
    })
    .filter((item): item is SheinStudioProductImagePrompt => Boolean(item));
}

function normalizeSelectedImages(input: unknown): SheinStudioSelectedSDSImage[] {
  return normalizeSelectedSDSImages(input);
}

function normalizeGroupedSelections(
  input: unknown,
  primarySelection?: SDSProductVariantSelection,
): GroupedSDSSelectionEligibility[] {
  if (!Array.isArray(input)) {
    return [];
  }
  return removePrimarySelectionFromGroupedSelections(
    input
      .map((item) => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const candidate = item as Partial<GroupedSDSSelectionEligibility> & {
        selection?: unknown;
      };
      return normalizeGroupedSDSSelectionEligibility({
        ...candidate,
        selection: normalizeSelection(candidate.selection),
      });
    })
      .filter((item): item is GroupedSDSSelectionEligibility => Boolean(item)),
    primarySelection,
  );
}

function normalizePromptHistory(input: unknown): SDSGroupedPromptHistoryEntry[] {
  if (!Array.isArray(input)) {
    return [];
  }
  return input
    .map((item) => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const candidate = item as Partial<SDSGroupedPromptHistoryEntry>;
      if (
        typeof candidate.prompt !== "string" ||
        typeof candidate.createdAt !== "string"
      ) {
        return null;
      }
      return {
        prompt: candidate.prompt,
        groupedImageMode: normalizeGroupedImageMode(candidate.groupedImageMode),
        createdAt: candidate.createdAt,
      } satisfies SDSGroupedPromptHistoryEntry;
    })
    .filter((item): item is SDSGroupedPromptHistoryEntry => Boolean(item))
    .slice(0, 5);
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

function resolveLegacyCompatibilitySnapshot(
  value: unknown,
): SheinStudioLegacyCompatibilitySnapshot | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  return value as SheinStudioLegacyCompatibilitySnapshot;
}

function normalizeGroupedWorkspace(
  input: unknown,
): SheinStudioGroupedWorkspace | null {
  if (!input || typeof input !== "object") {
    return null;
  }
  const candidate = input as Partial<SheinStudioGroupedWorkspace> & {
    primarySelection?: unknown;
    legacyCompatibilitySnapshot?: unknown;
  };
  const primarySelection = normalizeSelection(candidate.primarySelection);
  if (
    typeof candidate.id !== "string" ||
    !candidate.id.trim() ||
    typeof candidate.name !== "string" ||
    !candidate.name.trim() ||
    !primarySelection ||
    typeof candidate.currentPrompt !== "string" ||
    typeof candidate.updatedAt !== "string"
  ) {
    return null;
  }
  const legacyCompatibilitySnapshot = resolveLegacyCompatibilitySnapshot(
    candidate.legacyCompatibilitySnapshot,
  );
  return {
    id: candidate.id.trim(),
    name: candidate.name.trim(),
    primarySelection,
    groupedSelections: normalizeGroupedSelections(
      candidate.groupedSelections,
      primarySelection,
    ),
    styleCount:
      typeof candidate.styleCount === "string" ? candidate.styleCount : "1",
    sheinStoreId:
      typeof candidate.sheinStoreId === "string" ? candidate.sheinStoreId : "",
    imageStrategy: normalizeImageStrategy(candidate.imageStrategy),
    groupedImageMode: normalizeGroupedImageMode(candidate.groupedImageMode),
    selectedSdsImages: normalizeSelectedImages(candidate.selectedSdsImages),
    renderSizeImagesWithSds: candidate.renderSizeImagesWithSds ?? true,
    artworkGenerationMode: normalizeArtworkGenerationMode(
      candidate.artworkGenerationMode,
    ),
    currentPrompt: candidate.currentPrompt,
    promptMode: normalizePromptMode(candidate.promptMode),
    promptHistory: normalizePromptHistory(candidate.promptHistory),
    productImageCount:
      typeof candidate.productImageCount === "string"
        ? candidate.productImageCount
        : DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt:
      typeof candidate.productImagePrompt === "string"
        ? candidate.productImagePrompt
        : "",
    productImagePrompts: normalizeProductImagePrompts(candidate.productImagePrompts),
    artworkModel: normalizeArtworkModel(candidate.artworkModel),
    transparentBackground: candidate.transparentBackground ?? false,
    variationIntensity: normalizeVariationIntensity(candidate.variationIntensity),
    designs: Array.isArray(candidate.designs)
      ? candidate.designs.filter(isGeneratedDesign).map(normalizeGeneratedDesign)
      : Array.isArray(legacyCompatibilitySnapshot?.designs)
        ? legacyCompatibilitySnapshot.designs
            .filter(isGeneratedDesign)
            .map(normalizeGeneratedDesign)
      : [],
    selectedIds: Array.isArray(candidate.selectedIds)
      ? candidate.selectedIds.filter((item): item is string => typeof item === "string")
      : Array.isArray(legacyCompatibilitySnapshot?.selectedIds)
        ? legacyCompatibilitySnapshot.selectedIds.filter(
            (item): item is string => typeof item === "string",
          )
      : [],
    createdTasks: normalizeCreatedTasks(
      candidate.createdTasks ?? legacyCompatibilitySnapshot?.createdTasks,
    ),
    legacyCompatibilitySnapshot: legacyCompatibilitySnapshot ?? undefined,
    updatedAt: candidate.updatedAt,
  } satisfies SheinStudioGroupedWorkspace;
}

function normalizeGroupedWorkspaces(input: unknown): SheinStudioGroupedWorkspace[] {
  if (!Array.isArray(input)) {
    return [];
  }
  return input
    .map((item) => normalizeGroupedWorkspace(item))
    .filter((item): item is SheinStudioGroupedWorkspace => Boolean(item));
}

function buildLegacyGroupedWorkspace(
  raw: Partial<SheinStudioDraft> | Partial<SheinStudioSavedBatch>,
): SheinStudioGroupedWorkspace[] {
  const primarySelection = normalizeSelection(raw.selection);
  const prompt = typeof raw.prompt === "string" ? raw.prompt : "";
  if (!primarySelection || !prompt) {
    return [];
  }
  const dedupedGroupedSelections = normalizeGroupedSelections(
    raw.groupedSelections,
    primarySelection,
  );
  const designs = Array.isArray(raw.designs)
    ? raw.designs.filter(isGeneratedDesign).map(normalizeGeneratedDesign)
    : [];
  const selectedIds = Array.isArray(raw.selectedIds)
    ? raw.selectedIds.filter((item): item is string => typeof item === "string")
    : [];
  const createdTasks = normalizeCreatedTasks(raw.createdTasks);
  const hotStyleReferenceImageUrls = normalizeHotStyleReferenceImageUrls(
    raw.hotStyleReferenceImageUrls,
  );
  const hotStyleReferenceBrief = normalizeHotStyleReferenceText(
    raw.hotStyleReferenceBrief,
  );
  const hotStyleReferencePrompt = normalizeHotStyleReferenceText(
    raw.hotStyleReferencePrompt,
  );
  const artworkGenerationMode =
    normalizeArtworkGenerationMode(raw.artworkGenerationMode) ??
    (hotStyleReferenceImageUrls.length > 0 ||
    hotStyleReferenceBrief.trim() ||
    hotStyleReferencePrompt.trim()
      ? "hot_reference"
      : "theme_prompt");
  return [
    {
      id: `legacy-${primarySelection.parentProductId}-${primarySelection.variantId}`,
      name: primarySelection.productName || "未命名分组",
      primarySelection,
      groupedSelections: dedupedGroupedSelections,
      styleCount: typeof raw.styleCount === "string" ? raw.styleCount : "1",
      sheinStoreId: typeof raw.sheinStoreId === "string" ? raw.sheinStoreId : "",
      imageStrategy: normalizeImageStrategy(raw.imageStrategy),
      groupedImageMode: normalizeGroupedImageMode(raw.groupedImageMode),
      selectedSdsImages: normalizeSelectedImages(raw.selectedSdsImages),
      renderSizeImagesWithSds: raw.renderSizeImagesWithSds ?? true,
      artworkGenerationMode,
      currentPrompt: prompt,
      promptMode: normalizePromptMode(raw.promptMode),
      promptHistory: [],
      productImageCount:
        typeof raw.productImageCount === "string"
          ? raw.productImageCount
          : DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
      productImagePrompt:
        typeof raw.productImagePrompt === "string" ? raw.productImagePrompt : "",
      productImagePrompts: normalizeProductImagePrompts(raw.productImagePrompts),
      hotStyleReferenceImageUrls,
      hotStyleReferenceBrief,
      hotStyleReferencePrompt,
      artworkModel: normalizeArtworkModel(raw.artworkModel),
      transparentBackground: raw.transparentBackground ?? false,
      variationIntensity: normalizeVariationIntensity(raw.variationIntensity),
      designs,
      selectedIds,
      createdTasks,
      updatedAt: raw.updatedAt ?? new Date().toISOString(),
    },
  ];
}

export function normalizeDraft(raw: Partial<SheinStudioDraft> | null | undefined) {
  if (!raw) {
    return null;
  }
  const legacyRaw = raw as Partial<SheinStudioDraft> & LegacyBatchStatusCarrier;
  const hotStyleRaw = raw as Partial<SheinStudioDraft> & {
    hot_style_reference_image_urls?: unknown;
    hot_style_reference_brief?: unknown;
    hot_style_reference_prompt?: unknown;
  };
  const legacyCompatibilitySnapshot = resolveLegacyCompatibilitySnapshot(
    raw.legacyCompatibilitySnapshot,
  );

  const groups = normalizeGroupedWorkspaces(raw.groups);
  const normalizedGroups =
    groups.length > 0 ? groups : buildLegacyGroupedWorkspace(raw);

  return {
    artworkGenerationMode: normalizeArtworkGenerationMode(
      raw.artworkGenerationMode,
    ),
    prompt: typeof raw.prompt === "string" ? raw.prompt : "",
    promptMode: normalizePromptMode(raw.promptMode),
    styleCount: raw.styleCount ?? "4",
    hotStyleReferenceImageUrls: normalizeHotStyleReferenceImageUrls(
      hotStyleRaw.hot_style_reference_image_urls ?? raw.hotStyleReferenceImageUrls,
    ),
    hotStyleReferenceBrief: normalizeHotStyleReferenceText(
      hotStyleRaw.hot_style_reference_brief ?? raw.hotStyleReferenceBrief,
    ),
    hotStyleReferencePrompt: normalizeHotStyleReferenceText(
      hotStyleRaw.hot_style_reference_prompt ?? raw.hotStyleReferencePrompt,
    ),
    variationIntensity: normalizeVariationIntensity(raw.variationIntensity),
    productImageCount: raw.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: raw.productImagePrompt ?? "",
    productImagePrompts: normalizeProductImagePrompts(raw.productImagePrompts),
    artworkModel: normalizeArtworkModel(raw.artworkModel),
    transparentBackground: raw.transparentBackground ?? false,
    sheinStoreId: raw.sheinStoreId ?? "",
    imageStrategy: normalizeImageStrategy(raw.imageStrategy),
    groupedImageMode: normalizeGroupedImageMode(raw.groupedImageMode),
    selectedSdsImages: normalizeSelectedImages(raw.selectedSdsImages),
    renderSizeImagesWithSds: raw.renderSizeImagesWithSds ?? true,
    selectionVariantId: raw.selectionVariantId,
    selection: normalizeSelection(raw.selection),
    groupedSelections: normalizeGroupedSelections(
      raw.groupedSelections,
      normalizeSelection(raw.selection),
    ),
    groups: normalizedGroups,
    designs: Array.isArray(raw.designs)
      ? raw.designs.filter(isGeneratedDesign).map(normalizeGeneratedDesign)
      : Array.isArray(legacyCompatibilitySnapshot?.designs)
        ? legacyCompatibilitySnapshot.designs
            .filter(isGeneratedDesign)
            .map(normalizeGeneratedDesign)
      : [],
    selectedIds: Array.isArray(raw.selectedIds)
      ? raw.selectedIds.filter((item): item is string => typeof item === "string")
      : Array.isArray(legacyCompatibilitySnapshot?.selectedIds)
        ? legacyCompatibilitySnapshot.selectedIds.filter(
            (item): item is string => typeof item === "string",
          )
      : [],
    createdTasks: normalizeCreatedTasks(
      raw.createdTasks ?? legacyCompatibilitySnapshot?.createdTasks,
    ),
    legacyCompatibilitySnapshot: legacyCompatibilitySnapshot ?? undefined,
    generationError:
      typeof raw.generationError === "string"
        ? raw.generationError
        : legacyCompatibilitySnapshot?.generationError ?? "",
    generationJobId:
      typeof raw.generationJobId === "string"
        ? raw.generationJobId
        : legacyCompatibilitySnapshot?.generationJobId ?? "",
    generationJobs: normalizeGenerationJobs(
      raw.generationJobs ?? legacyCompatibilitySnapshot?.generationJobs,
    ),
    batchStatus:
      typeof raw.batchStatus === "string"
        ? raw.batchStatus
        : typeof legacyRaw.sessionStatus === "string"
          ? legacyRaw.sessionStatus
          : "",
    draftUpdatedAt:
      typeof raw.draftUpdatedAt === "string"
        ? raw.draftUpdatedAt
        : raw.updatedAt ?? new Date().toISOString(),
    updatedAt: raw.updatedAt ?? new Date().toISOString(),
  } satisfies SheinStudioDraft;
}

function hasOwnStorageProperty(value: object | undefined, key: PropertyKey) {
  return !!value && Object.prototype.hasOwnProperty.call(value, key);
}

export function normalizeBatch(raw: Partial<SheinStudioSavedBatch> | null | undefined) {
  if (!raw?.id || !raw.name) {
    return null;
  }
  const legacyRaw = raw as Partial<SheinStudioSavedBatch> & LegacyBatchStatusCarrier;
  const hotStyleRaw = raw as Partial<SheinStudioSavedBatch> & {
    hot_style_reference_image_urls?: unknown;
    hot_style_reference_brief?: unknown;
    hot_style_reference_prompt?: unknown;
  };
  const legacyCompatibilitySnapshot = resolveLegacyCompatibilitySnapshot(
    raw.legacyCompatibilitySnapshot,
  );

  const groups = normalizeGroupedWorkspaces(raw.groups);
  const normalizedGroups =
    groups.length > 0 ? groups : buildLegacyGroupedWorkspace(raw);
  const hasHotStyleReferenceImageUrls =
    hasOwnStorageProperty(hotStyleRaw, "hot_style_reference_image_urls") ||
    hasOwnStorageProperty(raw, "hotStyleReferenceImageUrls");
  const hasHotStyleReferenceBrief =
    hasOwnStorageProperty(hotStyleRaw, "hot_style_reference_brief") ||
    hasOwnStorageProperty(raw, "hotStyleReferenceBrief");
  const hasHotStyleReferencePrompt =
    hasOwnStorageProperty(hotStyleRaw, "hot_style_reference_prompt") ||
    hasOwnStorageProperty(raw, "hotStyleReferencePrompt");

  return {
    id: raw.id,
    tenantId: typeof raw.tenantId === "string" ? raw.tenantId : undefined,
    name: raw.name,
    artworkGenerationMode: normalizeArtworkGenerationMode(
      raw.artworkGenerationMode,
    ),
    prompt: raw.prompt ?? "",
    promptMode: normalizePromptMode(raw.promptMode),
    styleCount: raw.styleCount ?? "4",
    ...(hasHotStyleReferenceImageUrls
      ? {
          hotStyleReferenceImageUrls: normalizeHotStyleReferenceImageUrls(
            hotStyleRaw.hot_style_reference_image_urls ??
              raw.hotStyleReferenceImageUrls,
          ),
        }
      : {}),
    ...(hasHotStyleReferenceBrief
      ? {
          hotStyleReferenceBrief: normalizeHotStyleReferenceText(
            hotStyleRaw.hot_style_reference_brief ??
              raw.hotStyleReferenceBrief,
          ),
        }
      : {}),
    ...(hasHotStyleReferencePrompt
      ? {
          hotStyleReferencePrompt: normalizeHotStyleReferenceText(
            hotStyleRaw.hot_style_reference_prompt ??
              raw.hotStyleReferencePrompt,
          ),
        }
      : {}),
    variationIntensity: normalizeVariationIntensity(raw.variationIntensity),
    productImageCount: raw.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: raw.productImagePrompt ?? "",
    productImagePrompts: normalizeProductImagePrompts(raw.productImagePrompts),
    artworkModel: normalizeArtworkModel(raw.artworkModel),
    transparentBackground: raw.transparentBackground ?? false,
    sheinStoreId: raw.sheinStoreId ?? "",
    imageStrategy: normalizeImageStrategy(raw.imageStrategy),
    groupedImageMode: normalizeGroupedImageMode(raw.groupedImageMode),
    selectedSdsImages: normalizeSelectedImages(raw.selectedSdsImages),
    renderSizeImagesWithSds: raw.renderSizeImagesWithSds ?? true,
    selectionVariantId: raw.selectionVariantId,
    selection: normalizeSelection(raw.selection),
    groupedSelections: normalizeGroupedSelections(
      raw.groupedSelections,
      normalizeSelection(raw.selection),
    ),
    groups: normalizedGroups,
    designs: Array.isArray(raw.designs)
      ? raw.designs.filter(isGeneratedDesign).map(normalizeGeneratedDesign)
      : Array.isArray(legacyCompatibilitySnapshot?.designs)
        ? legacyCompatibilitySnapshot.designs
            .filter(isGeneratedDesign)
            .map(normalizeGeneratedDesign)
      : [],
    persistedDesignCount:
      typeof raw.persistedDesignCount === "number"
        ? raw.persistedDesignCount
        : undefined,
    selectedIds: Array.isArray(raw.selectedIds)
      ? raw.selectedIds.filter((item): item is string => typeof item === "string")
      : Array.isArray(legacyCompatibilitySnapshot?.selectedIds)
        ? legacyCompatibilitySnapshot.selectedIds.filter(
            (item): item is string => typeof item === "string",
          )
      : [],
    createdTasks: normalizeCreatedTasks(
      raw.createdTasks ?? legacyCompatibilitySnapshot?.createdTasks,
    ),
    legacyCompatibilitySnapshot: legacyCompatibilitySnapshot ?? undefined,
    generationError:
      typeof raw.generationError === "string"
        ? raw.generationError
        : legacyCompatibilitySnapshot?.generationError ?? "",
    generationJobId:
      typeof raw.generationJobId === "string"
        ? raw.generationJobId
        : legacyCompatibilitySnapshot?.generationJobId ?? "",
    generationJobs: normalizeGenerationJobs(
      raw.generationJobs ?? legacyCompatibilitySnapshot?.generationJobs,
    ),
    batchStatus:
      typeof raw.batchStatus === "string"
        ? raw.batchStatus
        : typeof legacyRaw.sessionStatus === "string"
          ? legacyRaw.sessionStatus
          : "",
    draftUpdatedAt:
      typeof raw.draftUpdatedAt === "string"
        ? raw.draftUpdatedAt
        : raw.updatedAt ?? new Date().toISOString(),
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
  if (!selection) {
    return undefined;
  }

  return {
    productId: selection.productId,
    parentProductId: selection.parentProductId,
    variantId: selection.variantId,
    prototypeGroupId: selection.prototypeGroupId,
    layerId: selection.layerId,
    productName: selection.productName,
    variantLabel: selection.variantLabel,
    printableWidth: selection.printableWidth,
    printableHeight: selection.printableHeight,
    templateImageUrl: selection.templateImageUrl,
    maskImageUrl: selection.maskImageUrl,
    blankDesignUrl: selection.blankDesignUrl,
    mockupImageUrl: selection.mockupImageUrl,
    mockupImageUrls: selection.mockupImageUrls,
    sizeReferenceImageUrls: selection.sizeReferenceImageUrls,
    productSize: selection.productSize,
    packagingSpecification: selection.packagingSpecification,
    selectedVariantIds: selection.selectedVariantIds,
    variants: selection.variants?.map((variant) => ({
      variantId: variant.variantId,
      variantSku: variant.variantSku,
      size: variant.size,
      color: variant.color,
      price: variant.price,
      weight: variant.weight,
      boxLength: variant.boxLength,
      boxWidth: variant.boxWidth,
      boxHeight: variant.boxHeight,
      productionCycle: variant.productionCycle,
      prototypeGroupId: variant.prototypeGroupId,
      layerId: variant.layerId,
      mockupImageUrl: variant.mockupImageUrl,
    })),
  } satisfies SDSProductVariantSelection;
}

export function deriveBatchName(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return "未命名批次";
  }
  return trimmed.length > 36 ? `${trimmed.slice(0, 36)}...` : trimmed;
}
