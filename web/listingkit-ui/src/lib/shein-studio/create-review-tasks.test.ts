import { describe, expect, it } from "vitest";

import {
  DEFAULT_SHEIN_STORE_ID,
  normalizeListingKitUploadFetchUrl,
  orderGeneratedProductImageUrls,
  sanitizeReviewTaskProductImageUrls,
} from "@/lib/shein-studio/create-review-tasks";
import { DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY } from "@/lib/shein-studio/storage-shared";

describe("SHEIN studio defaults", () => {
  it("defaults SDS source tasks to official SDS rendering for submit images", () => {
    expect(DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY).toBe("sds_official");
    expect(DEFAULT_SHEIN_STORE_ID).toBe("");
  });
});

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

describe("normalizeListingKitUploadFetchUrl", () => {
  it("rewrites listingkit upload api urls to the frontend proxy path", () => {
    expect(
      normalizeListingKitUploadFetchUrl(
        "http://localhost:8085/api/v1/listing-kits/uploads/files/20260505/demo.png?version=1",
      ),
    ).toBe("/api/listing-kits/uploads/files/20260505/demo.png?version=1");
  });

  it("leaves non-listingkit upload urls unchanged", () => {
    expect(
      normalizeListingKitUploadFetchUrl(
        "https://oss.shuomiai.com/listingkit-assets/20260505/demo.png",
      ),
    ).toBe("https://oss.shuomiai.com/listingkit-assets/20260505/demo.png");
  });
});
