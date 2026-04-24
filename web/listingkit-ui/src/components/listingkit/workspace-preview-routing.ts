import type { PreviewSlot, ReviewSlot } from "@/lib/types/listingkit";

import type { WorkspacePreviewSuggestion } from "@/components/listingkit/workspace-preview-suggestion";

function slotDisplayLabel(slot: ReviewSlot) {
  return slot.template_label ?? slot.purpose ?? slot.slot ?? "Preview";
}

export function deriveWorkspacePreviewSuggestion({
  slots,
  selectedSlot,
  focusedPreview,
}: {
  slots?: ReviewSlot[];
  selectedSlot?: string;
  focusedPreview?: PreviewSlot | null;
}): WorkspacePreviewSuggestion | null {
  const items = slots ?? [];
  if (items.length === 0) {
    return null;
  }

  const currentHasImage = Boolean(focusedPreview?.asset_url);
  const currentIsGallery = selectedSlot === "gallery";
  if (currentHasImage || currentIsGallery) {
    return null;
  }

  const preferred =
    items.find((slot) => slot.slot === "gallery" && slot.render_preview_available) ??
    items.find((slot) => slot.render_preview_available && slot.slot !== selectedSlot);
  if (!preferred) {
    return null;
  }

  return {
    slot: preferred,
    title: `Open ${slotDisplayLabel(preferred)}`,
    summary: "The current review focus is not the best place to inspect the generated image.",
    ctaLabel: "View Image",
  };
}
