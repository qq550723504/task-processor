import type {
  SDSProductDetail,
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

export function buildSDSVariantSelection(
  detail: SDSProductDetail | undefined,
  variant: SDSProductVariant,
): SDSProductVariantSelection {
  const productId = detail?.id ?? variant.parent_id ?? 0;
  const primaryLayer = resolvePrimaryLayer(variant);
  return {
    productId,
    parentProductId: productId,
    variantId: variant.id,
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
  };
}
