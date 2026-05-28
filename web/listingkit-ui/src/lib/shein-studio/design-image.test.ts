import { describe, expect, it } from "vitest";

import {
  hasGeneratedDesignSrc,
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
