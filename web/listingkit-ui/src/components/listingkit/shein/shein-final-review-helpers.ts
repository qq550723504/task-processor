import type {
  SheinFinalReviewImage,
  SheinPreviewPayload,
  SheinReadinessItem,
} from "@/lib/types/listingkit";

export type ReviewSummaryItem = {
  key: "category" | "attributes" | "sale_attributes" | "images";
  title: string;
  message: string;
  status: "done" | "blocked" | "warning";
  actionLabel?: string;
};

export function money(value?: number, currency?: string) {
  if (!value || value <= 0) {
    return "-";
  }
  return `${currency ?? "USD"} ${value.toFixed(2)}`;
}

export function hasBlockingKey(items: SheinReadinessItem[], keys: string[]) {
  return items.some((item) => keys.includes(item.key ?? ""));
}

export function imageRoleCounts(images?: SheinFinalReviewImage[]) {
  const counts = {
    final: images?.filter((image) => image.final !== false).length ?? 0,
    main: 0,
    swatch: 0,
    sizeMap: 0,
    skc: 0,
    gallery: 0,
  };
  for (const image of images ?? []) {
    if (image.final === false) {
      continue;
    }
    if (image.main || image.role === "main") counts.main += 1;
    else if (image.swatch || image.role === "swatch") counts.swatch += 1;
    else if (image.size_map || image.role === "size_map") counts.sizeMap += 1;
    else if (image.role === "skc") counts.skc += 1;
    else counts.gallery += 1;
  }
  return counts;
}

export function imageRoleLabel(image: SheinFinalReviewImage) {
  if (image.main || image.role === "main") return "主图";
  if (image.role === "swatch" || image.swatch) return "色块来源";
  if (image.role === "size_map" || image.size_map) return "尺寸图";
  if (image.role === "skc") return "SKC 图";
  if (image.role === "white_bg") return "白底图";
  return "图库";
}

export function imageRoleTone(image: SheinFinalReviewImage) {
  if (image.main || image.role === "main") return "bg-zinc-950 text-white";
  if (image.role === "swatch" || image.swatch) return "bg-amber-100 text-amber-800";
  if (image.role === "size_map" || image.size_map) return "bg-sky-100 text-sky-800";
  if (image.role === "skc") return "bg-emerald-100 text-emerald-800";
  return "bg-zinc-100 text-zinc-700";
}

export function summaryTone(status: ReviewSummaryItem["status"]) {
  switch (status) {
    case "blocked":
      return "border-amber-200 bg-amber-50 text-amber-900";
    case "warning":
      return "border-sky-200 bg-sky-50 text-sky-900";
    default:
      return "border-emerald-200 bg-emerald-50 text-emerald-900";
  }
}

export function summaryStatusLabel(status: ReviewSummaryItem["status"]) {
  switch (status) {
    case "blocked":
      return "需处理";
    case "warning":
      return "建议检查";
    default:
      return "已完成";
  }
}

export function initialPriceOverrides(pricing?: SheinPreviewPayload["pricing"]) {
  return Object.fromEntries(
    pricing?.sku_prices?.map((sku) => [
      sku.supplier_sku ?? "",
      String(sku.final_price ?? ""),
    ]) ?? [],
  );
}

export function manualPriceOverridesFromStrings(
  priceOverrides: Record<string, string>,
) {
  const out: Record<string, number> = {};
  for (const [sku, value] of Object.entries(priceOverrides)) {
    const price = Number(value);
    if (sku && Number.isFinite(price) && price > 0) {
      out[sku] = price;
    }
  }
  return out;
}

export function buildFinalReviewModel({
  customerBlockingCount,
  shein,
}: {
  customerBlockingCount: number;
  shein?: SheinPreviewPayload | null;
}) {
  const finalReview = shein?.final_review;
  const pricing = shein?.pricing;
  if (!finalReview && !pricing) {
    return null;
  }

  const blockers = finalReview?.blocking_items ?? [];
  const confirmed = finalReview?.confirmed === true;
  const ready = shein?.submit_readiness?.ready === true;
  const readinessBlockers = shein?.submit_readiness?.blocking_items ?? [];
  const visibleBlockers = blockers.length ? blockers : readinessBlockers;
  const allBlockingItems = [...readinessBlockers, ...blockers];
  const blockingCount = customerBlockingCount || visibleBlockers.length;
  const imageCounts = imageRoleCounts(finalReview?.images);
  const finalImages = (finalReview?.images ?? []).filter(
    (image) => image.final !== false && image.url,
  );
  const attributeResolution = shein?.editor_context?.attributes?.current;
  const finalAttributeCount = finalReview?.attributes?.length ?? 0;
  const resolvedAttributeCount =
    finalAttributeCount || attributeResolution?.resolved_count || 0;
  const attributeBlocked = hasBlockingKey(allBlockingItems, [
    "attributes",
    "attribute_review",
  ]);
  const attributeResolvedByState =
    !attributeBlocked &&
    attributeResolution?.status === "resolved" &&
    resolvedAttributeCount > 0;
  const attributeDone =
    !attributeBlocked && (finalAttributeCount > 0 || attributeResolvedByState);
  const attributeMessage =
    finalAttributeCount > 0
      ? `已确认 ${finalAttributeCount} 个普通属性`
      : attributeResolvedByState && attributeResolution?.source === "manual_fallback_review"
        ? `已按当前 SDS 属性确认 ${resolvedAttributeCount} 个普通属性`
        : attributeResolvedByState
          ? `已确认 ${resolvedAttributeCount} 个普通属性`
          : "普通属性未展示已确认结果，建议检查必填属性。";
  const imageBlocked =
    hasBlockingKey(allBlockingItems, ["images", "final_images", "preview_product"]) ||
    imageCounts.final === 0 ||
    imageCounts.main === 0;
  const summaryItems: ReviewSummaryItem[] = [
    {
      key: "category",
      title: "类目确认",
      status: hasBlockingKey(allBlockingItems, ["category", "category_review"])
        ? "blocked"
        : finalReview?.category_id
          ? "done"
          : "warning",
      message: finalReview?.category_id
        ? `已选择类目 ${finalReview.category_id}`
        : "还没有明确的 SHEIN 类目 ID，请确认类目后再提交。",
      actionLabel: "去确认类目",
    },
    {
      key: "attributes",
      title: "普通属性",
      status: attributeBlocked ? "blocked" : attributeDone ? "done" : "warning",
      message: attributeMessage,
      actionLabel: "去确认属性",
    },
    {
      key: "sale_attributes",
      title: "销售属性",
      status: hasBlockingKey(allBlockingItems, ["sale_attributes", "variants"])
        ? "blocked"
        : (finalReview?.sale_attributes?.length ?? 0) > 0
          ? "done"
          : "warning",
      message:
        (finalReview?.sale_attributes?.length ?? 0) > 0
          ? `已映射 ${finalReview?.sale_attributes?.length ?? 0} 个销售属性`
          : "销售属性未展示已映射结果，建议检查主规格和其他规格。",
      actionLabel: "去确认销售属性",
    },
    {
      key: "images",
      title: "图片资料",
      status: imageBlocked ? "blocked" : "done",
      message: imageBlocked
        ? "主图、色块图或最终提交图片不完整，请先检查图片角色。"
        : `最终 ${imageCounts.final} 张，主图 ${imageCounts.main} 张，色块图 ${imageCounts.swatch} 张`,
      actionLabel: "去检查图片",
    },
  ];
  const submitHint = !ready
      ? `还差 ${blockingCount} 个阻断项，修复后才能提交。`
      : !confirmed
        ? "资料已通过检查，请先确认当前结果无误。"
        : "可以保存到 SHEIN 草稿箱，也可以正式发布。";

  return {
    blockingCount,
    confirmed,
    finalImages,
    finalReview,
    imageBlocked,
    imageCounts,
    pricing,
    ready,
    submitHint,
    summaryItems,
  };
}
