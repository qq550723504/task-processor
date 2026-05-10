import type {
  SheinFinalReviewImage,
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
