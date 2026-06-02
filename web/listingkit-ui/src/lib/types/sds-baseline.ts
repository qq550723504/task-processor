import type { SDSProductVariantSelection } from "@/lib/types/sds";

export type SDSBaselineStatus =
  | "baseline_cached"
  | "ready"
  | "blocked"
  | "missing"
  | "failed";

export type SDSBaselineValidationStatus =
  | "unknown"
  | "ready"
  | "blocked"
  | "failed";

export type SDSBaselineReadiness = {
  baselineKey: string;
  cacheStatus?: SDSBaselineStatus;
  validationStatus?: SDSBaselineValidationStatus;
  reasonCode?: string;
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

export type SDSBaselineWarmRequest = {
  tenantId?: string;
  imageUrls?: string[];
  sds: Record<string, unknown>;
};

export type GroupedSDSSelectionEligibility = {
  selectionId: string;
  selection: SDSProductVariantSelection;
  baselineKey?: string;
  baselineStatus: SDSBaselineStatus;
  baselineReason: string;
  baselineReasonCode?: string;
  sheinStoreId: string;
  eligible: boolean;
  eligibilityReason?: string;
};

export type GroupedSDSSelectionInput = {
  selection: SDSProductVariantSelection;
  baselineStatus: SDSBaselineStatus;
  baselineReason?: string;
  baselineReasonCode?: string;
  eligible?: boolean;
  eligibilityReason?: string;
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
  return value === "baseline_cached" ||
    value === "ready" ||
    value === "blocked" ||
    value === "failed" ||
    value === "missing"
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
    baselineReasonCode:
      typeof item.baselineReasonCode === "string" && item.baselineReasonCode.trim()
        ? item.baselineReasonCode
        : undefined,
    sheinStoreId:
      typeof item.sheinStoreId === "string" ? item.sheinStoreId : "",
    eligible: item.eligible !== false,
    eligibilityReason:
      typeof item.eligibilityReason === "string"
        ? item.eligibilityReason
        : undefined,
  };
}

function selectionsReferToSameVariant(
  left: SDSProductVariantSelection | undefined,
  right: SDSProductVariantSelection | undefined,
) {
  if (!left?.variantId || !right?.variantId) {
    return false;
  }
  return (
    (left.parentProductId || left.productId || 0) ===
      (right.parentProductId || right.productId || 0) &&
    (left.prototypeGroupId || 0) === (right.prototypeGroupId || 0) &&
    left.variantId === right.variantId &&
    (left.layerId || "") === (right.layerId || "")
  );
}

export function removePrimarySelectionFromGroupedSelections(
  groupedSelections: GroupedSDSSelectionEligibility[],
  primarySelection?: SDSProductVariantSelection,
): GroupedSDSSelectionEligibility[] {
  const seen = new Set<string>();

  return groupedSelections.filter((item) => {
    if (!item?.selection) {
      return false;
    }
    if (primarySelection && selectionsReferToSameVariant(item.selection, primarySelection)) {
      return false;
    }
    const key = item.selectionId || buildGroupedSDSSelectionID(item.selection);
    if (key && seen.has(key)) {
      return false;
    }
    if (key) {
      seen.add(key);
    }
    return true;
  });
}

export function countSelectionsWithPrimary(
  primarySelection: SDSProductVariantSelection | undefined,
  groupedSelections: GroupedSDSSelectionEligibility[],
) {
  return (
    removePrimarySelectionFromGroupedSelections(groupedSelections, primarySelection).length +
    (primarySelection?.variantId ? 1 : 0)
  );
}
