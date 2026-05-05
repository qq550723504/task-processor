import { describe, expect, it } from "vitest";

import {
  resolveListingKitProxyTimeoutMs,
  shouldProxyListingKitResponseAsBinary,
} from "@/app/api/listing-kits/[...path]/route";

describe("resolveListingKitProxyTimeoutMs", () => {
  it("keeps the default timeout for regular listingkit requests", () => {
    expect(resolveListingKitProxyTimeoutMs("GET", ["tasks"])).toBe(15_000);
    expect(resolveListingKitProxyTimeoutMs("POST", ["generate"])).toBe(15_000);
    expect(resolveListingKitProxyTimeoutMs("POST", ["tasks", "123", "preview"])).toBe(
      15_000,
    );
  });

  it("extends the timeout for task submit requests", () => {
    expect(resolveListingKitProxyTimeoutMs("POST", ["tasks", "123", "submit"])).toBe(
      180_000,
    );
  });
});

describe("shouldProxyListingKitResponseAsBinary", () => {
  it("treats uploaded file routes as binary even when content type is generic", () => {
    expect(
      shouldProxyListingKitResponseAsBinary("application/octet-stream", [
        "uploads",
        "files",
        "20260505",
        "demo.png",
      ]),
    ).toBe(true);
  });

  it("keeps json task endpoints on text mode", () => {
    expect(
      shouldProxyListingKitResponseAsBinary("application/json; charset=utf-8", [
        "tasks",
        "123",
      ]),
    ).toBe(false);
  });
});
