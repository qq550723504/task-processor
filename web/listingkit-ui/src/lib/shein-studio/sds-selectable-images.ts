import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { SheinStudioSelectedSDSImage } from "@/lib/types/shein-studio";
import { isUsableSDSMockupImageUrl } from "@/lib/sds/mockup-urls";

export type SheinStudioSelectableSDSImage = SheinStudioSelectedSDSImage & {
  label: string;
  description?: string;
  kind: "mockup" | "size_reference";
};

export function buildSelectableSDSImages(
  selection?: SDSProductVariantSelection,
): SheinStudioSelectableSDSImage[] {
  if (!selection) {
    return [];
  }

  const images: SheinStudioSelectableSDSImage[] = [];
  const seen = new Set<string>();

  addSelectableImages(images, seen, selection.mockupImageUrls, {
    color: undefined,
    kind: "mockup",
    labelPrefix: "当前款式",
    variantSku: undefined,
  });
  addSelectableImages(images, seen, selection.sizeReferenceImageUrls, {
    color: undefined,
    kind: "size_reference",
    labelPrefix: "当前款式尺寸图",
    variantSku: undefined,
  });

  for (const variant of selection.variants ?? []) {
    addSelectableImages(images, seen, variant.mockupImageUrls, {
      color: variant.color,
      kind: "mockup",
      labelPrefix: variant.color?.trim() || variant.size?.trim() || "变体",
      variantSku: variant.variantSku,
    });
    addSelectableImages(images, seen, variant.sizeReferenceImageUrls, {
      color: variant.color,
      kind: "size_reference",
      labelPrefix: `${variant.color?.trim() || variant.size?.trim() || "变体"}尺寸图`,
      variantSku: variant.variantSku,
    });
  }

  return images;
}

export function normalizeSelectedSDSImages(
  input: unknown,
): SheinStudioSelectedSDSImage[] {
  if (!Array.isArray(input)) {
    return [];
  }
  const seen = new Set<string>();
  const result: SheinStudioSelectedSDSImage[] = [];
  for (const item of input) {
    if (!item || typeof item !== "object") {
      continue;
    }
    const image = item as Partial<SheinStudioSelectedSDSImage>;
    const imageUrl = image.imageUrl?.trim();
    if (!imageUrl || !isUsableSelectedSDSImageUrl(imageUrl) || seen.has(imageUrl)) {
      continue;
    }
    seen.add(imageUrl);
    result.push({
      imageUrl,
      variantSku: image.variantSku?.trim() || undefined,
      color: image.color?.trim() || undefined,
    });
  }
  return result;
}

export function buildDefaultSelectedSDSImages(
  images: SheinStudioSelectableSDSImage[],
  options?: {
    includeSizeReferenceImages?: boolean;
  },
): SheinStudioSelectedSDSImage[] {
  const includeSizeReferenceImages = options?.includeSizeReferenceImages ?? true;
  const mainImage = images.find((item) => item.kind === "mockup");
  const galleryImages = includeSizeReferenceImages
    ? images.filter((item) => item.kind === "size_reference")
    : [];

  return normalizeSelectedSDSImages(
    [mainImage, ...galleryImages]
      .filter((item): item is SheinStudioSelectableSDSImage => Boolean(item))
      .map((item) => ({
        imageUrl: item.imageUrl,
        variantSku: item.variantSku,
        color: item.color,
      })),
  );
}

function addSelectableImages(
  target: SheinStudioSelectableSDSImage[],
  seen: Set<string>,
  imageUrls: string[] | undefined,
  metadata: {
    variantSku?: string;
    color?: string;
    kind: "mockup" | "size_reference";
    labelPrefix: string;
  },
) {
  for (const [index, rawUrl] of (imageUrls ?? []).entries()) {
    const imageUrl = rawUrl?.trim();
    if (metadata.kind === "mockup" && !isUsableSDSMockupImageUrl(imageUrl ?? "")) {
      continue;
    }
    if (!imageUrl || seen.has(imageUrl)) {
      continue;
    }
    seen.add(imageUrl);
    target.push({
      imageUrl,
      variantSku: metadata.variantSku,
      color: metadata.color,
      kind: metadata.kind,
      label: `${metadata.labelPrefix} · SDS 图 ${index + 1}`,
      description: metadata.variantSku
        ? `${metadata.color || "默认"} · ${metadata.variantSku}`
        : metadata.color,
    });
  }
}

function isUsableSelectedSDSImageUrl(url: string) {
  return isUsableSDSMockupImageUrl(url) || !url.includes("/images/");
}
