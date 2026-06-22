import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { pollAsyncJob } from "@/lib/api/async-job";

function buildPollRequest(jobId: string) {
  return {
    url: `/api/listing-kits/studio/async-jobs/${jobId}`,
    init: {
      headers: new Headers({ Accept: "application/json" }),
      cache: "no-store" as const,
    },
  };
}

describe("pollAsyncJob", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("returns the result when a running job eventually succeeds", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ job_id: "job-1", status: "running" }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            job_id: "job-1",
            status: "succeeded",
            result: { ok: true },
          }),
          {
            status: 200,
            headers: { "content-type": "application/json" },
          },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const promise = pollAsyncJob<{ ok: boolean }>("job-1", {
      timeoutMs: 10_000,
      buildPollRequest,
    });

    await vi.advanceTimersByTimeAsync(4_200);

    await expect(promise).resolves.toEqual({ ok: true });
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("throws the backend failure payload when the job fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            job_id: "job-1",
            status: "failed",
            error: "render failed",
            upstream_status: 502,
          }),
          {
            status: 200,
            headers: { "content-type": "application/json" },
          },
        ),
      ),
    );

    const promise = pollAsyncJob("job-1", {
      timeoutMs: 10_000,
      buildPollRequest,
    });
    const assertion = expect(promise).rejects.toMatchObject({
      message: "render failed",
      status: 502,
      payload: expect.objectContaining({ error: "render failed" }),
    });

    await vi.advanceTimersByTimeAsync(2_100);

    await assertion;
  });

  it("times out when the job never reaches a terminal status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockImplementation(async () =>
        new Response(JSON.stringify({ job_id: "job-1", status: "running" }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      ),
    );

    const promise = pollAsyncJob("job-1", {
      timeoutMs: 3_000,
      buildPollRequest,
    });
    const assertion = expect(promise).rejects.toMatchObject({
      message: "ListingKit async job timed out after 3000ms",
      status: 408,
    });

    await vi.advanceTimersByTimeAsync(4_200);

    await assertion;
  });

  it("honors abort signals before polling", async () => {
    const controller = new AbortController();
    controller.abort(new Error("cancelled"));
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      pollAsyncJob("job-1", {
        timeoutMs: 10_000,
        signal: controller.signal,
        buildPollRequest,
      }),
    ).rejects.toThrow("cancelled");
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
