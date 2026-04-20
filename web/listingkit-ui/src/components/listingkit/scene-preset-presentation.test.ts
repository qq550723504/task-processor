import { describe, expect, it } from "vitest";

import {
  presentSceneDefaultsSource,
  presentScenePresetCompact,
  presentSceneValue,
} from "@/components/listingkit/scene-preset-presentation";

describe("scene preset presentation", () => {
  it("formats defaults source labels", () => {
    expect(presentSceneDefaultsSource("platform_category")).toBe(
      "Platform + category default",
    );
    expect(presentSceneDefaultsSource("explicit")).toBe("User override");
  });

  it("formats scene values", () => {
    expect(presentSceneValue("background_tone")).toBe("Background Tone");
    expect(presentSceneValue("studio")).toBe("Studio");
  });

  it("builds compact scene preset summary", () => {
    expect(
      presentScenePresetCompact({
        scene_category: "shoes",
        defaults_source: "platform_category",
        scene_style: "studio",
      }),
    ).toEqual({
      title: "Shoes",
      detail: "Platform + category default · Studio",
    });
  });
});

