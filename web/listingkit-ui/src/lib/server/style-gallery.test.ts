import { describe, expect, it } from "vitest";

import {
  normalizeStyleGalleryImageUrl,
  resolveStyleGalleryApiOrigin,
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

describe("resolveStyleGalleryApiOrigin", () => {
  it("prefers forwarded host and protocol from the current request", () => {
    const headers = new Headers({
      "x-forwarded-host": "pod.shuomiai.com",
      "x-forwarded-proto": "https",
      host: "listingkit-ui.task-processor.svc.cluster.local",
    });

    expect(resolveStyleGalleryApiOrigin(headers)).toBe("https://pod.shuomiai.com");
  });

  it("falls back to the local host when forwarded headers are absent", () => {
    const headers = new Headers({
      host: "localhost:3000",
    });

    expect(resolveStyleGalleryApiOrigin(headers)).toBe("http://localhost:3000");
  });
});
