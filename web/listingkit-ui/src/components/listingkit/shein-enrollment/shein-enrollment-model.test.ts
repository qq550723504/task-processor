import { describe, expect, it } from "vitest";

import {
  parseSheinActivityType,
  parseSheinEnrollmentTab,
  SHEIN_ACTIVITY_TYPE_OPTIONS,
} from "@/components/listingkit/shein-enrollment/shein-enrollment-model";

describe("shein enrollment model", () => {
  it("defaults unknown tabs to candidates", () => {
    expect(parseSheinEnrollmentTab(undefined)).toBe("candidates");
    expect(parseSheinEnrollmentTab("bogus")).toBe("candidates");
    expect(parseSheinEnrollmentTab("products")).toBe("candidates");
  });

  it("defaults unknown activity types to PROMOTION", () => {
    expect(parseSheinActivityType(undefined)).toBe("PROMOTION");
    expect(parseSheinActivityType("unknown")).toBe("PROMOTION");
    expect(parseSheinActivityType("MIXED")).toBe("PROMOTION");
    expect(SHEIN_ACTIVITY_TYPE_OPTIONS.map((option) => option.value)).toEqual([
      "PROMOTION",
      "TIME_LIMITED",
    ]);
  });
});
