import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const workspaceDir = dirname(fileURLToPath(import.meta.url));

function source(relativePath: string) {
  return readFileSync(join(workspaceDir, relativePath), "utf8");
}

describe("retired generation route consumers", () => {
  it("keeps the active workspace path on supported read APIs and local routing", () => {
    const activeSources = [
      source("use-workspace-data.ts"),
      source("use-workspace-navigation-actions.ts"),
      source("use-shein-workspace-actions.ts"),
    ].join("\n");

    for (const retiredRoute of [
      "generation-review-session",
      "generation-review-preview",
      "generation-navigation/dispatch",
      "generation-queue",
      "shein-image-regeneration",
    ]) {
      expect(activeSources).not.toContain(retiredRoute);
    }
  });

  it("redirects the retired queue entrypoint instead of rendering a dead API consumer", () => {
    const queuePage = readFileSync(
      join(workspaceDir, "../../../app/listing-kits/[taskId]/queue/page.tsx"),
      "utf8",
    );

    expect(queuePage).not.toContain("QueueScreen");
    expect(queuePage).toContain("redirect");
  });
});
