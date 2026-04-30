import type { SDSProductVariantSelection } from "@/lib/types/sds";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";

type SearchParamsLike = {
  get(name: string): string | null;
};

export function parseOptionalNumber(value?: string | null) {
  const parsed = Number(value ?? 0);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

export function parseOptionalStringArray(value?: string | null) {
  if (!value) {
    return undefined;
  }

  try {
    const parsed = JSON.parse(value) as unknown;
    if (!Array.isArray(parsed)) {
      return undefined;
    }

    const items = parsed.filter((item): item is string => typeof item === "string");
    return items.length > 0 ? items : undefined;
  } catch {
    return undefined;
  }
}

export function parseOptionalNumberArray(value?: string | null) {
  if (!value) {
    return undefined;
  }

  const items = value
    .split(",")
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isFinite(item) && item > 0);
  return items.length > 0 ? Array.from(new Set(items)) : undefined;
}

export function parseSheinStudioStep(value?: string | null): SheinStudioStepKey {
  if (
    value === "select" ||
    value === "generate" ||
    value === "review" ||
    value === "tasks"
  ) {
    return value;
  }
  return "select";
}

export function parseSelectionFromSearchParams(
  searchParams: SearchParamsLike,
): SDSProductVariantSelection | undefined {
  const variantId = parseOptionalNumber(searchParams.get("variantId"));
  if (!variantId) {
    return undefined;
  }

  return {
    productId: parseOptionalNumber(searchParams.get("productId")) ?? 0,
    parentProductId:
      parseOptionalNumber(searchParams.get("parentProductId")) ??
      parseOptionalNumber(searchParams.get("productId")) ??
      0,
    variantId,
    prototypeGroupId: parseOptionalNumber(searchParams.get("prototypeGroupId")) ?? 0,
    layerId: searchParams.get("layerId") ?? "",
    productName: searchParams.get("productName") ?? "已选择的 SDS 商品",
    variantLabel: searchParams.get("variantLabel") ?? "当前变体",
    printableWidth: parseOptionalNumber(searchParams.get("printWidth")),
    printableHeight: parseOptionalNumber(searchParams.get("printHeight")),
    templateImageUrl: searchParams.get("templateImageUrl") ?? undefined,
    maskImageUrl: searchParams.get("maskImageUrl") ?? undefined,
    blankDesignUrl: searchParams.get("blankDesignUrl") ?? undefined,
    mockupImageUrl: searchParams.get("mockupImageUrl") ?? undefined,
    mockupImageUrls: parseOptionalStringArray(searchParams.get("mockupImageUrls")),
    selectedVariantIds: parseOptionalNumberArray(searchParams.get("variantIds")),
  };
}
