import { describe, expect, it } from "vitest";

import {
  hasGeneratedDesignSrc,
  resolveGeneratedDesignFinalSrc,
  resolveGeneratedDesignOriginalSrc,
  resolveGeneratedDesignSrc,
} from "@/lib/shein-studio/design-image";

describe("resolveGeneratedDesignSrc", () => {
  it("rewrites listingkit upload urls to the frontend proxy path", () => {
    expect(
      resolveGeneratedDesignSrc({
        id: "design-1",
        imageUrl:
          "http://localhost:8085/api/v1/listing-kits/uploads/files/20260528/demo.png?version=1",
      }),
    ).toBe("/api/listing-kits/uploads/files/20260528/demo.png?version=1");
  });

  it("keeps remote non-upload urls unchanged", () => {
    expect(
      resolveGeneratedDesignSrc({
        id: "design-1",
        imageUrl: "https://oss.shuomiai.com/listingkit-assets/20260528/demo.png",
      }),
    ).toBe("https://oss.shuomiai.com/listingkit-assets/20260528/demo.png");
  });

  it("falls back to data url when image url is missing", () => {
    expect(
      resolveGeneratedDesignSrc({
        id: "design-1",
        dataUrl: "data:image/png;base64,abc",
      }),
    ).toBe("data:image/png;base64,abc");
  });
});

describe("resolveGeneratedDesignOriginalSrc", () => {
  it("uses the original image url when available", () => {
    expect(
      resolveGeneratedDesignOriginalSrc({
        id: "design-1",
        imageUrl: "/api/v1/listing-kits/uploads/files/final.png",
        originalImageUrl: "/api/v1/listing-kits/uploads/files/original.png",
      }),
    ).toBe("/api/listing-kits/uploads/files/original.png");
  });

  it("falls back to the current image url when original image url is missing", () => {
    expect(
      resolveGeneratedDesignOriginalSrc({
        id: "design-2",
        imageUrl: "https://cdn.example.test/ordinary.png",
      }),
    ).toBe("https://cdn.example.test/ordinary.png");
  });
});

describe("resolveGeneratedDesignFinalSrc", () => {
  it("returns an empty string until background removal succeeds", () => {
    expect(
      resolveGeneratedDesignFinalSrc({
        id: "design-3",
        imageUrl: "/api/v1/listing-kits/uploads/files/final.png",
        backgroundRemovalStatus: "pending",
      }),
    ).toBe("");
  });

  it("does not fall back to data url when background removal succeeds without image url", () => {
    expect(
      resolveGeneratedDesignFinalSrc({
        id: "design-4",
        dataUrl: "data:image/png;base64,abc",
        backgroundRemovalStatus: "succeeded",
      }),
    ).toBe("");
  });

  it("returns the normalized image url after background removal succeeds", () => {
    expect(
      resolveGeneratedDesignFinalSrc({
        id: "design-5",
        imageUrl: "/api/v1/listing-kits/uploads/files/final.png",
        backgroundRemovalStatus: "succeeded",
      }),
    ).toBe("/api/listing-kits/uploads/files/final.png");
  });
});

describe("hasGeneratedDesignSrc", () => {
  it("returns true when the normalized src is available", () => {
    expect(
      hasGeneratedDesignSrc({
        id: "design-1",
        imageUrl:
          "http://localhost:8085/api/v1/listing-kits/uploads/files/20260528/demo.png",
      }),
    ).toBe(true);
  });
});
