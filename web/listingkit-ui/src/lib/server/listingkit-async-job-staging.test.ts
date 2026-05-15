import { mkdtemp, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { afterEach, describe, expect, it, vi } from "vitest";

const originalCwd = process.cwd();
let tempDir: string | null = null;

async function loadStagingModule() {
  tempDir = await mkdtemp(path.join(os.tmpdir(), "listingkit-async-staging-"));
  process.chdir(tempDir);
  process.env.LISTINGKIT_UI_STORAGE_DIR = path.join(tempDir, "storage");
  vi.resetModules();
  return import("@/lib/server/listingkit-async-job-staging");
}

describe("listingkit async job staging", () => {
  afterEach(async () => {
    process.chdir(originalCwd);
    delete process.env.LISTINGKIT_UI_STORAGE_DIR;
    vi.restoreAllMocks();
    vi.resetModules();
    if (tempDir) {
      await rm(tempDir, { recursive: true, force: true });
      tempDir = null;
    }
  });

  it("can append chunks after the module is reloaded", async () => {
    const firstModule = await loadStagingModule();
    const stage = firstModule.createListingKitAsyncJobStage({
      path: "/studio/designs",
      chunkCount: 2,
    });

    vi.resetModules();
    const secondModule = await import("@/lib/server/listingkit-async-job-staging");

    expect(() =>
      secondModule.appendListingKitAsyncJobStageChunk({
        stageId: stage.stage_id,
        chunkIndex: 0,
        chunk: "{\"prompt\":",
      }),
    ).not.toThrow();
  });

  it("can start a staged job after chunks are appended across reloads", async () => {
    vi.doMock("@/lib/server/listingkit-async-jobs", () => ({
      startListingKitAsyncJob: vi.fn((input: { path: string; body: unknown }) => ({
        job_id: "job-1",
        path: input.path,
        status: "running",
        result: input.body,
      })),
    }));

    const firstModule = await loadStagingModule();
    const stage = firstModule.createListingKitAsyncJobStage({
      path: "/studio/designs",
      chunkCount: 2,
    });
    firstModule.appendListingKitAsyncJobStageChunk({
      stageId: stage.stage_id,
      chunkIndex: 0,
      chunk: "{\"prompt\":",
    });

    vi.resetModules();
    vi.doMock("@/lib/server/listingkit-async-jobs", () => ({
      startListingKitAsyncJob: vi.fn((input: { path: string; body: unknown }) => ({
        job_id: "job-1",
        path: input.path,
        status: "running",
        result: input.body,
      })),
    }));
    const secondModule = await import("@/lib/server/listingkit-async-job-staging");
    secondModule.appendListingKitAsyncJobStageChunk({
      stageId: stage.stage_id,
      chunkIndex: 1,
      chunk: "\"retro cherries\"}",
    });

    expect(secondModule.startListingKitAsyncJobFromStage(stage.stage_id)).toMatchObject({
      job_id: "job-1",
      path: "/studio/designs",
      result: { prompt: "retro cherries" },
    });
  });
});
