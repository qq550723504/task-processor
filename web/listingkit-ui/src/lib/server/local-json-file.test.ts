import { mkdtemp, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { afterEach, describe, expect, it } from "vitest";

import {
  readLocalJsonFile,
  readLocalJsonFileSync,
  writeLocalJsonFile,
  writeLocalJsonFileSync,
} from "@/lib/server/local-json-file";

let tempDir: string | null = null;

describe("local json file helpers", () => {
  afterEach(async () => {
    delete process.env.LISTINGKIT_UI_STORAGE_DIR;
    if (tempDir) {
      await rm(tempDir, { recursive: true, force: true });
      tempDir = null;
    }
  });

  it("returns fallback when the file is missing or invalid", async () => {
    tempDir = await mkdtemp(path.join(os.tmpdir(), "listingkit-local-json-"));
    process.env.LISTINGKIT_UI_STORAGE_DIR = tempDir;

    expect(readLocalJsonFileSync("missing.json", { ok: false })).toEqual({
      ok: false,
    });
  });

  it("writes json atomically under LISTINGKIT_UI_STORAGE_DIR", async () => {
    tempDir = await mkdtemp(path.join(os.tmpdir(), "listingkit-local-json-"));
    process.env.LISTINGKIT_UI_STORAGE_DIR = tempDir;

    writeLocalJsonFileSync("state.json", { ok: true });

    await expect(readFile(path.join(tempDir, "state.json"), "utf8")).resolves.toBe(
      `${JSON.stringify({ ok: true }, null, 2)}\n`,
    );
    expect(readLocalJsonFileSync("state.json", { ok: false })).toEqual({
      ok: true,
    });
  });

  it("supports async reads and writes", async () => {
    tempDir = await mkdtemp(path.join(os.tmpdir(), "listingkit-local-json-"));
    process.env.LISTINGKIT_UI_STORAGE_DIR = tempDir;

    await writeLocalJsonFile("async-state.json", { count: 2 });

    await expect(
      readLocalJsonFile("async-state.json", { count: 0 }),
    ).resolves.toEqual({ count: 2 });
  });
});
