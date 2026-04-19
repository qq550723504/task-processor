import { buildQueryString } from "@/lib/api/query-string";

describe("buildQueryString", () => {
  it("serializes listingkit filters and omits empty values", () => {
    const result = buildQueryString({
      platform: "shein",
      slot: "",
      render_preview_available: true,
      preview_capability: "detail_preview",
      page: 2,
      page_size: 25,
      delta_token: undefined,
    });

    expect(result).toBe(
      "page=2&page_size=25&platform=shein&preview_capability=detail_preview&render_preview_available=true",
    );
  });

  it("returns an empty string when no query params are present", () => {
    expect(buildQueryString({})).toBe("");
  });
});
