import type {
  SheinPendingAttributeCandidate,
  SheinResolvedAttribute,
} from "@/lib/types/listingkit";

export type SelectedAttributeInput = {
  selectedValueId?: string;
  selectedValueIds?: string[];
  extraValue?: string;
  textValue?: string;
};

function isMultiValueCandidate(candidate: SheinPendingAttributeCandidate) {
  return (candidate.attribute_input_num ?? 1) > 1;
}

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
  selectedValues: Record<string, SelectedAttributeInput>,
): SheinResolvedAttribute[] {
  return candidates.flatMap<SheinResolvedAttribute>((candidate) => {
    const key = String(candidate.attribute_id ?? candidate.name);
    const selectedInput = selectedValues[key] ?? {};
    const rawValue = selectedInput.textValue?.trim() ?? "";
    const extraValue = selectedInput.extraValue?.trim() ?? "";
    if (!candidate.attribute_id) {
      return [];
    }
    if ((candidate.attribute_value_list?.length ?? 0) === 0) {
      if (!rawValue) {
        return [];
      }
      return [
        {
          name: candidate.name ?? candidate.attribute_name_en ?? candidate.attribute_name,
          value: rawValue,
          attribute_id: candidate.attribute_id,
          attribute_extra_value: rawValue,
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
    }
    const selectedValueIDs = isMultiValueCandidate(candidate)
      ? (selectedInput.selectedValueIds ?? [])
          .map((value) => Number(value))
          .filter((value) => value > 0)
      : [Number(selectedInput.selectedValueId ?? "")].filter((value) => value > 0);
    if (selectedValueIDs.length === 0) {
      return [];
    }
    return selectedValueIDs.map((selectedValueID) => {
      const selectedValue = candidate.attribute_value_list?.find(
        (option) => option.attribute_value_id === selectedValueID,
      );
      return {
        name: candidate.name ?? candidate.attribute_name_en ?? candidate.attribute_name,
        value:
          selectedValue?.value_en ??
          selectedValue?.value ??
          String(selectedValueID),
        attribute_id: candidate.attribute_id,
        attribute_value_id: selectedValueID,
        attribute_extra_value: extraValue || undefined,
        attribute_type: candidate.attribute_type,
        attribute_mode: candidate.attribute_mode,
        data_dimension: candidate.data_dimension,
        cascade_attribute_id: candidate.cascade_attribute_id,
        matched_by: "manual_attribute_review",
        required: candidate.required,
        important: candidate.important,
        skc_scope: candidate.skc_scope,
      };
    });
  });
}
