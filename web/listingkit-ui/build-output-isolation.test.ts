import { access, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import packageJson from "./package.json";
import { cleanNextBuildOutput } from "./scripts/clean-next-build-output.mjs";

const temporaryDirectories: string[] = [];

afterEach(async () => {
  await Promise.all(temporaryDirectories.splice(0).map(directory => rm(directory, { recursive: true, force: true })));
});

describe("Next build output isolation", () => {
  it("removes dev-generated output before every production build", async () => {
    const root = await mkdtemp(path.join(tmpdir(), "listingkit-next-build-"));
    temporaryDirectories.push(root);
    const generated = path.join(root, ".next", "dev", "types", "validator.ts");
    const source = path.join(root, "source.ts");
    await mkdir(path.dirname(generated), { recursive: true });
    await writeFile(generated, "export type Broken = {\n");
    await writeFile(source, "export const kept = true;\n");

    await cleanNextBuildOutput(root);

    await expect(access(path.join(root, ".next"))).rejects.toMatchObject({ code: "ENOENT" });
    await expect(readFile(source, "utf8")).resolves.toBe("export const kept = true;\n");
    expect(packageJson.scripts.build).toBe("node scripts/clean-next-build-output.mjs && next build");
  });
});
