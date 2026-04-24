import type { SDSProductDetail, SDSProductVariant } from "@/lib/types/sds";

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
};

export async function loadSDSListingKitMetadata(input: {
  parentProductId: number;
  variantId: number;
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
  return buildSDSListingKitMetadata(detail, variant);
}

function buildSDSListingKitMetadata(
  detail: SDSProductDetail,
  variant?: SDSProductVariant,
): SDSListingKitMetadata {
  const productDetails = detail.product_details;
  const layers = variant?.designPrototype?.prototypeLayerList ?? [];
  const primaryLayer = layers.find((layer) => layer.isMasterMap === 1) ?? layers[0];
  const mockups = [...(variant?.designPrototype?.prototypeResultGroups ?? [])]
    .sort((left, right) => (left.sort ?? 0) - (right.sort ?? 0))
    .map((group) => group.resultImage)
    .filter((image): image is string => Boolean(image));

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
  };
}
