import { describe, expect, it } from "vitest";

import { buildSheinStoreOptions } from "@/lib/query/use-shein-store-selector";

describe("buildSheinStoreOptions", () => {
  it("includes authorized stores without profiles and enriches profiled stores", () => {
    const options = buildSheinStoreOptions(
      [
        {
          id: 870,
          store_id: "SHEIN-US-870",
          name: "US 主店",
          platform: "SHEIN",
          region: "US",
        },
        {
          id: 871,
          store_id: "SHEIN-GB-871",
          name: "GB 店铺",
          platform: "SHEIN",
          region: "GB",
        },
        {
          id: 872,
          store_id: "SHEIN-DE-872",
          name: "已禁用店铺",
          platform: "SHEIN",
          region: "DE",
          status: 1,
        },
      ],
      [
        {
          id: 11,
          store_id: 870,
          enabled: true,
          site: "US",
        },
      ],
    );

    expect(options).toHaveLength(2);
    expect(options[0]).toMatchObject({
      id: 11,
      store_id: 870,
      site: "US",
      store: {
        name: "US 主店",
        store_id: "SHEIN-US-870",
        region: "US",
      },
    });
    expect(options[1]).toMatchObject({
      store_id: 871,
      enabled: true,
      store: {
        name: "GB 店铺",
        store_id: "SHEIN-GB-871",
        region: "GB",
      },
    });
  });
});
