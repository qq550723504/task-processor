import { describe, expect, it } from "vitest";

import { resolveWorkspaceScenePreset } from "@/components/listingkit/workspace-scene-preset";

describe("resolveWorkspaceScenePreset", () => {
  it("prefers review preview scene preset", () => {
    const result = resolveWorkspaceScenePreset({
      reviewPreviewPreset: { prompt_key: "productimage.scene.shoes" },
      focusedScenePreset: { prompt_key: "productimage.scene.default" },
      queueItems: [
        {
          slot: "gallery",
          selected_asset_id: "gallery-1",
          scene_preset: { prompt_key: "productimage.scene.bags" },
        },
      ],
      selectedSlot: "gallery",
    });

    expect(result?.prompt_key).toBe("productimage.scene.shoes");
  });

  it("falls back to focused scene preset when preview is empty", () => {
    const result = resolveWorkspaceScenePreset({
      reviewPreviewPreset: {},
      focusedScenePreset: { prompt_key: "productimage.scene.default" },
      queueItems: [
        {
          slot: "gallery",
          scene_preset: { prompt_key: "productimage.scene.bags" },
        },
      ],
      selectedSlot: "gallery",
    });

    expect(result?.prompt_key).toBe("productimage.scene.default");
  });

  it("falls back to queue item scene preset for selected slot", () => {
    const result = resolveWorkspaceScenePreset({
      queueItems: [
        {
          slot: "main",
          scene_preset: { prompt_key: "productimage.scene.default" },
        },
        {
          slot: "gallery",
          scene_preset: { prompt_key: "productimage.scene.shoes" },
        },
      ],
      selectedSlot: "gallery",
    });

    expect(result?.prompt_key).toBe("productimage.scene.shoes");
  });

  it("falls back to preview scene presets for selected platform and slot", () => {
    const result = resolveWorkspaceScenePreset({
      previewScenePresets: {
        shein: [
          {
            slot: "gallery",
            asset_id: "gallery-1",
            scene_preset: { prompt_key: "productimage.scene.jewelry" },
          },
        ],
      },
      selectedPlatform: "shein",
      selectedSlot: "gallery",
      focusedAssetId: "gallery-1",
    });

    expect(result?.prompt_key).toBe("productimage.scene.jewelry");
  });

  it("prefers focused asset match among queue items", () => {
    const result = resolveWorkspaceScenePreset({
      queueItems: [
        {
          slot: "gallery",
          selected_asset_id: "gallery-2",
          scene_preset: { prompt_key: "productimage.scene.default" },
        },
        {
          slot: "gallery",
          selected_asset_id: "gallery-1",
          scene_preset: { prompt_key: "productimage.scene.shoes" },
        },
      ],
      selectedSlot: "gallery",
      focusedAssetId: "gallery-1",
    });

    expect(result?.prompt_key).toBe("productimage.scene.shoes");
  });

  it("returns undefined when no scene preset exists", () => {
    const result = resolveWorkspaceScenePreset({
      queueItems: [{ slot: "gallery" }],
      selectedSlot: "gallery",
    });

    expect(result).toBeUndefined();
  });
});
