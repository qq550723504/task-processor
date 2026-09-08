import { rm } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

export async function cleanNextBuildOutput(root = process.cwd()) {
  await rm(path.resolve(root, ".next"), { recursive: true, force: true });
}

const entrypoint = process.argv[1]
  ? pathToFileURL(path.resolve(process.argv[1])).href
  : "";

if (entrypoint === import.meta.url) {
  await cleanNextBuildOutput();
}
