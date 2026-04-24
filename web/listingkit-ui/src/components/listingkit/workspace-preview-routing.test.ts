import { deriveWorkspacePreviewSuggestion } from "@/components/listingkit/workspace-preview-routing";

describe("deriveWorkspacePreviewSuggestion", () => {
  it("suggests gallery when the current slot is not image-focused", () => {
    const suggestion = deriveWorkspacePreviewSuggestion({
      selectedSlot: "auxiliary",
      focusedPreview: {
        asset_id: "selling-point-1",
      },
      slots: [
        {
          slot: "auxiliary",
          purpose: "selling_point",
          asset_id: "selling-point-1",
          template_label: "SHEIN Selling Point",
          render_preview_available: true,
        },
        {
          slot: "gallery",
          purpose: "gallery",
          asset_id: "gallery-1",
          template_label: "SHEIN Lifestyle Gallery",
          render_preview_available: true,
        },
      ],
    });

    expect(suggestion?.slot.slot).toBe("gallery");
    expect(suggestion?.title).toBe("Open SHEIN Lifestyle Gallery");
  });

  it("does not suggest another slot when the current preview already has an image", () => {
    const suggestion = deriveWorkspacePreviewSuggestion({
      selectedSlot: "gallery",
      focusedPreview: {
        asset_id: "gallery-1",
        asset_url: "http://127.0.0.1:9100/listingkit-assets/gallery-1.png",
      },
      slots: [
        {
          slot: "gallery",
          purpose: "gallery",
          asset_id: "gallery-1",
          template_label: "SHEIN Lifestyle Gallery",
          render_preview_available: true,
        },
      ],
    });

    expect(suggestion).toBeNull();
  });
});
