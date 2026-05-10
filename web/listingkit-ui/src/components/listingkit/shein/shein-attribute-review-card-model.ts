import type {
  SheinPendingAttributeCandidate,
  SheinResolvedAttribute,
} from "@/lib/types/listingkit";

export function presentAttributeReviewStatus(status?: string) {
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

export function pendingCandidatesSignature(
  candidates: SheinPendingAttributeCandidate[],
) {
  return candidates
    .map((candidate) => `${candidate.attribute_id ?? ""}:${candidate.name ?? ""}`)
    .join("|");
}

export function buildSelectedAttributes(
  candidates: SheinPendingAttributeCandidate[],
  selectedValues: Record<string, string>,
): SheinResolvedAttribute[] {
  return candidates.flatMap((candidate) => {
    const key = String(candidate.attribute_id ?? candidate.name);
    const selectedValueID = Number(selectedValues[key]);
    if (!candidate.attribute_id || !selectedValueID) {
      return [];
    }
    const selectedValue = candidate.attribute_value_list?.find(
      (option) => option.attribute_value_id === selectedValueID,
    );
    return [
      {
        name: candidate.name ?? candidate.attribute_name_en ?? candidate.attribute_name,
        value:
          selectedValue?.value_en ??
          selectedValue?.value ??
          String(selectedValueID),
        attribute_id: candidate.attribute_id,
        attribute_value_id: selectedValueID,
        attribute_type: candidate.attribute_type,
        attribute_mode: candidate.attribute_mode,
        data_dimension: candidate.data_dimension,
        cascade_attribute_id: candidate.cascade_attribute_id,
        matched_by: "manual_attribute_review",
        required: candidate.required,
        important: candidate.important,
        skc_scope: candidate.skc_scope,
      },
    ];
  });
}
