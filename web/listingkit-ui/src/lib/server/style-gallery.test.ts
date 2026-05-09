import { describe, expect, it } from "vitest";

import { isAIGeneratedGallerySource } from "./style-gallery";

describe("isAIGeneratedGallerySource", () => {
  it("keeps only AI generated style image sources", () => {
    expect(isAIGeneratedGallerySource("studio_saved")).toBe(true);
    expect(isAIGeneratedGallerySource("studio_legacy")).toBe(true);
    expect(isAIGeneratedGallerySource("published_input")).toBe(true);
    expect(isAIGeneratedGallerySource("task_source")).toBe(false);
    expect(isAIGeneratedGallerySource("task_mockup")).toBe(false);
    expect(isAIGeneratedGallerySource("task_shein")).toBe(false);
  });
});
