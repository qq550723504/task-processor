import { createListingKitTask } from "@/lib/api/create-task";
import { generateSheinStudioProductImages } from "@/lib/api/shein-studio";
import { uploadListingKitImages } from "@/lib/api/upload-images";
import { resolveGeneratedDesignSrc } from "@/lib/shein-studio/design-image";
import {
  buildSDSProductReferenceImageUrls,
  buildSDSVariantReferenceImageUrls,
} from "@/lib/shein-studio/sds-reference-images";
import {
  type SDSListingKitVariantMetadata,
  loadSDSListingKitMetadata,
} from "@/lib/sds/product-metadata";
import type { SDSProductVariantSelection, SDSSelectedProductVariant } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariantProductImageSet,
} from "@/lib/types/shein-studio";

export const DEFAULT_SHEIN_STORE_ID = "869";
const DEFAULT_AI_PRODUCT_IMAGE_COUNT = 5;
const MAX_AI_PRODUCT_IMAGE_COUNT = 9;

export function parsePositiveInt(input: string) {
  const parsed = Number.parseInt(input.trim(), 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return undefined;
  }
  return parsed;
}

export function orderGeneratedProductImageUrls(
  images: Array<{
    imageUrl?: string;
    role?: string;
  }>,
) {
  return [...images]
    .sort((left, right) => {
      const leftPriority = left.role === "main" ? 0 : 1;
      const rightPriority = right.role === "main" ? 0 : 1;
      return leftPriority - rightPriority;
    })
    .map((image) => image.imageUrl?.trim())
    .filter((url): url is string => Boolean(url));
}

function isSDSPreviewProductImageUrl(url: string) {
  try {
    const parsed = new URL(url);
    return parsed.hostname.toLowerCase().endsWith("sdspod.com") && parsed.pathname.includes("/images/");
  } catch {
    return false;
  }
}

export function sanitizeReviewTaskProductImageUrls(
  urls: string[] | undefined,
  imageStrategy: SheinStudioImageStrategy,
) {
  if (imageStrategy === "sds_official") {
    return [];
  }

  const sanitized: string[] = [];
  const seen = new Set<string>();
  for (const rawUrl of urls ?? []) {
    const trimmed = rawUrl.trim();
    if (!trimmed || seen.has(trimmed) || isSDSPreviewProductImageUrl(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    sanitized.push(trimmed);
  }
  return sanitized;
}

export function normalizeListingKitUploadFetchUrl(url: string) {
  try {
    const parsed = new URL(url);
    const prefix = "/api/v1/listing-kits/uploads/files/";
    if (!parsed.pathname.startsWith(prefix)) {
      return url;
    }

    const proxiedPath = parsed.pathname.replace(
      prefix,
      "/api/listing-kits/uploads/files/",
    );
    return `${proxiedPath}${parsed.search}`;
  } catch {
    return url;
  }
}

function buildStyleShortId(design: SheinStudioGeneratedDesign, index: number) {
  const source = design.id || `style-${index + 1}`;
  const token = source.replace(/[^a-zA-Z0-9]/g, "").slice(0, 8).toUpperCase();
  return token || `STYLE${index + 1}`;
}

function buildStyleName(design: SheinStudioGeneratedDesign, index: number, prompt: string) {
  const source = design.revisedPrompt || prompt || `Style ${index + 1}`;
  const compact = source.trim().replace(/\s+/g, " ");
  return compact.slice(0, 64) || `Style ${index + 1}`;
}

function selectColorRepresentatives(selection: SDSProductVariantSelection) {
  const variants = selection.variants ?? [];
  const byColor = new Map<string, SDSSelectedProductVariant>();
  for (const variant of variants) {
    const color = variant.color?.trim();
    const key = (color || "default").toLowerCase();
    if (!byColor.has(key)) {
      byColor.set(key, variant);
    }
  }
  return [...byColor.values()];
}

function selectColorRepresentativesFromMetadata(
  variants: SDSListingKitVariantMetadata[] | undefined,
) {
  const byColor = new Map<string, SDSSelectedProductVariant>();
  for (const variant of variants ?? []) {
    const selected = selectedVariantFromMetadata(variant);
    const color = selected.color?.trim();
    const key = (color || "default").toLowerCase();
    if (!byColor.has(key)) {
      byColor.set(key, selected);
    }
  }
  return [...byColor.values()];
}

function selectedVariantFromMetadata(
  variant: SDSListingKitVariantMetadata,
): SDSSelectedProductVariant {
  return {
    variantId: variant.variant_id,
    variantSku: variant.variant_sku,
    size: variant.size,
    color: variant.color,
    price: variant.price,
    weight: variant.weight,
    boxLength: variant.box_length,
    boxWidth: variant.box_width,
    boxHeight: variant.box_height,
    productionCycle: variant.production_cycle,
    prototypeGroupId: variant.prototype_group_id,
    layerId: variant.layer_id,
    templateImageUrl: variant.template_image_url,
    maskImageUrl: variant.mask_image_url,
    blankDesignUrl: variant.blank_design_url,
    mockupImageUrl: variant.mockup_image_url,
    mockupImageUrls: variant.mockup_image_urls,
    sizeReferenceImageUrls: variant.size_reference_image_urls,
  };
}

async function buildDesignFile(design: SheinStudioGeneratedDesign, index: number) {
  const src = resolveGeneratedDesignSrc(design);
  if (!src) {
    throw new Error("Generated design image is missing.");
  }

  const response = await fetch(normalizeListingKitUploadFetchUrl(src), {
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(`Load generated design failed: ${response.status}`);
  }
  const blob = await response.blob();
  return new File([blob], `${design.id || `style-${index + 1}`}-design.png`, {
    type: blob.type || "image/png",
  });
}

export async function createSheinReviewTasks(input: {
  prompt: string;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  renderSizeImagesWithSds?: boolean;
  selection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  onProgress?: (message: string) => void;
}) {
  const {
    designs,
    imageStrategy = "ai_generated",
    onProgress,
    selectedSdsImages = [],
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    prompt,
    renderSizeImagesWithSds = true,
    selection,
    selectedIds,
    sheinStoreId,
  } = input;

  if (!selection?.variantId) {
    throw new Error("Select an SDS variant first.");
  }

  const approved = designs.filter((design) => selectedIds.includes(design.id));
  if (approved.length === 0) {
    throw new Error("Approve at least one style before creating SHEIN tasks.");
  }

  const storeID =
    parsePositiveInt(sheinStoreId) ?? parsePositiveInt(DEFAULT_SHEIN_STORE_ID);
  const productImageTotal = Math.min(
    MAX_AI_PRODUCT_IMAGE_COUNT,
    parsePositiveInt(productImageCount ?? "") ?? DEFAULT_AI_PRODUCT_IMAGE_COUNT,
  );
  const created: SheinStudioCreatedTask[] = [];
  onProgress?.("Loading SDS product metadata...");
  const sdsMetadata = await loadSDSListingKitMetadata({
    parentProductId: selection.parentProductId,
    variantId: selection.variantId,
    selectedVariants: selection.variants,
    selectedVariantIds: selection.selectedVariantIds,
  });

  for (let index = 0; index < approved.length; index += 1) {
    const styleId = buildStyleShortId(approved[index], index);
    const styleName = buildStyleName(approved[index], index, prompt);
    onProgress?.(`Uploading approved style ${index + 1} of ${approved.length}...`);
    const reviewFiles = [await buildDesignFile(approved[index], index)];
    const uploaded = await uploadListingKitImages(reviewFiles);
    const styleImageURLs = uploaded.image_urls ?? [];
    if (styleImageURLs.length === 0) {
      throw new Error("Uploaded review image URLs are missing.");
    }
    let productImageURLs = sanitizeReviewTaskProductImageUrls(
      approved[index].productImageUrls,
      imageStrategy,
    );
    let variantProductImages: SheinStudioVariantProductImageSet[] = [];
    if (imageStrategy === "ai_generated" || imageStrategy === "hybrid") {
      onProgress?.(
        `Generating AI product images for style ${index + 1} of ${approved.length}...`,
      );
      const generatedProductImages = await generateSheinStudioProductImages({
        prompt: prompt.trim(),
        productName: sdsMetadata.product_name ?? selection.productName,
        categoryPath: sdsMetadata.category_path,
        styleName,
        sourceDesignUrl: styleImageURLs[0],
        productReferenceImageUrls: buildSDSProductReferenceImageUrls(selection),
        customPrompt: productImagePrompt,
        imagePrompts: productImagePrompts,
        count: productImageTotal,
      });
      productImageURLs = sanitizeReviewTaskProductImageUrls(
        orderGeneratedProductImageUrls(generatedProductImages.images),
        imageStrategy,
      );
      if (productImageURLs.length === 0) {
        throw new Error("AI product image URLs are missing.");
      }

      const metadataColorRepresentatives = selectColorRepresentativesFromMetadata(
        sdsMetadata.variants,
      );
      const colorRepresentatives =
        metadataColorRepresentatives.length > 0
          ? metadataColorRepresentatives
          : selectColorRepresentatives(selection);
      if (colorRepresentatives.length > 1) {
        variantProductImages = [
          {
            variantSku: colorRepresentatives[0].variantSku,
            color: colorRepresentatives[0].color,
            imageUrls: productImageURLs,
          },
        ];
        for (const variant of colorRepresentatives.slice(1)) {
          const colorLabel = variant.color?.trim() || "this color variant";
          onProgress?.(
            `Generating ${colorLabel} SKC image for style ${index + 1} of ${approved.length}...`,
          );
          const generatedVariantImages = await generateSheinStudioProductImages({
            prompt: prompt.trim(),
            productName: sdsMetadata.product_name ?? selection.productName,
            categoryPath: sdsMetadata.category_path,
            styleName: `${styleName} ${colorLabel}`,
            sourceDesignUrl: styleImageURLs[0],
            productReferenceImageUrls: buildSDSVariantReferenceImageUrls(
              variant,
              selection,
            ),
            customPrompt: [
              productImagePrompt?.trim(),
              `Generate the product image for the SDS color variant "${colorLabel}". Keep the approved artwork identical, but match the base product color and material from this variant's SDS reference image.`,
            ]
              .filter(Boolean)
              .join("\n"),
            imagePrompts: productImagePrompts,
            count: productImageTotal,
          });
          const imageUrls = orderGeneratedProductImageUrls(generatedVariantImages.images);
          if (imageUrls.length > 0) {
            variantProductImages.push({
              variantSku: variant.variantSku,
              color: variant.color,
              imageUrls,
            });
          }
        }
      }
    }

    onProgress?.(`Creating SHEIN data task ${index + 1} of ${approved.length}...`);
    const task = await createListingKitTask({
      text: prompt.trim(),
      image_urls: styleImageURLs,
      platforms: ["shein"],
      ...(storeID ? { shein_store_id: storeID } : {}),
      options: {
        image_strategy: imageStrategy,
        process_images: false,
        shein_studio: {
          style_id: styleId,
          style_name: styleName,
          source_design_urls: styleImageURLs,
          source_design_width: approved[index].sourceWidth,
          source_design_height: approved[index].sourceHeight,
          product_image_urls: productImageURLs,
          selected_sds_images: selectedSdsImages.map((item) => ({
            image_url: item.imageUrl,
            variant_sku: item.variantSku,
            color: item.color,
          })),
          variant_product_images: variantProductImages.map((item) => ({
            variant_sku: item.variantSku,
            color: item.color,
            image_urls: item.imageUrls,
          })),
          size_reference_image_urls: sdsMetadata.size_reference_image_urls,
          render_size_images_with_sds: renderSizeImagesWithSds,
        },
        sds: {
          ...sdsMetadata,
          variant_id: selection.variantId,
          parent_product_id: selection.parentProductId,
          prototype_group_id: selection.prototypeGroupId,
          layer_id: selection.layerId,
          blank_design_url: selection.blankDesignUrl ?? sdsMetadata.blank_design_url,
          template_image_url: selection.templateImageUrl ?? sdsMetadata.template_image_url,
          mask_image_url: selection.maskImageUrl ?? sdsMetadata.mask_image_url,
          printable_width: selection.printableWidth,
          printable_height: selection.printableHeight,
          design_type: "material",
          fit_level: 1,
          resize_mode: 0,
          style_id: styleId,
          style_name: styleName,
          variants: sdsMetadata.variants,
        },
      },
    });

    created.push({
      id: task.task_id,
      title: `Style ${index + 1}`,
      designId: approved[index].id,
    });
  }

  if (created.length === 0) {
    throw new Error("No SHEIN data tasks were created.");
  }
  onProgress?.(`Created ${created.length} SHEIN data task${created.length === 1 ? "" : "s"}.`);
  return created;
}
