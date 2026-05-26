"use client";

import { useMemo, useState } from "react";

import {
  buildSDSVariantOptions,
  filterSDSVariants,
  summarizeSelectedSDSVariants,
} from "@/components/listingkit/sds/sds-variant-picker-model";
import {
  SDSVariantFilters,
  SDSVariantGrid,
  SDSVariantPickerHeader,
  SDSVariantPickerStatus,
  SDSVariantSelectionSummary,
} from "@/components/listingkit/sds/sds-variant-picker-sections";
import type { SDSProductSummary, SDSProductVariant } from "@/lib/types/sds";

export function SDSVariantPicker({
  onAddSelectedVariantsToGroupedCandidates,
  open,
  product,
  variants,
  selectedVariantId,
  isLoading,
  hasError,
  onClose,
  onSelectVariants,
}: {
  onAddSelectedVariantsToGroupedCandidates?: (
    primary: SDSProductVariant,
    variants: SDSProductVariant[],
  ) => void;
  open: boolean;
  product?: SDSProductSummary;
  variants: SDSProductVariant[];
  selectedVariantId?: number;
  isLoading: boolean;
  hasError: boolean;
  onClose: () => void;
  onSelectVariants: (
    primary: SDSProductVariant,
    variants: SDSProductVariant[],
  ) => void;
}) {
  const [sizeFilter, setSizeFilter] = useState("");
  const [colorFilter, setColorFilter] = useState("");
  const [selectionState, setSelectionState] = useState<{
    key: string;
    ids: number[];
  }>({ key: "", ids: [] });

  const sizeOptions = useMemo(
    () => buildSDSVariantOptions(variants, "size"),
    [variants],
  );
  const colorOptions = useMemo(
    () => buildSDSVariantOptions(variants, "color_name"),
    [variants],
  );
  const filteredVariants = useMemo(
    () => filterSDSVariants({ colorFilter, sizeFilter, variants }),
    [colorFilter, sizeFilter, variants],
  );
  const variantKey = useMemo(
    () => variants.map((variant) => variant.id).join(":"),
    [variants],
  );
  const selectedIds =
    selectionState.key === variantKey
      ? selectionState.ids
      : variants.map((variant) => variant.id);
  const selectedVariants = useMemo(
    () => variants.filter((variant) => selectedIds.includes(variant.id)),
    [selectedIds, variants],
  );
  const { selectedColors, selectedSizes } = useMemo(
    () => summarizeSelectedSDSVariants(selectedVariants),
    [selectedVariants],
  );

  function toggleVariant(variantId: number) {
    setSelectionState({
      key: variantKey,
      ids: selectedIds.includes(variantId)
        ? selectedIds.filter((id) => id !== variantId)
        : [...selectedIds, variantId],
    });
  }

  function selectFilteredVariants() {
    const filteredIds = filteredVariants.map((variant) => variant.id);
    setSelectionState({
      key: variantKey,
      ids: Array.from(new Set([...selectedIds, ...filteredIds])),
    });
  }

  function clearFilteredVariants() {
    const filteredIds = new Set(filteredVariants.map((variant) => variant.id));
    setSelectionState({
      key: variantKey,
      ids: selectedIds.filter((id) => !filteredIds.has(id)),
    });
  }

  function useSelectedVariants() {
    const { primary, selected } = resolveSelectedVariants({
      selectedVariantId,
      selectedVariants,
      variants,
    });
    if (!primary) {
      return;
    }
    onSelectVariants(primary, selected);
  }

  function addSelectedVariantsToGroupedCandidates() {
    const { primary, selected } = resolveSelectedVariants({
      selectedVariantId,
      selectedVariants,
      variants,
    });
    if (!primary) {
      return;
    }
    onAddSelectedVariantsToGroupedCandidates?.(primary, selected);
  }

  function selectAsPrimary(variant: SDSProductVariant) {
    const selected =
      selectedVariants.length > 0 ? selectedVariants : [variant];
    onSelectVariants(
      variant,
      selected.some((item) => item.id === variant.id)
        ? selected
        : [variant, ...selected],
    );
  }

  if (!open) {
    return null;
  }

  const blockingStatus =
    isLoading || hasError || variants.length === 0 ? (
      <SDSVariantPickerStatus
        hasError={hasError}
        isLoading={isLoading}
        variantCount={variants.length}
      />
    ) : null;

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-zinc-950/40 p-4 backdrop-blur-sm md:items-center">
      <div className="flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-[2rem] border border-white/60 bg-[linear-gradient(180deg,_rgba(255,255,255,0.97),_rgba(244,244,245,0.95))] shadow-[0_24px_96px_rgba(24,24,27,0.24)]">
        <SDSVariantPickerHeader onClose={onClose} product={product} />

        <div className="overflow-auto px-5 py-5 md:px-6">
          {blockingStatus ?? (
            <div className="space-y-4">
              <SDSVariantFilters
                colorFilter={colorFilter}
                colorOptions={colorOptions}
                filteredCount={filteredVariants.length}
                setColorFilter={setColorFilter}
                setSizeFilter={setSizeFilter}
                sizeFilter={sizeFilter}
                sizeOptions={sizeOptions}
              />
              <SDSVariantSelectionSummary
                addSelectedVariantsToGroupedCandidates={
                  addSelectedVariantsToGroupedCandidates
                }
                clearFilteredVariants={clearFilteredVariants}
                selectFilteredVariants={selectFilteredVariants}
                selectedColorCount={selectedColors.length}
                selectedSizeCount={selectedSizes.length}
                selectedVariantCount={selectedVariants.length}
                useSelectedVariants={useSelectedVariants}
              />
              <SDSVariantGrid
                filteredVariants={filteredVariants}
                onSelectAsPrimary={selectAsPrimary}
                selectedIds={selectedIds}
                selectedVariantId={selectedVariantId}
                toggleVariant={toggleVariant}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function resolveSelectedVariants({
  selectedVariantId,
  selectedVariants,
  variants,
}: {
  selectedVariantId?: number;
  selectedVariants: SDSProductVariant[];
  variants: SDSProductVariant[];
}) {
  const selected =
    selectedVariants.length > 0 ? selectedVariants : variants.slice(0, 1);
  const primary =
    selected.find((variant) => variant.id === selectedVariantId) ?? selected[0];
  return { primary, selected };
}
