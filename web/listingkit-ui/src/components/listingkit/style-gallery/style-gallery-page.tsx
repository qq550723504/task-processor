"use client";

import { useMemo, useState } from "react";

import {
  StyleGalleryDimensionFilters,
  StyleGalleryEmptyState,
  StyleGalleryGrid,
  StyleGalleryHero,
  StyleGalleryMetrics,
  StyleGalleryNoResults,
} from "@/components/listingkit/style-gallery/style-gallery-page-sections";
import {
  matchesDimensionFilter,
  type DimensionPreset,
  type ImageDimensions,
} from "@/components/listingkit/style-gallery/style-gallery-page-model";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import type { StyleGalleryResponse } from "@/lib/types/style-gallery";

export { formatImageDimensions } from "@/components/listingkit/style-gallery/style-gallery-page-model";

type GalleryPageProps = {
  initialGallery: StyleGalleryResponse;
};

export function StyleGalleryPage({ initialGallery }: GalleryPageProps) {
  const items = initialGallery.items;
  const [dimensionPreset, setDimensionPreset] =
    useState<DimensionPreset>("all");
  const [dimensionsById, setDimensionsById] = useState<
    Record<string, ImageDimensions>
  >({});
  const [minHeight, setMinHeight] = useState("");
  const [minWidth, setMinWidth] = useState("");
  const minHeightValue = Number(minHeight);
  const minWidthValue = Number(minWidth);
  const hasMinHeight = Number.isFinite(minHeightValue) && minHeightValue > 0;
  const hasMinWidth = Number.isFinite(minWidthValue) && minWidthValue > 0;
  const hasActiveDimensionFilter =
    dimensionPreset !== "all" || hasMinHeight || hasMinWidth;
  const visibleItems = useMemo(
    () =>
      items.filter((item) =>
        matchesDimensionFilter(dimensionsById[item.id], {
          preset: dimensionPreset,
          minHeight: hasMinHeight ? minHeightValue : undefined,
          minWidth: hasMinWidth ? minWidthValue : undefined,
        }),
      ),
    [
      dimensionPreset,
      dimensionsById,
      hasMinHeight,
      hasMinWidth,
      items,
      minHeightValue,
      minWidthValue,
    ],
  );

  function handleDimensions(itemId: string, dimensions: ImageDimensions) {
    setDimensionsById((current) => {
      const existing = current[itemId];
      if (
        existing?.width === dimensions.width &&
        existing.height === dimensions.height
      ) {
        return current;
      }
      return { ...current, [itemId]: dimensions };
    });
  }

  return (
    <ListingKitPageShell
      backgroundClassName="isolate overflow-hidden bg-[radial-gradient(circle_at_12%_8%,rgba(14,165,233,0.15),transparent_28%),radial-gradient(circle_at_85%_0%,rgba(245,158,11,0.18),transparent_30%),linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)]"
      overlayClassName="bg-[linear-gradient(rgba(24,24,27,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.035)_1px,transparent_1px)] bg-[size:34px_34px]"
    >
      <StyleGalleryHero />
      <StyleGalleryMetrics gallery={initialGallery} />
      <StyleGalleryDimensionFilters
        dimensionPreset={dimensionPreset}
        itemCount={items.length}
        minHeight={minHeight}
        minWidth={minWidth}
        setDimensionPreset={setDimensionPreset}
        setMinHeight={setMinHeight}
        setMinWidth={setMinWidth}
        visibleCount={visibleItems.length}
      />

      {items.length === 0 ? (
        <StyleGalleryEmptyState />
      ) : visibleItems.length === 0 ? (
        <StyleGalleryNoResults
          hasActiveDimensionFilter={hasActiveDimensionFilter}
        />
      ) : (
        <StyleGalleryGrid
          items={visibleItems}
          onDimensions={handleDimensions}
        />
      )}
    </ListingKitPageShell>
  );
}
