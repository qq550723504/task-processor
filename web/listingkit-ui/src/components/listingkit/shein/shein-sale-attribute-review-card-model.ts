import type { SheinSaleAttributeCandidateInfo } from "@/lib/types/listingkit";

export function presentSaleReviewStatus(status?: string) {
  switch (status) {
    case "resolved":
      return "已完成";
    case "partial":
      return "待补齐";
    case "blocked":
      return "有阻断";
    default:
      return status;
  }
}

export function matchingCandidates(
  candidates: SheinSaleAttributeCandidateInfo[],
  attributeID?: number,
  scope?: string,
) {
  return candidates.filter((candidate) => {
    const matchesAttribute = attributeID
      ? candidate.attribute_id === attributeID
      : false;
    return matchesAttribute || candidate.selected_scope === scope;
  });
}

export function scopeLabel(scope: string) {
  switch (scope) {
    case "primary":
      return "主规格";
    case "secondary":
      return "其他规格";
    case "skc":
      return "主规格/SKC";
    case "sku":
      return "其他规格/SKU";
    default:
      return scope;
  }
}
