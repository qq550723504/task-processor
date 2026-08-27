import {
  buildPlatformWorkspaceHref,
  buildProductWorkspaceHref,
  buildWorkspaceSearch,
  shouldSyncFocusedTargetToRoute,
} from "@/components/listingkit/workspace/workspace-routing";

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

describe("buildProductWorkspaceHref", () => {
  it("clears platform review params and persists the product section", () => {
    expect(
      buildProductWorkspaceHref(
        "task-1",
        "foo=bar&platform=shein&slot=main&preview_capability=detail_preview&section_key=general_review",
        "images",
      ),
    ).toBe("/listing-kits/task-1/workspace?foo=bar&product_section=images");
  });

  it("keeps a product route marker when selecting the overview", () => {
    expect(
      buildProductWorkspaceHref(
        "task-1",
        "foo=bar&platform=shein&slot=main",
        "overview",
      ),
    ).toBe("/listing-kits/task-1/workspace?foo=bar&product_section=overview");
  });
});

describe("buildPlatformWorkspaceHref", () => {
  it("clears focus parameters when switching platforms", () => {
    expect(
      buildPlatformWorkspaceHref(
        "task-1",
        "foo=bar&platform=shein&slot=main&preview_capability=detail_preview&section_key=general_review",
        "amazon",
      ),
    ).toBe("/listing-kits/task-1/workspace?foo=bar&platform=amazon");
  });
});

describe("shouldSyncFocusedTargetToRoute", () => {
  it("does not overwrite a canonical product route with the focused platform target", () => {
    expect(shouldSyncFocusedTargetToRoute("product_section=overview")).toBe(false);
    expect(shouldSyncFocusedTargetToRoute("product_section=images")).toBe(false);
    expect(shouldSyncFocusedTargetToRoute("platform=shein")).toBe(true);
  });
});
