import { describe, expect, it } from "vitest";

import { cacheUpdatedLabel } from "./shein-submit-readiness-helpers";

describe("cacheUpdatedLabel", () => {
  it("formats offset timestamps into compact readable text", () => {
    expect(cacheUpdatedLabel("2026-06-11T17:05:54.515876248+08:00")).toBe(
      "2026/06/11 17:05:54",
    );
  });

  it("formats zulu timestamps into local readable text", () => {
    expect(cacheUpdatedLabel("2026-06-11T09:05:54Z")).toBe(
      "2026/06/11 17:05:54",
    );
  });

  it("keeps empty values explicit", () => {
    expect(cacheUpdatedLabel()).toBe("暂无时间");
  });
});
