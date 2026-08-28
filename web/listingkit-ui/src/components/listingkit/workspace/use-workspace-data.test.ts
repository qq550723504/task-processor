import { describe, expect, it } from "vitest";

import { resolveWorkspaceTitle } from "@/components/listingkit/workspace/use-workspace-data";

describe("resolveWorkspaceTitle", () => {
  it("does not leak a SHEIN-adapted title into another platform workspace", () => {
    expect(
      resolveWorkspaceTitle({
        selectedPlatform: "temu",
        sheinFinalTitle: "SHEIN Adapted Tote",
        sheinSourceTitle: "SHEIN Source Tote",
        canonicalTitle: "Canonical Tote",
      }),
    ).toBe("Canonical Tote");
  });
});
