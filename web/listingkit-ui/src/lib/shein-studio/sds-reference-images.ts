import type { SDSProductVariantSelection } from "@/lib/types/sds";

function addUnique(urls: string[], seen: Set<string>, url?: string) {
  const value = url?.trim();
  if (!value || seen.has(value)) {
    return;
  }
  seen.add(value);
  urls.push(value);
}

function addVariantReference(
  urls: string[],
  seen: Set<string>,
  variant: {
    mockupImageUrls?: string[];
    mockupImageUrl?: string;
    blankDesignUrl?: string;
    templateImageUrl?: string;
    sizeReferenceImageUrls?: string[];
  },
) {
  for (const url of variant.sizeReferenceImageUrls ?? []) {
    addUnique(urls, seen, url);
  }
  for (const url of variant.mockupImageUrls ?? []) {
    addUnique(urls, seen, url);
  }
  addUnique(urls, seen, variant.mockupImageUrl);
}

export function buildSDSProductReferenceImageUrls(
  selection: SDSProductVariantSelection,
  max = 5,
) {
  const urls: string[] = [];
  const seen = new Set<string>();

  addVariantReference(urls, seen, selection);

  const colorRepresentatives = new Map<
    string,
    NonNullable<SDSProductVariantSelection["variants"]>[number]
  >();
  for (const variant of selection.variants ?? []) {
    const colorKey = (variant.color || "default").trim().toLowerCase();
    if (!colorRepresentatives.has(colorKey)) {
      colorRepresentatives.set(colorKey, variant);
    }
  }
  for (const variant of colorRepresentatives.values()) {
    addVariantReference(urls, seen, variant);
  }

  addUnique(urls, seen, selection.blankDesignUrl);
  addUnique(urls, seen, selection.templateImageUrl);
  for (const variant of colorRepresentatives.values()) {
    addUnique(urls, seen, variant.blankDesignUrl);
    addUnique(urls, seen, variant.templateImageUrl);
  }

  return urls.slice(0, max);
}

export function buildSDSVariantReferenceImageUrls(
  variant: NonNullable<SDSProductVariantSelection["variants"]>[number],
  selection: SDSProductVariantSelection,
  max = 5,
) {
  const urls: string[] = [];
  const seen = new Set<string>();

  addVariantReference(urls, seen, variant);
  addUnique(urls, seen, variant.blankDesignUrl);
  addUnique(urls, seen, variant.templateImageUrl);

  addVariantReference(urls, seen, selection);
  addUnique(urls, seen, selection.blankDesignUrl);
  addUnique(urls, seen, selection.templateImageUrl);

  return urls.slice(0, max);
}
