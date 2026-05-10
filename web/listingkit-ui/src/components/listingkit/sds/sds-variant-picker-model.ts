import type { SDSProductVariant } from "@/lib/types/sds";

export function formatVariantWeight(value?: number) {
  if (!value) {
    return "-";
  }
  return `${value}g`;
}

export function buildSDSVariantOptions(
  variants: SDSProductVariant[],
  field: "color_name" | "size",
) {
  return Array.from(
    new Set(
      variants
        .map((variant) => variant[field])
        .filter((value): value is string => Boolean(value)),
    ),
  ).sort((a, b) => String(a).localeCompare(String(b)));
}

export function filterSDSVariants({
  colorFilter,
  sizeFilter,
  variants,
}: {
  colorFilter: string;
  sizeFilter: string;
  variants: SDSProductVariant[];
}) {
  return variants.filter((variant) => {
    if (sizeFilter && variant.size !== sizeFilter) {
      return false;
    }
    if (colorFilter && variant.color_name !== colorFilter) {
      return false;
    }
    return true;
  });
}

export function summarizeSelectedSDSVariants(
  selectedVariants: SDSProductVariant[],
) {
  return {
    selectedColors: Array.from(
      new Set(selectedVariants.map((variant) => variant.color_name || "default")),
    ),
    selectedSizes: Array.from(
      new Set(selectedVariants.map((variant) => variant.size || "One size")),
    ),
  };
}
