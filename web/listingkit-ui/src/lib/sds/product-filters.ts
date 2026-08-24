import type { SDSProductSummary } from "@/lib/types/sds";

export const sdsWeightBands = [
  { value: "", label: "不限重量" },
  { value: "light", label: "0-200g" },
  { value: "medium", label: "200-500g" },
  { value: "heavy", label: "500g+" },
] as const;

export const sdsCycleBands = [
  { value: "", label: "不限周期" },
  { value: "24h", label: "<=24h" },
  { value: "48h", label: "<=48h" },
  { value: "72h", label: "<=72h" },
  { value: "72h_plus", label: ">72h" },
] as const;

export function formatWeight(product: SDSProductSummary) {
  const min = product.weightMin ?? product.minWeight ?? product.weight;
  const max = product.weightMax ?? product.weightMin ?? product.minWeight ?? product.weight;

  if (!min && !max) {
    return "-";
  }

  if (min && max && min !== max) {
    return `${min}-${max}g`;
  }

  return `${min ?? max}g`;
}

export function formatProductionCycle(product: SDSProductSummary) {
  const cycle = product.productionCycle;
  if (!cycle) {
    return "-";
  }
  return `${cycle}h`;
}
