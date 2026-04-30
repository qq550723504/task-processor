import { describe, expect, it } from "vitest";

import { resolveListingKitProxyTimeoutMs } from "@/app/api/listing-kits/[...path]/route";

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
