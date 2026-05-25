import { describe, expect, it } from "vitest";

import { buildSheinStudioStepHref } from "@/lib/shein-studio/navigation";

describe("buildSheinStudioStepHref", () => {
  it("preserves user params and updates the step", () => {
    const href = buildSheinStudioStepHref(
      "/listing-kits/sds",
      new URLSearchParams(
        "keyword=beer&page=1&variantId=124111&step=generate&_rsc=123",
      ),
      "review",
    );

    expect(href).toBe(
      "/listing-kits/sds?keyword=beer&page=1&variantId=124111&step=review",
    );
  });
});
