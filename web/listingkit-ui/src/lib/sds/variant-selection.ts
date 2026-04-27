import type {
  SDSProductDetail,
  SDSSelectedProductVariant,
  SDSProductVariant,
  SDSProductVariantSelection,
} from "@/lib/types/sds";

function resolveLayerId(variant: SDSProductVariant) {
  const layers = variant.designPrototype?.prototypeLayerList ?? [];
  return layers.find((layer) => layer.isMasterMap === 1)?.id ?? layers[0]?.id ?? "";
}

function resolvePrimaryLayer(variant: SDSProductVariant) {
  const layers = variant.designPrototype?.prototypeLayerList ?? [];
  return layers.find((layer) => layer.isMasterMap === 1) ?? layers[0];
}

function resolvePrimaryMockup(variant: SDSProductVariant) {
  const groups = variant.designPrototype?.prototypeResultGroups ?? [];
  return groups.find((group) => group.faceSheetState)?.resultImage ?? groups[0]?.resultImage;
}

function resolveMockupImages(variant: SDSProductVariant) {
  const groups = [...(variant.designPrototype?.prototypeResultGroups ?? [])]
    .sort((left, right) => (left.sort ?? 0) - (right.sort ?? 0))
    .map((group) => group.resultImage)
    .filter((image): image is string => Boolean(image));

  return groups.length > 0 ? groups : [];
}

function resolveSizeReferenceImages(variant: SDSProductVariant) {
  const groups = [...(variant.designPrototype?.prototypeResultGroups ?? [])]
    .sort((left, right) => (left.sort ?? 0) - (right.sort ?? 0));
  const preferred = groups
    .filter((group) => isLikelySizeReferenceGroup(group))
    .map((group) => group.resultImage)
    .filter((image): image is string => Boolean(image));
  if (preferred.length > 0) {
    return preferred;
  }
  return [];
}

function isLikelySizeReferenceGroup(group: { sort?: number; resultImage?: string }) {
  const url = group.resultImage?.toLowerCase() ?? "";
  return (
    url.includes("size") ||
    url.includes("measure") ||
    url.includes("dimension") ||
    url.includes("spec")
  );
}

export function buildSDSSelectedVariant(
  detail: SDSProductDetail | undefined,
  variant: SDSProductVariant,
): SDSSelectedProductVariant {
  const primaryLayer = resolvePrimaryLayer(variant);
  return {
    variantId: variant.id,
    variantSku: variant.sku,
    size: variant.size,
    color: variant.color_name,
    price: variant.currentPrice,
    weight: variant.weight,
    boxLength: variant.box_length,
    boxWidth: variant.box_width,
    boxHeight: variant.box_height,
    productionCycle: variant.productionCycle,
    prototypeGroupId: variant.designPrototype?.prototypeGroupId,
    layerId: resolveLayerId(variant),
    templateImageUrl: primaryLayer?.thumbnailUrl ?? primaryLayer?.imageUrl,
    maskImageUrl: primaryLayer?.maskUrl ?? primaryLayer?.maskShowUrl ?? primaryLayer?.maskThumbnailUrl,
    blankDesignUrl: detail?.blankDesignUrl,
    mockupImageUrl: resolvePrimaryMockup(variant),
    mockupImageUrls: resolveMockupImages(variant),
    sizeReferenceImageUrls: resolveSizeReferenceImages(variant),
  };
}

export function buildSDSVariantSelection(
  detail: SDSProductDetail | undefined,
  variant: SDSProductVariant,
  selectedVariants?: SDSProductVariant[],
): SDSProductVariantSelection {
  const productId = detail?.id ?? variant.parent_id ?? 0;
  const primaryLayer = resolvePrimaryLayer(variant);
  const variants =
    selectedVariants && selectedVariants.length > 0
      ? selectedVariants.map((item) => buildSDSSelectedVariant(detail, item))
      : [buildSDSSelectedVariant(detail, variant)];
  return {
    productId,
    parentProductId: productId,
    variantId: variant.id,
    variants,
    selectedVariantIds: variants.map((item) => item.variantId),
    prototypeGroupId: variant.designPrototype?.prototypeGroupId ?? 0,
    layerId: resolveLayerId(variant),
    productName: detail?.name ?? "SDS product",
    variantLabel: `${variant.size || "One size"} · ${variant.color_name || "default"}`,
    printableWidth: primaryLayer?.printWidth,
    printableHeight: primaryLayer?.printHeight,
    templateImageUrl: primaryLayer?.thumbnailUrl ?? primaryLayer?.imageUrl,
    maskImageUrl: primaryLayer?.maskUrl ?? primaryLayer?.maskShowUrl ?? primaryLayer?.maskThumbnailUrl,
    blankDesignUrl: detail?.blankDesignUrl,
    mockupImageUrl: resolvePrimaryMockup(variant),
    mockupImageUrls: resolveMockupImages(variant),
    sizeReferenceImageUrls: resolveSizeReferenceImages(variant),
  };
}
