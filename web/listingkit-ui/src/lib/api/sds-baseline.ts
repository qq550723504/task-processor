import { apiRequest } from "@/lib/api/client";
import { loadSDSListingKitMetadata } from "@/lib/sds/product-metadata";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SDSBaselineReadiness,
  SDSBaselineReadinessRequest,
  SDSBaselineWarmRequest,
} from "@/lib/types/sds-baseline";

export async function getSDSBaselineReadiness(
  input: SDSBaselineReadinessRequest,
) {
  const searchParams = new URLSearchParams();
  if (input.tenantId?.trim()) {
    searchParams.set("tenant_id", input.tenantId.trim());
  }
  searchParams.set("parent_product_id", String(input.parentProductId));
  searchParams.set("prototype_group_id", String(input.prototypeGroupId));
  searchParams.set("variant_id", String(input.variantId));
  const selectedVariantIDs = (input.selectedVariantIds ?? [])
    .map((item) => Number(item))
    .filter((item) => Number.isFinite(item) && item > 0);
  if (selectedVariantIDs.length > 0) {
    searchParams.set("selected_variant_ids", selectedVariantIDs.join(","));
  }
  return apiRequest<SDSBaselineReadiness>(
    `/sds/baselines/readiness?${searchParams.toString()}`,
    {
      method: "GET",
    },
  );
}

export async function warmSDSBaseline(input: SDSBaselineWarmRequest) {
  return apiRequest<SDSBaselineReadiness>("/sds/baselines/warm", {
    method: "POST",
    body: {
      tenant_id: input.tenantId?.trim() || undefined,
      image_urls: input.imageUrls ?? [],
      sds: input.sds,
    },
  });
}

export async function warmSDSBaselineForSelection(
  selection: SDSProductVariantSelection,
) {
  const metadata = await loadSDSListingKitMetadata({
    parentProductId: selection.parentProductId,
    variantId: selection.variantId,
    selectedVariants: selection.variants,
    selectedVariantIds: selection.selectedVariantIds,
  });
  const imageUrls = Array.from(
    new Set(
      [
        ...(selection.mockupImageUrls ?? []),
        selection.mockupImageUrl,
        ...(metadata.mockup_image_urls ?? []),
        selection.templateImageUrl,
        metadata.template_image_url,
        selection.blankDesignUrl,
        metadata.blank_design_url,
      ].filter((item): item is string => Boolean(item?.trim())),
    ),
  );
  return warmSDSBaseline({
    imageUrls,
    sds: {
      ...metadata,
      variant_id: selection.variantId,
      parent_product_id: selection.parentProductId,
      prototype_group_id: selection.prototypeGroupId,
      layer_id: selection.layerId,
      blank_design_url: selection.blankDesignUrl ?? metadata.blank_design_url,
      template_image_url: selection.templateImageUrl ?? metadata.template_image_url,
      mask_image_url: selection.maskImageUrl ?? metadata.mask_image_url,
      printable_width: selection.printableWidth,
      printable_height: selection.printableHeight,
      design_type: "material",
      fit_level: 1,
      resize_mode: 0,
      mockup_image_urls:
        selection.mockupImageUrls?.length
          ? selection.mockupImageUrls
          : metadata.mockup_image_urls,
      variants: metadata.variants,
    },
  });
}
