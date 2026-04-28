import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
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
export const SHEIN_STUDIO_PRODUCT_IMAGE_ROLES = [
  {
    role: "main",
    label: "Main image",
    hint: "Amazon primary image, pure white background, centered full product, no text/logo/shadow.",
    defaultPrompt:
      "Amazon primary image standard, 1:1, high-definition, pure white background RGB(255,255,255), product centered, fully displayed, no crop, no shadow, no reflection, no text, no watermark, no logo, soft even lighting, clear material texture, clean and simple, ready to upload.",
  },
  {
    role: "scene",
    label: "Scene image",
    hint: "Realistic natural usage scene based on the actual product type, no clutter or text.",
    defaultPrompt:
      "Realistic natural usage scene based on the actual product, product as visual center, no clutter, soft lighting, unified color tone, clear material texture, no text, no watermark, no brand, no logo, shows product scene compatibility.",
  },
  {
    role: "selling_point",
    label: "Selling point image",
    hint: "3-4 short selling points with minimal icons, product visible, no unsupported claims.",
    defaultPrompt:
      "White or light gray clean background, product centered or left, 3-4 key selling points on right or bottom, each with minimal icon and short text, clear font, high contrast, text/icons do not block product, unified tone, no watermark, no brand, no logo.",
  },
  {
    role: "detail",
    label: "Detail image",
    hint: "Close-up for print quality, material texture, or craftsmanship; optional short callout.",
    defaultPrompt:
      "Focus on key product details, magnified and sharp, visible material texture, white or light gray background, optional short text 1-2 lines plus small minimal icon, icon does not cover details, unified tone, no clutter, no watermark, no brand, no logo.",
  },
  {
    role: "dimension",
    label: "Dimension image",
    hint: "Use SDS size reference or accurate dimensions only. If dimensions are shown, inches only.",
    defaultPrompt:
      "Use SDS size reference mockup or accurate SDS dimensions only. Product fully displayed, pure white background, clean high-contrast dimension lines and numbers, no crop, no shadow, no reflection, no clutter, no watermark, no logo. If dimensions are displayed, show inches only and do not invent measurements.",
  },
  {
    role: "angle",
    label: "Alternate angle",
    hint: "Second useful angle showing shape, thickness, or construction.",
    defaultPrompt:
      "Second useful product angle showing shape, thickness, or construction, high-definition product photography, clean background, product sharp and central, no text, no watermark, no brand, no logo.",
  },
  {
    role: "multi_scene",
    label: "Multi-scenario image",
    hint: "Up to 3 usage scenarios; optional short descriptions/icons if useful.",
    defaultPrompt:
      "Multiple usage scenarios combined in one image, clean layout with 3 clear areas, each shows one real usage scenario, product clearly visible, optional minimal icon plus one short description text per scenario, unified color, text/icons do not cover product, clean background, no clutter, no watermark.",
  },
  {
    role: "material",
    label: "Material image",
    hint: "Surface finish, fabric, print area, or tactile qualities.",
    defaultPrompt:
      "Highlight material, surface finish, fabric, print area, or tactile quality, macro or close detail, realistic lighting, clean background, optional minimal callout only if useful, no clutter, no watermark, no brand, no logo.",
  },
  {
    role: "character_scene",
    label: "Character scene image",
    hint: "Optional. Use a person only if the product naturally supports human use or wearing.",
    defaultPrompt:
      "Optional human usage scene. Use a real person only if the product is naturally worn, held, used, or demonstrated by a person; otherwise create a normal product scene. Natural expression, normal posture, product clear, optional simple 2-3 lines text plus minimal icon, text/icon do not cover product or person, no clutter, no watermark, no brand, no logo.",
  },
] as const;

export function normalizeImageStrategy(value: unknown): SheinStudioImageStrategy {
  return value === "sds_official" || value === "hybrid" || value === "ai_generated"
    ? value
    : "sds_official";
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
  return selection ? selection : undefined;
}

export function deriveBatchName(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return "Untitled batch";
  }
  return trimmed.length > 36 ? `${trimmed.slice(0, 36)}...` : trimmed;
}
