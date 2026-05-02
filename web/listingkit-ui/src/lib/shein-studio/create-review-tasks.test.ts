import { describe, expect, it } from "vitest";

import {
  orderGeneratedProductImageUrls,
  sanitizeReviewTaskProductImageUrls,
} from "@/lib/shein-studio/create-review-tasks";

describe("orderGeneratedProductImageUrls", () => {
  it("keeps the main white-background image first when present", () => {
    expect(
      orderGeneratedProductImageUrls([
        { imageUrl: "https://example.com/detail.jpg", role: "detail" },
        { imageUrl: "https://example.com/main.jpg", role: "main" },
        { imageUrl: "https://example.com/scene.jpg", role: "scene" },
      ]),
    ).toEqual([
      "https://example.com/main.jpg",
      "https://example.com/detail.jpg",
      "https://example.com/scene.jpg",
    ]);
  });

  it("drops empty urls while preserving non-main image order", () => {
    expect(
      orderGeneratedProductImageUrls([
        { imageUrl: "https://example.com/scene.jpg", role: "scene" },
        { imageUrl: "   ", role: "main" },
        { imageUrl: "https://example.com/detail.jpg", role: "detail" },
      ]),
    ).toEqual([
      "https://example.com/scene.jpg",
      "https://example.com/detail.jpg",
    ]);
  });
});

describe("sanitizeReviewTaskProductImageUrls", () => {
  it("drops SDS preview product images before creating review tasks", () => {
    expect(
      sanitizeReviewTaskProductImageUrls(
        [
          "https://cdn.sdspod.com/images/path/to/preview.jpg",
          "https://oss.shuomiai.com/listingkit-assets/20260502/generated-main.png",
          "https://cdn.sdspod.com/out/36811/202605/mockup.jpg",
        ],
        "ai_generated",
      ),
    ).toEqual([
      "https://oss.shuomiai.com/listingkit-assets/20260502/generated-main.png",
      "https://cdn.sdspod.com/out/36811/202605/mockup.jpg",
    ]);
  });

  it("clears product image urls for the SDS official strategy", () => {
    expect(
      sanitizeReviewTaskProductImageUrls(
        [
          "https://cdn.sdspod.com/images/path/to/preview.jpg",
          "https://oss.shuomiai.com/listingkit-assets/20260502/generated-main.png",
        ],
        "sds_official",
      ),
    ).toEqual([]);
  });

  it("deduplicates and trims remaining urls", () => {
    expect(
      sanitizeReviewTaskProductImageUrls(
        [
          "  https://oss.shuomiai.com/listingkit-assets/20260502/generated-main.png  ",
          "https://oss.shuomiai.com/listingkit-assets/20260502/generated-main.png",
          "",
        ],
        "hybrid",
      ),
    ).toEqual(["https://oss.shuomiai.com/listingkit-assets/20260502/generated-main.png"]);
  });
});
