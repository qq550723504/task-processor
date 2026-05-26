import { apiRequest } from "@/lib/api/client";
import type {
  SDSBaselineReadiness,
  SDSBaselineReadinessRequest,
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
