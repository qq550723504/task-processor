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

export function normalizeSDSBaselineStatus(value: unknown): SDSBaselineStatus {
  return value === "ready" || value === "failed" || value === "missing"
    ? value
    : "missing";
}

export function normalizeGroupedSDSSelectionEligibility(
  item: Partial<GroupedSDSSelectionEligibility> | null | undefined,
): GroupedSDSSelectionEligibility | null {
  if (!item?.selection) {
    return null;
  }
  const selectionId =
    typeof item.selectionId === "string" && item.selectionId.trim()
      ? item.selectionId
      : buildGroupedSDSSelectionID(item.selection);
  if (!selectionId) {
    return null;
  }
  return {
    selectionId,
    selection: item.selection,
    baselineKey:
      typeof item.baselineKey === "string" && item.baselineKey.trim()
        ? item.baselineKey
        : undefined,
    baselineStatus: normalizeSDSBaselineStatus(item.baselineStatus),
    baselineReason:
      typeof item.baselineReason === "string" ? item.baselineReason : "",
    sheinStoreId:
      typeof item.sheinStoreId === "string" ? item.sheinStoreId : "",
    eligible: item.eligible !== false,
    eligibilityReason:
      typeof item.eligibilityReason === "string"
        ? item.eligibilityReason
        : undefined,
  };
}
