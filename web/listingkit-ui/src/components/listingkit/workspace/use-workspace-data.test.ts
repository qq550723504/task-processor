import { describe, expect, it } from "vitest";

import {
  mergeNavigationPlatformCards,
  resolveWorkspaceTitle,
} from "@/components/listingkit/workspace/use-workspace-data";
import type { PlatformCard } from "@/lib/types/listingkit";

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

  it("skips whitespace-only platform title candidates", () => {
    expect(
      resolveWorkspaceTitle({
        selectedPlatform: "amazon",
        amazonTitle: "   ",
        canonicalTitle: "Canonical Tote",
      }),
    ).toBe("Canonical Tote");
    expect(
      resolveWorkspaceTitle({
        selectedPlatform: "shein",
        sheinFinalTitle: "\t",
        sheinSourceTitle: "SHEIN Source Tote",
        canonicalTitle: "Canonical Tote",
      }),
    ).toBe("SHEIN Source Tote");
  });
});

describe("mergeNavigationPlatformCards", () => {
  it("uses the live session card for a platform while preserving preview-only platforms", () => {
    const previewCards: PlatformCard[] = [
      { platform: "shein", status: "ready", summary: "stale preview" },
      { platform: "temu", status: "ready", summary: "preview only" },
    ];
    const sessionCards: PlatformCard[] = [
      { platform: "shein", status: "failed", summary: "live recovery required" },
    ];

    const merged = mergeNavigationPlatformCards(previewCards, sessionCards);

    expect(merged).toEqual([
      { platform: "shein", status: "failed", summary: "live recovery required" },
      { platform: "temu", status: "ready", summary: "preview only" },
    ]);
    expect(merged[0]).toBe(sessionCards[0]);
  });
});
