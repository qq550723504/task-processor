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
    const key = buildSharedBySizeGroupKey(selection);
    const existing = buckets.get(key);
    if (existing) {
      existing.selectionIds.push(selectionId);
      existing.selections.push(selection);
      continue;
    }
    buckets.set(key, {
      key,
      label: buildSharedBySizeGroupLabel(selection),
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
    : buildSharedBySizeGroupKey(selection);
}

