import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const spec = resolve(repoRoot, "docs/api/listingkit-asset.openapi.yaml");
const output = resolve(repoRoot, "web/listingkit-ui/src/lib/api/generated/listingkit-asset.ts");

mkdirSync(dirname(output), { recursive: true });
const uiRoot = resolve(repoRoot, "web/listingkit-ui");
const generator = resolve(uiRoot, "node_modules/openapi-typescript/bin/cli.js");

execFileSync(process.execPath, [generator, spec, "-o", output], {
  cwd: uiRoot,
  stdio: "inherit",
});
