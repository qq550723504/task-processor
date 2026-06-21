import { buildGroupedSDSSelectionID } from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
} from "@/lib/types/shein-studio";

export type SheinStudioGroupedGenerationTarget = {
  key: string;
  label: string;
  selection: SDSProductVariantSelection;
  selectionIds: string[];
  selections: SDSProductVariantSelection[];
};

export function buildGroupedGenerationTargets({
  activeSelection,
  groupedSelections,
  groupedImageMode,
}: {
  activeSelection?: SDSProductVariantSelection;
  groupedSelections: SDSProductVariantSelection[];
  groupedImageMode: SheinStudioGroupedImageMode;
}) {
  const allSelections = [
    ...(activeSelection?.variantId ? [activeSelection] : []),
    ...groupedSelections,
  ];
  if (groupedImageMode === "per_product") {
    return allSelections
      .map((selection) => {
        const selectionId = buildGroupedSDSSelectionID(selection);
        if (!selectionId) {
          return null;
        }
        return {
          key: selectionId,
          label: buildPerProductLabel(selection),
          selection,
          selectionIds: [selectionId],
          selections: [selection],
        } satisfies SheinStudioGroupedGenerationTarget;
      })
      .filter(
        (item): item is SheinStudioGroupedGenerationTarget => Boolean(item),
      );
  }

  const buckets = new Map<string, SheinStudioGroupedGenerationTarget>();
  for (const selection of allSelections) {
    const selectionId = buildGroupedSDSSelectionID(selection);
    if (!selectionId) {
      continue;
    }
    const compatibilityKey = buildSharedCompatibilityGroupKey(selection);
    const key = compatibilityKey || selectionId;
    const label = compatibilityKey
      ? buildSharedBySizeGroupLabel(selection)
      : buildPerProductLabel(selection);
    const existing = buckets.get(key);
    if (existing) {
      existing.selectionIds.push(selectionId);
      existing.selections.push(selection);
      continue;
    }
    buckets.set(key, {
      key,
      label,
      selection,
      selectionIds: [selectionId],
      selections: [selection],
    });
  }
  return [...buckets.values()];
}

export function buildSharedBySizeGroupKey(selection: SDSProductVariantSelection) {
  const width = selection.printableWidth ?? 0;
  const height = selection.printableHeight ?? 0;
  return `size:${width}x${height}`;
}

export function buildSharedCompatibilityGroupKey(
  selection: SDSProductVariantSelection,
) {
  const normalized = buildSharedCompatibilityRawKey(selection);
  return normalized ? `compat:${sha1Hex(normalized)}` : "";
}

export function buildSharedCompatibilityRawKey(
  selection: SDSProductVariantSelection,
) {
  const parentProductId = selection.parentProductId ?? 0;
  const prototypeGroupId = selection.prototypeGroupId ?? 0;
  const layerId = selection.layerId?.trim() ?? "";
  const designType = selection.designType?.trim() ?? "";
  const printableWidth = selection.printableWidth ?? 0;
  const printableHeight = selection.printableHeight ?? 0;
  const templateImageUrl = selection.templateImageUrl?.trim() ?? "";
  const maskImageUrl = selection.maskImageUrl?.trim() ?? "";
  if (
    parentProductId <= 0 ||
    prototypeGroupId <= 0 ||
    !layerId ||
    !designType ||
    printableWidth <= 0 ||
    printableHeight <= 0 ||
    !templateImageUrl ||
    !maskImageUrl
  ) {
    return "";
  }
  return [
    parentProductId.toString(),
    prototypeGroupId.toString(),
    layerId,
    designType,
    printableWidth.toString(),
    printableHeight.toString(),
    templateImageUrl,
    maskImageUrl,
  ].join("|");
}

export function buildSharedBySizeGroupLabel(selection: SDSProductVariantSelection) {
  const width = selection.printableWidth ?? 0;
  const height = selection.printableHeight ?? 0;
  return width > 0 && height > 0
    ? `${width} x ${height}`
    : "自动尺寸";
}

export function buildPerProductLabel(selection: SDSProductVariantSelection) {
  const productName = selection.productName?.trim() || "SDS 商品";
  const variantLabel = selection.variantLabel?.trim();
  return variantLabel ? `${productName} · ${variantLabel}` : productName;
}

export function resolveDesignTargetKey(
  design: SheinStudioGeneratedDesign,
  selection: SDSProductVariantSelection,
  groupedImageMode: SheinStudioGroupedImageMode,
) {
  if (design.targetGroupKey?.trim()) {
    return design.targetGroupKey.trim();
  }
  return groupedImageMode === "per_product"
    ? buildGroupedSDSSelectionID(selection)
    : buildSharedCompatibilityGroupKey(selection) ||
        buildGroupedSDSSelectionID(selection);
}

function sha1Hex(input: string) {
  const bytes = new TextEncoder().encode(input);
  const words: number[] = [];
  for (let index = 0; index < bytes.length; index += 1) {
    words[index >> 2] |= bytes[index] << (24 - (index % 4) * 8);
  }
  words[bytes.length >> 2] |= 0x80 << (24 - (bytes.length % 4) * 8);
  words[(((bytes.length + 8) >> 6) << 4) + 15] = bytes.length * 8;

  let h0 = 0x67452301;
  let h1 = 0xefcdab89;
  let h2 = 0x98badcfe;
  let h3 = 0x10325476;
  let h4 = 0xc3d2e1f0;

  for (let block = 0; block < words.length; block += 16) {
    const w = new Array<number>(80);
    for (let i = 0; i < 16; i += 1) {
      w[i] = words[block + i] | 0;
    }
    for (let i = 16; i < 80; i += 1) {
      w[i] = rotateLeft(w[i - 3] ^ w[i - 8] ^ w[i - 14] ^ w[i - 16], 1);
    }

    let a = h0;
    let b = h1;
    let c = h2;
    let d = h3;
    let e = h4;

    for (let i = 0; i < 80; i += 1) {
      let f: number;
      let k: number;
      if (i < 20) {
        f = (b & c) | (~b & d);
        k = 0x5a827999;
      } else if (i < 40) {
        f = b ^ c ^ d;
        k = 0x6ed9eba1;
      } else if (i < 60) {
        f = (b & c) | (b & d) | (c & d);
        k = 0x8f1bbcdc;
      } else {
        f = b ^ c ^ d;
        k = 0xca62c1d6;
      }
      const temp = (rotateLeft(a, 5) + f + e + k + w[i]) | 0;
      e = d;
      d = c;
      c = rotateLeft(b, 30);
      b = a;
      a = temp;
    }

    h0 = (h0 + a) | 0;
    h1 = (h1 + b) | 0;
    h2 = (h2 + c) | 0;
    h3 = (h3 + d) | 0;
    h4 = (h4 + e) | 0;
  }

  return [h0, h1, h2, h3, h4]
    .map((value) => (value >>> 0).toString(16).padStart(8, "0"))
    .join("");
}

function rotateLeft(value: number, bits: number) {
  return (value << bits) | (value >>> (32 - bits));
}

