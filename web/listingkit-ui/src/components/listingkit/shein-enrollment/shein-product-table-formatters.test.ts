import { describe, expect, it } from "vitest";

import {
  formatInventorySummary,
  formatProductTimes,
  getCostSourceLabel,
} from "@/components/listingkit/shein-enrollment/shein-product-table-formatters";

describe("shein product table formatters", () => {
  it("parses JSON inventory summaries into structured values", () => {
    expect(
      formatInventorySummary('{"total_inventory":999,"saleable_inventory":321}'),
    ).toEqual({
      total: "999",
      available: "321",
      raw: null,
    });
  });

  it("parses synced inventory snapshots into structured values", () => {
    expect(formatInventorySummary('{"total":999,"available":321}')).toEqual({
      total: "999",
      available: "321",
      raw: null,
    });
  });

  it("falls back to raw inventory text when the value is not JSON", () => {
    expect(formatInventorySummary("warehouse A: 50")).toEqual({
      total: null,
      available: null,
      raw: "warehouse A: 50",
    });
  });

  it("maps cost source labels", () => {
    expect(getCostSourceLabel("manual")).toBe("人工");
    expect(getCostSourceLabel("auto")).toBe("自动");
    expect(getCostSourceLabel("none")).toBe("缺失");
  });

  it("returns product times in stable labeled order", () => {
    expect(
      formatProductTimes({
        created_at: "2024-01-02 03:04:05",
        publish_time: "2024-02-03 04:05:06",
        first_shelf_time: "2024-03-04 05:06:07",
        last_sync_at: "2024-04-05 06:07:08",
      }),
    ).toEqual([
      { label: "创建", value: "2024-01-02 03:04:05" },
      { label: "发布", value: "2024-02-03 04:05:06" },
      { label: "首次上架", value: "2024-03-04 05:06:07" },
      { label: "最近同步", value: "2024-04-05 06:07:08" },
    ]);
  });
});
