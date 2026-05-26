import type { SDSProductVariantSelection } from "@/lib/types/sds";

export type SDSBaselineStatus = "ready" | "missing" | "failed";

export type SDSBaselineReadiness = {
  baselineKey: string;
  status: SDSBaselineStatus;
  reason?: string;
};

export type SDSBaselineReadinessRequest = {
  tenantId?: string;
  parentProductId: number;
  prototypeGroupId: number;
  variantId: number;
  selectedVariantIds?: number[];
};

export type GroupedSDSSelectionEligibility = {
  selectionId: string;
  selection: SDSProductVariantSelection;
  baselineKey?: string;
  baselineStatus: SDSBaselineStatus;
  baselineReason: string;
  sheinStoreId: string;
  eligible: boolean;
  eligibilityReason?: string;
};

export type GroupedSDSSelectionInput = {
  selection: SDSProductVariantSelection;
  baselineStatus: SDSBaselineStatus;
  baselineReason?: string;
};

export function buildGroupedSDSSelectionID(
  selection?: SDSProductVariantSelection,
) {
  if (!selection?.variantId) {
    return "";
  }
  const selectedVariantIDs =
    selection.selectedVariantIds?.length
      ? selection.selectedVariantIds
      : selection.variants?.map((item) => item.variantId) ?? [];
  return [
    selection.parentProductId || selection.productId || 0,
    selection.prototypeGroupId || 0,
    selection.variantId,
    selection.layerId || "",
    selectedVariantIDs.join(","),
  ].join(":");
}
