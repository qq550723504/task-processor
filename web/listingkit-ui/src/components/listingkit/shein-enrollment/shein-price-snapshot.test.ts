import { describe, expect, it } from "vitest";

import { formatSheinPriceSnapshot } from "@/components/listingkit/shein-enrollment/shein-price-snapshot";

describe("formatSheinPriceSnapshot", () => {
  it("formats common currency snapshots with symbols", () => {
    expect(formatSheinPriceSnapshot("USD 29.99")).toBe("$29.99");
    expect(formatSheinPriceSnapshot("EUR 29.99")).toBe("€29.99");
  });

  it("formats JSON snapshots from SHEIN sync records", () => {
    expect(
      formatSheinPriceSnapshot('{"sale_price":34.17,"currency":"USD","sub_site":"US"}'),
    ).toBe("$34.17");
  });

  it("falls back to original text for unsupported formats", () => {
    expect(formatSheinPriceSnapshot("USD 29.99 - USD 39.99")).toBe(
      "USD 29.99 - USD 39.99",
    );
    expect(formatSheinPriceSnapshot("")).toBe("-");
  });
});
