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
  baselineStatus: SDSBaselineStatus;
  baselineReason: string;
  sheinStoreId: string;
};

export type GroupedSDSSelectionInput = {
  selection: SDSProductVariantSelection;
  baselineStatus: SDSBaselineStatus;
  baselineReason?: string;
};
