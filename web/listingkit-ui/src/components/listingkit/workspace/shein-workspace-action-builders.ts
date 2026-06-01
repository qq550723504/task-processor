import type { ApplyRevisionRequest } from "@/lib/api/revision";
import type {
  SheinInspectionSKCPatchPayload,
  SheinManualCategoryCandidate,
  SheinPreviewPayload,
  SheinResolvedAttribute,
  SheinResolvedSaleAttribute,
  SheinSaleAttributeTemplateOption,
} from "@/lib/types/listingkit";

type ApplyRevisionSKCPatch = NonNullable<
  NonNullable<ApplyRevisionRequest["shein"]>["skc_patches"]
>[number];

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

export function buildRefreshCurrentSheinCategoryRevision(
  sheinPreview?: SheinPreviewPayload,
): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.category?.current;

  if (!current?.category_id) {
    return null;
  }

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Refresh SHEIN category",
    shein: {
      category_resolution: {
        category_id: current.category_id,
        category_id_list: current.category_id_list,
        product_type_id: current.product_type_id,
        top_category_id: current.top_category_id,
        matched_path: current.category_path,
        source: "manual_refresh",
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

export function buildRegenerateSheinAttributesRevision(
  sheinPreview?: SheinPreviewPayload,
): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.attributes?.current;
  const category = sheinPreview?.editor_context?.category?.current;
  const categoryId = category?.category_id ?? sheinPreview?.category_id;

  if (!current || !categoryId) {
    return null;
  }

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Regenerate SHEIN attributes",
    shein: {
      regenerate_attributes: true,
      category_resolution: {
        category_id: categoryId,
        category_id_list: category?.category_id_list,
        product_type_id: category?.product_type_id,
        top_category_id: category?.top_category_id,
        matched_path: category?.category_path,
        source: category?.source ?? "manual_refresh",
        status: "resolved",
      },
    },
  };
}

export function buildConfirmCurrentSheinSaleAttributesRevision(
  sheinPreview?: SheinPreviewPayload,
): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.sale_attributes?.current;
  const suggestedResolution =
    sheinPreview?.editor_context?.revision_skeleton?.shein
      ?.sale_attribute_resolution;
  if (!current?.primary_attribute_id) {
    return null;
  }

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Confirm current SHEIN sale attributes",
    shein: {
      sale_attribute_resolution: {
        ...(suggestedResolution ?? {
          primary_source_dimension: current.primary_source_dimension,
          secondary_source_dimension: current.secondary_source_dimension,
          skc_value_assignments: current.skc_value_assignments,
          sku_value_assignments: current.sku_value_assignments,
        }),
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
      skc_patches: current.skc_patches ?? [],
    },
  };
}

export function buildRegenerateSheinSaleAttributesRevision(
  sheinPreview?: SheinPreviewPayload,
): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.sale_attributes?.current;
  const category = sheinPreview?.editor_context?.category?.current;
  const categoryId = category?.category_id ?? sheinPreview?.category_id;

  if (!current || !categoryId) {
    return null;
  }

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Regenerate SHEIN sale attributes",
    shein: {
      regenerate_sale_attributes: true,
      category_resolution: {
        category_id: categoryId,
        category_id_list: category?.category_id_list,
        product_type_id: category?.product_type_id,
        top_category_id: category?.top_category_id,
        matched_path: category?.category_path,
        source: category?.source ?? "manual_refresh",
        status: "resolved",
      },
    },
  };
}

export function buildApplyManualSheinSaleAttributesRevision({
  sheinPreview,
  primaryOption,
  secondaryOption,
  skcSelections,
  skuSelections,
}: {
  sheinPreview?: SheinPreviewPayload;
  primaryOption?: SheinSaleAttributeTemplateOption | null;
  secondaryOption?: SheinSaleAttributeTemplateOption | null;
  skcSelections: Record<string, { valueId?: number; textValue?: string }>;
  skuSelections: Record<string, { valueId?: number; textValue?: string }>;
}): ApplyRevisionRequest | null {
  const current = sheinPreview?.editor_context?.sale_attributes?.current;
  const skcPatches = current?.skc_patches ?? [];
  if (!current || !primaryOption?.attribute_id || skcPatches.length === 0) {
    return null;
  }

  const nextSKCPatches: ApplyRevisionSKCPatch[] = [];
  for (const patch of skcPatches) {
    const nextPatch = buildManualSaleSKCPatch({
      patch,
      primaryOption,
      secondaryOption,
      skcSelections,
      skuSelections,
    });
    if (nextPatch) {
      nextSKCPatches.push(nextPatch);
    }
  }
  if (nextSKCPatches.length === 0) {
    return null;
  }

  const firstPrimary = nextSKCPatches[0]?.sale_attribute;
  const firstSecondary = nextSKCPatches
    .flatMap((patch) => patch.sku_patches ?? [])
    .flatMap((skuPatch) => skuPatch.sale_attributes ?? [])[0];

  return {
    platform: "shein",
    actor: "workspace",
    reason: "Apply manual SHEIN sale attributes",
    shein: {
      sale_attribute_resolution: {
        status: "resolved",
        source: "manual_review",
        recommend_category_review: false,
        category_review_reason: "",
        primary_attribute_id: primaryOption.attribute_id,
        secondary_attribute_id: secondaryOption?.attribute_id,
        skc_attributes: firstPrimary ? [firstPrimary] : [],
        sku_attributes: firstSecondary ? [firstSecondary] : [],
        selection_summary: [
          "SHEIN 销售属性已由人工按当前类目模板重新填写。",
        ],
        review_notes: [
          "SHEIN 销售属性已人工填写真实 attribute_value_id。",
        ],
      },
      skc_patches: nextSKCPatches,
    },
  };
}

function buildManualSaleSKCPatch({
  patch,
  primaryOption,
  secondaryOption,
  skcSelections,
  skuSelections,
}: {
  patch: SheinInspectionSKCPatchPayload;
  primaryOption: SheinSaleAttributeTemplateOption;
  secondaryOption?: SheinSaleAttributeTemplateOption | null;
  skcSelections: Record<string, { valueId?: number; textValue?: string }>;
  skuSelections: Record<string, { valueId?: number; textValue?: string }>;
}): ApplyRevisionSKCPatch | null {
  const supplierCode = patch.supplier_code;
  if (!supplierCode) {
    return null;
  }
  const primarySelection = skcSelections[supplierCode];
  if (!hasManualSaleAttributeSelection(primarySelection)) {
    return null;
  }
  const primaryValueID = primarySelection?.valueId;
  const primaryValue = primaryOption.attribute_value_list?.find(
    (option) => option.attribute_value_id === primaryValueID,
  );
  const saleAttribute = buildManualResolvedSaleAttribute(
    primaryOption,
    primaryValueID,
    resolveManualSaleAttributeTextValue(
      primarySelection,
      primaryValue?.value_en ?? primaryValue?.value,
    ),
    "skc",
  );
  if (!saleAttribute) {
    return null;
  }

  const skuPatches =
    patch.sku_patches?.map((skuPatch) => {
      const supplierSKU = skuPatch.supplier_sku;
      if (!supplierSKU) {
        return null;
      }
      const secondarySelection = skuSelections[supplierSKU];
      const secondaryValueID = secondarySelection?.valueId;
      const secondaryValue = secondaryOption?.attribute_value_list?.find(
        (option) => option.attribute_value_id === secondaryValueID,
      );
      const saleAttributes =
        secondaryOption?.attribute_id && hasManualSaleAttributeSelection(secondarySelection)
        ? [
            buildManualResolvedSaleAttribute(
              secondaryOption,
              secondaryValueID,
              resolveManualSaleAttributeTextValue(
                secondarySelection,
                secondaryValue?.value_en ?? secondaryValue?.value,
              ),
              "sku",
            ),
          ].filter(
            (value): value is SheinResolvedSaleAttribute => Boolean(value),
          )
        : undefined;

      return {
        supplier_sku: supplierSKU,
        attributes: skuPatch.attributes,
        sale_attributes: saleAttributes,
      };
    }).filter((item): item is NonNullable<typeof item> => Boolean(item)) ?? [];

  return {
    supplier_code: supplierCode,
    skc_name: patch.skc_name,
    sale_name: patch.sale_name,
    main_image_url: patch.main_image_url,
    sale_attribute: saleAttribute,
    sku_patches: skuPatches,
  };
}

function buildManualResolvedSaleAttribute(
  option: SheinSaleAttributeTemplateOption,
  attributeValueID: number | undefined,
  value: string | undefined,
  scope: "skc" | "sku",
): SheinResolvedSaleAttribute | null {
  if (!option.attribute_id || !value) {
    return null;
  }
  return {
    scope,
    name: option.name_en ?? option.name,
    value,
    attribute_id: option.attribute_id,
    attribute_value_id: attributeValueID,
    matched_by: "manual_review",
  };
}

function hasManualSaleAttributeSelection(selection?: {
  valueId?: number;
  textValue?: string;
}) {
  return Boolean(selection?.valueId) || Boolean(selection?.textValue?.trim());
}

function resolveManualSaleAttributeTextValue(
  selection: { valueId?: number; textValue?: string } | undefined,
  fallbackValue: string | undefined,
) {
  const customText = selection?.textValue?.trim();
  if (customText) {
    return customText;
  }
  return fallbackValue;
}
