import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const spec = resolve(repoRoot, "docs/api/listingkit-asset.openapi.yaml");
const output = resolve(repoRoot, "web/listingkit-ui/src/lib/api/generated");

mkdirSync(dirname(output), { recursive: true });
const uiRoot = resolve(repoRoot, "web/listingkit-ui");
const generator = resolve(uiRoot, "node_modules/@hey-api/openapi-ts/bin/run.js");

execFileSync(process.execPath, [generator, "-i", spec, "-o", output, "-p", "@hey-api/typescript"], {
  cwd: uiRoot,
  stdio: "inherit",
});
