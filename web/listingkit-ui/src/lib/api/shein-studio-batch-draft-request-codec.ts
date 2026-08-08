import {
  deriveStudioBatchDraftName,
  normalizeStudioHotStyleReferenceImageUrls,
} from "@/lib/api/shein-studio-batch-draft-codec-primitives";
import { encodeStudioBatchDraftLegacySnapshot } from "@/lib/api/shein-studio-batch-draft-legacy-adapter";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioPersistedBatchView,
  SheinStudioPersistedGroupedWorkspace,
} from "@/lib/types/shein-studio";

export type UpsertSheinStudioBatchDraftInput = Omit<
  SheinStudioPersistedBatchView,
  "updatedAt" | "sheinStoreId"
> & {
  id?: string;
  expectedUpdatedAt?: string;
  name?: string;
  sheinStoreId?: string;
};

export function buildStudioBatchDraftSelectionKey(
  selection?: SDSProductVariantSelection,
) {
  if (!selection) {
    return "";
  }
  return JSON.stringify({
    productId: selection.productId,
    parentProductId: selection.parentProductId,
    variantId: selection.variantId,
    prototypeGroupId: selection.prototypeGroupId,
    layerId: selection.layerId,
    printableWidth: selection.printableWidth ?? null,
    printableHeight: selection.printableHeight ?? null,
    selectedVariantIds: selection.selectedVariantIds ?? [],
  });
}

function selectionToPayload(selection: SDSProductVariantSelection) {
  return {
    product_id: selection.productId,
    parent_product_id: selection.parentProductId,
    variant_id: selection.variantId,
    prototype_group_id: selection.prototypeGroupId,
    layer_id: selection.layerId,
    product_size: selection.productSize,
    packaging_specification: selection.packagingSpecification,
    product_name: selection.productName,
    variant_label: selection.variantLabel,
    printable_width: selection.printableWidth,
    printable_height: selection.printableHeight,
    template_image_url: selection.templateImageUrl,
    mask_image_url: selection.maskImageUrl,
    blank_design_url: selection.blankDesignUrl,
    mockup_image_url: selection.mockupImageUrl,
    mockup_image_urls: selection.mockupImageUrls,
    size_reference_image_urls: selection.sizeReferenceImageUrls,
    selected_variant_ids: selection.selectedVariantIds,
    variants: selection.variants?.map((variant) => ({
      variant_id: variant.variantId,
      variant_sku: variant.variantSku,
      size: variant.size,
      color: variant.color,
      price: variant.price,
      weight: variant.weight,
      box_length: variant.boxLength,
      box_width: variant.boxWidth,
      box_height: variant.boxHeight,
      production_cycle: variant.productionCycle,
      prototype_group_id: variant.prototypeGroupId,
      layer_id: variant.layerId,
      template_image_url: variant.templateImageUrl,
      mask_image_url: variant.maskImageUrl,
      blank_design_url: variant.blankDesignUrl,
      mockup_image_url: variant.mockupImageUrl,
      mockup_image_urls: variant.mockupImageUrls,
      size_reference_image_urls: variant.sizeReferenceImageUrls,
    })),
  };
}

function groupedSelectionToPayload(selection: GroupedSDSSelectionEligibility) {
  return {
    selection_id: selection.selectionId,
    selection: selectionToPayload(selection.selection),
    baseline_key: selection.baselineKey,
    baseline_status: selection.baselineStatus,
    baseline_reason: selection.baselineReason,
    baseline_reason_code: selection.baselineReasonCode,
    shein_store_id: selection.sheinStoreId,
    eligible: selection.eligible,
    eligibility_reason: selection.eligibilityReason,
  };
}

function groupedWorkspaceToPayload(group: SheinStudioPersistedGroupedWorkspace) {
  return {
    id: group.id,
    name: group.name,
    primary_selection: selectionToPayload(group.primarySelection),
    grouped_selections: group.groupedSelections.map(groupedSelectionToPayload),
    style_count: group.styleCount,
    shein_store_id: group.sheinStoreId,
    image_strategy: group.imageStrategy,
    grouped_image_mode: group.groupedImageMode,
    selected_sds_images: group.selectedSdsImages,
    render_size_images_with_sds: group.renderSizeImagesWithSds,
    current_prompt: group.currentPrompt,
    prompt_mode: group.promptMode,
    prompt_history: group.promptHistory.map((entry) => ({
      prompt: entry.prompt,
      grouped_image_mode: entry.groupedImageMode,
      created_at: entry.createdAt,
    })),
    product_image_count: group.productImageCount,
    product_image_prompt: group.productImagePrompt,
    product_image_prompts: group.productImagePrompts,
    artwork_model: group.artworkModel,
    transparent_background: group.transparentBackground,
    transparent_background_mode: group.transparentBackgroundMode,
    variation_intensity: group.variationIntensity,
    legacy_compatibility_snapshot: encodeStudioBatchDraftLegacySnapshot(
      group.legacyCompatibilitySnapshot,
    ),
    updated_at: group.updatedAt,
  };
}

export function buildStudioBatchDraftUpsertPayload(
  input: UpsertSheinStudioBatchDraftInput,
) {
  const explicitBatchName = input.name?.trim() || undefined;
  const batchName =
    explicitBatchName ??
    (input.id ? undefined : deriveStudioBatchDraftName(input.prompt));
  return {
    id: input.id,
    expected_updated_at: input.expectedUpdatedAt,
    batch_name: batchName,
    prompt: input.prompt,
    prompt_mode: input.promptMode,
    style_count: input.styleCount,
    hot_style_reference_image_urls: normalizeStudioHotStyleReferenceImageUrls(
      input.hotStyleReferenceImageUrls,
    ),
    hot_style_reference_brief: input.hotStyleReferenceBrief,
    hot_style_reference_prompt: input.hotStyleReferencePrompt,
    variation_intensity: input.variationIntensity,
    product_image_count: input.productImageCount,
    product_image_prompt: input.productImagePrompt,
    product_image_prompts: input.productImagePrompts,
    artwork_model: input.artworkModel,
    image_strategy: input.imageStrategy,
    grouped_image_mode: input.groupedImageMode,
    selected_sds_images: input.selectedSdsImages,
    transparent_background: input.transparentBackground,
    transparent_background_mode: input.transparentBackgroundMode,
    render_size_images_with_sds: input.renderSizeImagesWithSds,
    shein_store_id: input.sheinStoreId,
    selection: input.selection ? selectionToPayload(input.selection) : undefined,
    legacy_compatibility_snapshot: encodeStudioBatchDraftLegacySnapshot(
      input.legacyCompatibilitySnapshot,
    ),
    groups: input.groups?.map(groupedWorkspaceToPayload),
    grouped_selections: input.groupedSelections?.map(groupedSelectionToPayload),
  };
}
