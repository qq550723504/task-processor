import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";

describe("sanitizedNavigationSearchParams", () => {
  it("removes Next.js internal RSC params from navigation URLs", () => {
    const params = sanitizedNavigationSearchParams(
      new URLSearchParams(
        "productId=124110&variantId=124111&_rsc=6ggpw&step=generate",
      ),
    );

    expect(params.toString()).toBe(
      "productId=124110&variantId=124111&step=generate",
    );
  });

  it("removes Next.js internal double underscore params", () => {
    const params = sanitizedNavigationSearchParams(
      new URLSearchParams("platform=shein&__nextLocale=zh-CN&section_key=images"),
    );

    expect(params.toString()).toBe("platform=shein&section_key=images");
  });
});
