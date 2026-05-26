import { describe, expect, it } from "vitest";

import { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";

describe("formatSheinStoreOptionLabel", () => {
  it("includes the store name and distinguishing metadata", () => {
    expect(
      formatSheinStoreOptionLabel({
        store_id: 869,
        store: {
          id: 869,
          name: "SHEIN US 1",
          store_id: "shein-us-1",
          region: "NA",
        },
        site: "US",
      }),
    ).toBe("SHEIN US 1 (shein-us-1 / NA / US)");
  });

  it("falls back to external store id when a display name is unavailable", () => {
    expect(
      formatSheinStoreOptionLabel({
        store_id: 870,
        store: {
          id: 870,
          store_id: "shein-us-2",
        },
        site: "US",
      }),
    ).toBe("shein-us-2 (US)");
  });
});
