import { buildWorkspaceSearch } from "@/components/listingkit/workspace-routing";

describe("buildWorkspaceSearch", () => {
  it("derives search params from focused target", () => {
    const result = buildWorkspaceSearch(
      "platform=amazon",
      {
        platform: "shein",
        slot: "main",
        capability: "detail_preview",
        section_key: "detail_preview-main",
      },
    );

    expect(result).toBe(
      "platform=shein&slot=main&preview_capability=detail_preview&section_key=detail_preview-main",
    );
  });

  it("drops empty values and preserves unrelated params", () => {
    const result = buildWorkspaceSearch("foo=bar&slot=gallery", {
      platform: "temu",
      slot: "",
    });

    expect(result).toBe("foo=bar&platform=temu");
  });
});
