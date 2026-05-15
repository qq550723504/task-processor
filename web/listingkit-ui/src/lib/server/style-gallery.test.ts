import path from "node:path";

import { afterEach, describe, expect, it } from "vitest";

import { getGalleryImageRoots, isAIGeneratedGallerySource } from "./style-gallery";

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

describe("getGalleryImageRoots", () => {
  afterEach(() => {
    delete process.env.LISTINGKIT_UI_STORAGE_DIR;
  });

  it("uses LISTINGKIT_UI_STORAGE_DIR for legacy studio assets", () => {
    process.env.LISTINGKIT_UI_STORAGE_DIR = path.join("tmp", "listingkit-ui");

    expect(getGalleryImageRoots().legacy).toBe(
      path.join("tmp", "listingkit-ui", "shein-studio-assets"),
    );
  });
});
