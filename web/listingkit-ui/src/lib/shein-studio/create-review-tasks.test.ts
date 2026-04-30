import { describe, expect, it } from "vitest";

import { orderGeneratedProductImageUrls } from "@/lib/shein-studio/create-review-tasks";

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
