import {
  loadSDSListingKitMetadata,
  type SDSListingKitMetadata,
  type SDSListingKitVariantMetadata,
} from "@/lib/sds/product-metadata";
import type {
  SDSProductVariantSelection,
  SDSSelectedProductVariant,
} from "@/lib/types/sds";

export async function hydrateSDSVariantSelection(
  selection?: SDSProductVariantSelection,
) {
  if (!selection?.parentProductId || !selection.variantId) {
    return selection;
  }

  const metadata = await loadSDSListingKitMetadata({
    parentProductId: selection.parentProductId,
    variantId: selection.variantId,
    selectedVariantIds: selection.selectedVariantIds,
  });

  return mergeSDSSelectionMetadata(selection, metadata);
}

export function mergeSDSSelectionMetadata(
  selection: SDSProductVariantSelection,
  metadata: SDSListingKitMetadata,
): SDSProductVariantSelection {
  const primaryVariant = metadata.variants?.find(
    (variant) => variant.variant_id === selection.variantId,
  );

  return {
    ...selection,
    designType: "material",
    variants: metadata.variants?.length
      ? metadata.variants.map(toSelectedVariant)
      : selection.variants,
    productSize: metadata.product_size || selection.productSize,
    packagingSpecification:
      metadata.packaging_specification || selection.packagingSpecification,
    productName: metadata.product_name || selection.productName,
    variantLabel: buildVariantLabel(primaryVariant) || selection.variantLabel,
    printableWidth: selection.printableWidth,
    printableHeight: selection.printableHeight,
    templateImageUrl: metadata.template_image_url || selection.templateImageUrl,
    maskImageUrl: metadata.mask_image_url || selection.maskImageUrl,
    blankDesignUrl: metadata.blank_design_url || selection.blankDesignUrl,
    mockupImageUrl:
      primaryVariant?.mockup_image_url ||
      metadata.mockup_image_urls?.[0] ||
      selection.mockupImageUrl,
    mockupImageUrls: metadata.mockup_image_urls || selection.mockupImageUrls,
    sizeReferenceImageUrls:
      metadata.size_reference_image_urls || selection.sizeReferenceImageUrls,
  };
}

function toSelectedVariant(
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

function buildVariantLabel(variant?: SDSListingKitVariantMetadata) {
  if (!variant) {
    return "";
  }
  return `${variant.size || "One size"} · ${variant.color || "default"}`;
}
