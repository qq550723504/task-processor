import type { SDSProductDetail, SDSProductVariant, SDSSelectedProductVariant } from "@/lib/types/sds";

export type SDSListingKitMetadata = {
  product_name?: string;
  product_sku?: string;
  product_english_name?: string;
  category_path?: string[];
  material?: string;
  material_description?: string;
  production_process?: string;
  product_performance?: string;
  applicable_scenarios?: string;
  washing_instructions?: string;
  special_description?: string;
  design_area?: string;
  picture_request?: string;
  is_electricity?: number;
  variant_sku?: string;
  variant_size?: string;
  variant_color?: string;
  variant_price?: number;
  variant_weight?: number;
  production_cycle?: number;
  blank_design_url?: string;
  template_image_url?: string;
  mask_image_url?: string;
  mockup_image_urls?: string[];
  size_reference_image_urls?: string[];
  variants?: SDSListingKitVariantMetadata[];
};

export type SDSListingKitVariantMetadata = {
  variant_id: number;
  variant_sku?: string;
  size?: string;
  color?: string;
  price?: number;
  weight?: number;
  box_length?: number;
  box_width?: number;
  box_height?: number;
  production_cycle?: number;
  prototype_group_id?: number;
  layer_id?: string;
  template_image_url?: string;
  mask_image_url?: string;
  blank_design_url?: string;
  mockup_image_url?: string;
  mockup_image_urls?: string[];
  size_reference_image_urls?: string[];
};

export async function loadSDSListingKitMetadata(input: {
  parentProductId: number;
  variantId: number;
  selectedVariants?: SDSSelectedProductVariant[];
  selectedVariantIds?: number[];
}): Promise<SDSListingKitMetadata> {
  if (!input.parentProductId || !input.variantId) {
    return {};
  }

  const response = await fetch(`/api/sds/products/${input.parentProductId}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    return {};
  }

  const detail = (await response.json()) as SDSProductDetail;
  const variant = detail.subproducts?.items?.find((item) => item.id === input.variantId);
  return buildSDSListingKitMetadata(
    detail,
    variant,
    input.selectedVariants,
    input.selectedVariantIds,
  );
}

function buildSDSListingKitMetadata(
  detail: SDSProductDetail,
  variant?: SDSProductVariant,
  selectedVariants?: SDSSelectedProductVariant[],
  selectedVariantIds?: number[],
): SDSListingKitMetadata {
  const productDetails = detail.product_details;
  const layers = variant?.designPrototype?.prototypeLayerList ?? [];
  const primaryLayer = layers.find((layer) => layer.isMasterMap === 1) ?? layers[0];
  const mockups = [...(variant?.designPrototype?.prototypeResultGroups ?? [])]
    .sort((left, right) => (left.sort ?? 0) - (right.sort ?? 0))
    .map((group) => group.resultImage)
    .filter((image): image is string => Boolean(image));
  const sizeReferences = resolveSizeReferenceImages(variant);
  const detailVariants = detail.subproducts?.items ?? [];
  const variantsFromIds =
    selectedVariantIds && selectedVariantIds.length > 0
      ? selectedVariantIds
          .map((id) => detailVariants.find((item) => item.id === id))
          .filter((item): item is SDSProductVariant => Boolean(item))
      : [];
  const resolvedSelectedVariants =
    selectedVariants && selectedVariants.length > 0
      ? selectedVariants
      : variantsFromIds.map((item) => toSelectedVariantMetadata(detail, item));

  return {
    product_name: detail.name,
    product_sku: detail.sku,
    product_english_name: detail.english_name,
    category_path: detail.categories?.map((category) => category.name).filter(Boolean),
    material: detail.texture?.name,
    material_description: productDetails?.material_description,
    production_process: productDetails?.production_process,
    product_performance: productDetails?.product_performance,
    applicable_scenarios: productDetails?.applicable_scenarios,
    washing_instructions: productDetails?.washing_instructions,
    special_description: productDetails?.special_description,
    design_area: productDetails?.design_area,
    picture_request: productDetails?.picture_request,
    is_electricity: detail.isElectricity,
    variant_sku: variant?.sku,
    variant_size: variant?.size,
    variant_color: variant?.color_name,
    variant_price: variant?.currentPrice,
    variant_weight: variant?.weight,
    production_cycle: variant?.productionCycle ?? detail.productionCycle,
    blank_design_url: detail.blankDesignUrl,
    template_image_url: primaryLayer?.thumbnailUrl ?? primaryLayer?.imageUrl,
    mask_image_url:
      primaryLayer?.maskUrl ?? primaryLayer?.maskShowUrl ?? primaryLayer?.maskThumbnailUrl,
    mockup_image_urls: mockups.length > 0 ? mockups : undefined,
    size_reference_image_urls: sizeReferences.length > 0 ? sizeReferences : undefined,
    variants:
      resolvedSelectedVariants.length > 0
        ? resolvedSelectedVariants.map((item) => ({
            variant_id: item.variantId,
            variant_sku: item.variantSku,
            size: item.size,
            color: item.color,
            price: item.price,
            weight: item.weight,
            box_length: item.boxLength,
            box_width: item.boxWidth,
            box_height: item.boxHeight,
            production_cycle: item.productionCycle,
            prototype_group_id: item.prototypeGroupId,
            layer_id: item.layerId,
            template_image_url: item.templateImageUrl,
            mask_image_url: item.maskImageUrl,
            blank_design_url: item.blankDesignUrl,
            mockup_image_url: item.mockupImageUrl,
            mockup_image_urls: item.mockupImageUrls,
            size_reference_image_urls: item.sizeReferenceImageUrls,
          }))
        : undefined,
  };
}

function resolveSizeReferenceImages(variant?: SDSProductVariant) {
  const groups = [...(variant?.designPrototype?.prototypeResultGroups ?? [])].sort(
    (left, right) => (left.sort ?? 0) - (right.sort ?? 0),
  );
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

function toSelectedVariantMetadata(
  detail: SDSProductDetail,
  variant: SDSProductVariant,
): SDSSelectedProductVariant {
  const layers = variant.designPrototype?.prototypeLayerList ?? [];
  const primaryLayer = layers.find((layer) => layer.isMasterMap === 1) ?? layers[0];
  const mockups = [...(variant.designPrototype?.prototypeResultGroups ?? [])]
    .sort((left, right) => (left.sort ?? 0) - (right.sort ?? 0))
    .map((group) => group.resultImage)
    .filter((image): image is string => Boolean(image));

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
    layerId: layers.find((layer) => layer.isMasterMap === 1)?.id ?? layers[0]?.id ?? "",
    templateImageUrl: primaryLayer?.thumbnailUrl ?? primaryLayer?.imageUrl,
    maskImageUrl:
      primaryLayer?.maskUrl ?? primaryLayer?.maskShowUrl ?? primaryLayer?.maskThumbnailUrl,
    blankDesignUrl: detail.blankDesignUrl,
    mockupImageUrl:
      variant.designPrototype?.prototypeResultGroups?.find((group) => group.faceSheetState)
        ?.resultImage ?? variant.designPrototype?.prototypeResultGroups?.[0]?.resultImage,
    mockupImageUrls: mockups.length > 0 ? mockups : undefined,
    sizeReferenceImageUrls: resolveSizeReferenceImages(variant),
  };
}
