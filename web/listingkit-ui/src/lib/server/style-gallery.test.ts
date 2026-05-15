import path from "node:path";

import { afterEach, describe, expect, it } from "vitest";

import {
  getGalleryImageRoots,
  isAIGeneratedGallerySource,
  normalizeStyleGalleryImageUrl,
} from "./style-gallery";

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

describe("normalizeStyleGalleryImageUrl", () => {
  it("routes ListingKit uploaded image URLs through the UI proxy", () => {
    expect(
      normalizeStyleGalleryImageUrl(
        "http://localhost:8085/api/v1/listing-kits/uploads/files/20260509/demo.png?version=1",
      ),
    ).toBe("/api/listing-kits/uploads/files/20260509/demo.png?version=1");
  });

  it("keeps unrelated absolute URLs unchanged", () => {
    expect(normalizeStyleGalleryImageUrl("https://cdn.example.com/demo.png")).toBe(
      "https://cdn.example.com/demo.png",
    );
  });
});
