import { mkdtemp, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { afterEach, describe, expect, it, vi } from "vitest";

const originalCwd = process.cwd();
let tempDir: string | null = null;

async function loadJobsModule() {
  tempDir = await mkdtemp(path.join(os.tmpdir(), "listingkit-async-jobs-"));
  process.chdir(tempDir);
  process.env.LISTINGKIT_UI_STORAGE_DIR = path.join(tempDir, "storage");
  vi.resetModules();
  return import("@/lib/server/listingkit-async-jobs");
}

describe("listingkit async jobs", () => {
  afterEach(async () => {
    process.chdir(originalCwd);
    delete process.env.LISTINGKIT_UI_STORAGE_DIR;
    delete process.env.LISTINGKIT_UI_ASYNC_JOB_TIMEOUT_MS;
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.resetModules();
    if (tempDir) {
      await rm(tempDir, { recursive: true, force: true });
      tempDir = null;
    }
  });

  it("can read a running job after the module is reloaded", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockReturnValue(new Promise<Response>(() => undefined)),
    );

    const firstModule = await loadJobsModule();
    const started = firstModule.startListingKitAsyncJob({
      path: "/studio/designs",
      body: { prompt: "retro cherries" },
    });

    vi.resetModules();
    const secondModule = await import("@/lib/server/listingkit-async-jobs");

    expect(secondModule.getListingKitAsyncJob(started.job_id)).toMatchObject({
      job_id: started.job_id,
      path: "/studio/designs",
      status: "running",
    });
  });

  it("marks stale running jobs as failed instead of leaving them running", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-15T00:00:00Z"));
    process.env.LISTINGKIT_UI_ASYNC_JOB_TIMEOUT_MS = "1000";
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockReturnValue(new Promise<Response>(() => undefined)),
    );

    const jobsModule = await loadJobsModule();
    const started = jobsModule.startListingKitAsyncJob({
      path: "/studio/designs",
      body: { prompt: "retro cherries" },
    });

    vi.setSystemTime(new Date("2026-05-15T00:00:02Z"));

    expect(jobsModule.getListingKitAsyncJob(started.job_id)).toMatchObject({
      job_id: started.job_id,
      status: "failed",
      error: "ListingKit async job timed out before completion",
    });

    vi.useRealTimers();
  });

  it("does not record a late upstream success after a job has timed out", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-15T00:00:00Z"));
    process.env.LISTINGKIT_UI_ASYNC_JOB_TIMEOUT_MS = "1000";
    const infoSpy = vi.spyOn(console, "info").mockImplementation(() => undefined);
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined);

    let resolveFetch: (response: Response) => void = () => undefined;
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockReturnValue(
        new Promise<Response>((resolve) => {
          resolveFetch = resolve;
        }),
      ),
    );

    const jobsModule = await loadJobsModule();
    const started = jobsModule.startListingKitAsyncJob({
      path: "/studio/designs",
      body: { prompt: "retro cherries" },
    });

    vi.setSystemTime(new Date("2026-05-15T00:00:02Z"));
    expect(jobsModule.getListingKitAsyncJob(started.job_id)).toMatchObject({
      status: "failed",
      error: "ListingKit async job timed out before completion",
    });

    resolveFetch(
      new Response(JSON.stringify({ images: [{ id: "late-success" }] }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
    await vi.waitFor(() => {
      expect(
        warnSpy.mock.calls.some((call) =>
          String(call[0]).includes("listingkit async job late response ignored"),
        ),
      ).toBe(true);
    });

    expect(jobsModule.getListingKitAsyncJob(started.job_id)).toMatchObject({
      status: "failed",
      error: "ListingKit async job timed out before completion",
    });
    expect(
      infoSpy.mock.calls.some((call) =>
        String(call[0]).includes("listingkit async job succeeded"),
      ),
    ).toBe(false);
  });
});
