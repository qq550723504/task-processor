import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const currentFile = fileURLToPath(import.meta.url);
const sheinStudioDir = path.dirname(currentFile);

function productionSources(root: string): string[] {
  const entries = fs.readdirSync(root, { withFileTypes: true });
  return entries.flatMap((entry) => {
    const fullPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      return productionSources(fullPath);
    }
    if (!/\.(ts|tsx)$/.test(entry.name)) {
      return [];
    }
    if (
      entry.name.endsWith(".test.ts") ||
      entry.name.endsWith(".test.tsx") ||
      entry.name.includes("test-harness")
    ) {
      return [];
    }
    return [fullPath];
  });
}

describe("SHEIN Studio production entrypoints", () => {
  it("keeps batch workbench code off the legacy standalone task creation module", () => {
    const offenders = productionSources(sheinStudioDir).filter((file) => {
      const source = fs.readFileSync(file, "utf8");
      return source.includes("@/lib/shein-studio/create-review-tasks");
    });

    expect(offenders.map((file) => path.relative(sheinStudioDir, file))).toEqual([]);
  });
});
