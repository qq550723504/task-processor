import { describe, expect, it } from "vitest";

import {
  normalizeStyleGalleryImageUrl,
} from "./style-gallery";

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
