import { mkdtemp, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { afterEach, describe, expect, it, vi } from "vitest";

const originalCwd = process.cwd();
let tempDir: string | null = null;

async function loadStorageModule() {
  tempDir = await mkdtemp(path.join(os.tmpdir(), "shein-studio-storage-"));
  process.chdir(tempDir);
  vi.resetModules();
  return import("@/lib/server/shein-studio-storage");
}

describe("shein studio server storage", () => {
  afterEach(async () => {
    process.chdir(originalCwd);
    delete process.env.LISTINGKIT_UI_STORAGE_DIR;
    vi.resetModules();
    if (tempDir) {
      await rm(tempDir, { recursive: true, force: true });
      tempDir = null;
    }
  });

  it("uses LISTINGKIT_UI_STORAGE_DIR when configured", async () => {
    const workspace = await mkdtemp(path.join(os.tmpdir(), "shein-studio-workspace-"));
    const storageDir = await mkdtemp(path.join(os.tmpdir(), "shein-studio-storage-dir-"));
    tempDir = workspace;
    process.chdir(workspace);
    process.env.LISTINGKIT_UI_STORAGE_DIR = storageDir;
    vi.resetModules();

    const { saveSheinStudioDraft } = await import("@/lib/server/shein-studio-storage");

    await saveSheinStudioDraft({
      prompt: "persistent storage",
      styleCount: "1",
      sheinStoreId: "869",
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });

    const raw = await readFile(
      path.join(storageDir, "shein-studio-storage.json"),
      "utf8",
    );
    const stored = JSON.parse(raw) as { draft?: { prompt?: string } };
    expect(stored.draft?.prompt).toBe("persistent storage");

    await rm(storageDir, { recursive: true, force: true });
  });

  it("preserves concurrent batch saves", async () => {
    const { saveSheinStudioBatch } = await loadStorageModule();

    const [first, second] = await Promise.all([
      saveSheinStudioBatch({
        id: "batch-1",
        prompt: "first batch",
        styleCount: "1",
        sheinStoreId: "869",
        designs: [],
        selectedIds: [],
        createdTasks: [],
      }),
      saveSheinStudioBatch({
        id: "batch-2",
        prompt: "second batch",
        styleCount: "1",
        sheinStoreId: "869",
        designs: [],
        selectedIds: [],
        createdTasks: [],
      }),
    ]);

    expect(first?.id).toBe("batch-1");
    expect(second?.id).toBe("batch-2");

    const raw = await readFile(path.join(tempDir!, ".data", "shein-studio-storage.json"), "utf8");
    const stored = JSON.parse(raw) as { batches?: Array<{ id: string }> };
    expect(stored.batches?.map((batch) => batch.id).sort()).toEqual(["batch-1", "batch-2"]);
  });
});
