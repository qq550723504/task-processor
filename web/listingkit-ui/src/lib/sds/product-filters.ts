import type { SDSProductSummary } from "@/lib/types/sds";

export const sdsWeightBands = [
  { value: "", label: "Any weight" },
  { value: "light", label: "0-200g" },
  { value: "medium", label: "200-500g" },
  { value: "heavy", label: "500g+" },
] as const;

export const sdsCycleBands = [
  { value: "", label: "Any cycle" },
  { value: "24h", label: "<=24h" },
  { value: "48h", label: "<=48h" },
  { value: "72h", label: "<=72h" },
  { value: "72h_plus", label: ">72h" },
] as const;

function resolveWeight(product: SDSProductSummary) {
  return (
    product.weightMin ??
    product.weightMax ??
    product.minWeight ??
    product.weight ??
    0
  );
}

export function matchesWeightBand(product: SDSProductSummary, band?: string) {
  if (!band) {
    return true;
  }

  const weight = resolveWeight(product);
  if (band === "light") {
    return weight > 0 && weight <= 200;
  }
  if (band === "medium") {
    return weight > 200 && weight <= 500;
  }
  if (band === "heavy") {
    return weight > 500;
  }

  return true;
}

export function matchesCycleBand(product: SDSProductSummary, band?: string) {
  if (!band) {
    return true;
  }

  const cycle = product.productionCycle ?? 0;
  if (band === "24h") {
    return cycle > 0 && cycle <= 24;
  }
  if (band === "48h") {
    return cycle > 0 && cycle <= 48;
  }
  if (band === "72h") {
    return cycle > 0 && cycle <= 72;
  }
  if (band === "72h_plus") {
    return cycle > 72;
  }

  return true;
}

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
