import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioArtworkModel,
  SheinStudioDraft,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
  SheinStudioStorageData,
} from "@/lib/types/shein-studio";

export const MAX_SHEIN_STUDIO_BATCHES = 12;
export const DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY: SheinStudioImageStrategy =
  "ai_generated";
export const DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT = "5";
export const DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL: SheinStudioArtworkModel =
  "nanobanana";
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
  return value === "gpt-image-2" || value === "nanobanana"
    ? value
    : DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL;
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

export function isCreatedTask(item: unknown): item is SheinStudioCreatedTask {
  return (
    !!item &&
    typeof item === "object" &&
    typeof (item as SheinStudioCreatedTask).id === "string" &&
    typeof (item as SheinStudioCreatedTask).title === "string" &&
    typeof (item as SheinStudioCreatedTask).designId === "string"
  );
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
    productImageCount: raw.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: raw.productImagePrompt ?? "",
    productImagePrompts: normalizeProductImagePrompts(raw.productImagePrompts),
    artworkModel: normalizeArtworkModel(raw.artworkModel),
    transparentBackground: raw.transparentBackground ?? false,
    sheinStoreId: raw.sheinStoreId ?? "",
    imageStrategy: normalizeImageStrategy(raw.imageStrategy),
    renderSizeImagesWithSds: raw.renderSizeImagesWithSds ?? true,
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
    productImageCount: raw.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: raw.productImagePrompt ?? "",
    productImagePrompts: normalizeProductImagePrompts(raw.productImagePrompts),
    artworkModel: normalizeArtworkModel(raw.artworkModel),
    transparentBackground: raw.transparentBackground ?? false,
    sheinStoreId: raw.sheinStoreId ?? "",
    imageStrategy: normalizeImageStrategy(raw.imageStrategy),
    renderSizeImagesWithSds: raw.renderSizeImagesWithSds ?? true,
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
