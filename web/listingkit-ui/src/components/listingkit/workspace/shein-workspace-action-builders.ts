import type { ApplyRevisionRequest } from "@/lib/api/revision";
import type {
  SheinManualCategoryCandidate,
  SheinPreviewPayload,
  SheinResolvedAttribute,
} from "@/lib/types/listingkit";

export function buildApplySuggestedSheinCategoryRevision(
  sheinPreview?: SheinPreviewPayload,
): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.category?.current;
  const suggested = current?.suggested_category;

  if (!suggested?.category_id) {
    return null;
  }

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Apply suggested SHEIN category",
    shein: {
      category_resolution: {
        category_id: suggested.category_id,
        category_id_list: suggested.category_id_list,
        product_type_id: suggested.product_type_id,
        top_category_id: suggested.top_category_id,
        matched_path: suggested.matched_path,
        source: suggested.source,
        status: "resolved",
      },
      sale_attribute_resolution: {
        recommend_category_review: false,
        category_review_reason: "",
      },
    },
  };
}

export function buildConfirmCurrentSheinCategoryRevision(
  sheinPreview?: SheinPreviewPayload,
): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.category?.current;

  if (!current?.category_id) {
    return null;
  }

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Confirm current SHEIN category",
    shein: {
      category_resolution: {
        category_id: current.category_id,
        category_id_list: current.category_id_list,
        product_type_id: current.product_type_id,
        top_category_id: current.top_category_id,
        matched_path: current.category_path,
        source: current.source ?? "manual_confirm",
        status: "resolved",
      },
      sale_attribute_resolution: {
        recommend_category_review: false,
        category_review_reason: "",
      },
    },
  };
}

export function buildApplyManualSheinCategoryRevision(
  candidate: SheinManualCategoryCandidate,
): ApplyRevisionRequest {
  return {
    platform: "shein",
    actor: "workspace",
    reason: "Apply manual SHEIN category",
    shein: {
      category_resolution: {
        category_id: candidate.category_id,
        category_id_list: candidate.category_id_list,
        product_type_id: candidate.product_type_id,
        top_category_id: candidate.top_category_id,
        matched_path: candidate.category_path,
        source: candidate.source ?? "manual_search",
        status: "resolved",
      },
      sale_attribute_resolution: {
        recommend_category_review: false,
        category_review_reason: "",
      },
    },
  };
}

export function buildConfirmSheinAttributesRevision({
  attributes,
  sheinPreview,
}: {
  attributes: SheinResolvedAttribute[];
  sheinPreview?: SheinPreviewPayload;
}): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.attributes?.current;
  if (!current || attributes.length === 0) {
    return null;
  }
  const resolvedAttributes = [
    ...(current.resolved_attributes ?? []),
    ...attributes,
  ];
  const selectedIDs = new Set(
    attributes
      .map((attribute) => attribute.attribute_id)
      .filter((attributeID): attributeID is number => Boolean(attributeID)),
  );
  const pendingAttributeCandidates =
    current.pending_attribute_candidates?.filter(
      (candidate) => !selectedIDs.has(candidate.attribute_id ?? 0),
    ) ?? [];
  const recommendedAttributeCandidates =
    current.recommended_attribute_candidates?.filter(
      (candidate) => !selectedIDs.has(candidate.attribute_id ?? 0),
    ) ?? [];
  const pendingAttributes =
    current.pending_attributes?.filter((attribute) => {
      const matchingCandidate = current.pending_attribute_candidates?.find(
        (candidate) => candidate.name === attribute.name,
      );
      return (
        !matchingCandidate?.attribute_id ||
        !selectedIDs.has(matchingCandidate.attribute_id)
      );
    }) ?? [];

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Apply SHEIN attribute candidate selections",
    shein: {
      attribute_resolution: {
        status: pendingAttributeCandidates.length === 0 ? "resolved" : "partial",
        source: "manual_review",
        category_id: sheinPreview?.category_id,
        template_count: current.template_count,
        resolved_count: resolvedAttributes.length,
        unresolved_count: pendingAttributeCandidates.length,
        resolved_attributes: resolvedAttributes,
        pending_attributes: pendingAttributes,
        pending_attribute_candidates: pendingAttributeCandidates,
        recommended_attribute_candidates: recommendedAttributeCandidates,
        review_notes:
          pendingAttributeCandidates.length === 0
            ? ["SHEIN 普通属性已人工确认"]
            : current.review_notes,
      },
    },
  };
}

export function buildConfirmSheinFallbackAttributesRevision(
  sheinPreview?: SheinPreviewPayload,
): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.attributes?.current;
  if (!current) {
    return null;
  }
  const resolvedCount =
    current.resolved_count ??
    current.resolved_attributes?.length ??
    current.pending_attributes?.length ??
    0;
  if (resolvedCount <= 0 && !(current.review_notes?.length ?? 0)) {
    return null;
  }

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Confirm SHEIN fallback attributes for internal testing",
    shein: {
      attribute_resolution: {
        status: "resolved",
        source: "manual_fallback_review",
        category_id: sheinPreview?.category_id,
        template_count: current.template_count,
        resolved_count: Math.max(resolvedCount, 1),
        unresolved_count: 0,
        pending_attributes: [],
        pending_attribute_candidates: [],
        recommended_attribute_candidates: [],
        review_notes: [
          "内部测试已按当前 SDS 属性确认；当前未写入真实 SHEIN attribute_id，正式发布前建议重新获取模板后复核。",
        ],
      },
    },
  };
}

export function buildConfirmCurrentSheinSaleAttributesRevision(
  sheinPreview?: SheinPreviewPayload,
): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.sale_attributes?.current;
  if (!current?.primary_attribute_id) {
    return null;
  }

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Confirm current SHEIN sale attributes",
    shein: {
      sale_attribute_resolution: {
        status: "resolved",
        source: "manual_review",
        recommend_category_review: false,
        category_review_reason: "",
        primary_attribute_id: current.primary_attribute_id,
        secondary_attribute_id: current.secondary_attribute_id,
        skc_attributes: current.skc_attributes ?? [],
        sku_attributes: current.sku_attributes ?? [],
        selection_summary: current.selection_summary ?? [],
        review_notes: [
          "SHEIN 销售属性已按当前主规格和其他规格人工确认。",
        ],
      },
    },
  };
}
