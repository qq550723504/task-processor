import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { apiAsyncRequest } from "@/lib/api/client";
import { buildAsyncJobResumeKey, saveAsyncJobResumeEntry } from "@/lib/api/async-job-resume";

describe("apiAsyncRequest", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    window.localStorage.clear();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("keeps polling after transient poll failures", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ job_id: "job-1", status: "running" }), {
          status: 202,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockRejectedValueOnce(new Error("aborted"))
      .mockRejectedValueOnce(new Error("aborted"))
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            job_id: "job-1",
            status: "succeeded",
            result: { images: [{ id: "1" }] },
          }),
          {
            status: 200,
            headers: { "content-type": "application/json" },
          },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const promise = apiAsyncRequest<{ images: Array<{ id: string }> }>(
      "/studio/designs",
      {
        body: { prompt: "flag" },
        timeoutMs: 20_000,
      },
    );

    await vi.advanceTimersByTimeAsync(6_500);

    await expect(promise).resolves.toEqual({
      images: [{ id: "1" }],
    });
  });

  it("resumes an existing async job instead of starting a duplicate one", async () => {
    const key = buildAsyncJobResumeKey("/studio/designs", { prompt: "flag" });
    saveAsyncJobResumeEntry(key, "job-123");

    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          job_id: "job-123",
          status: "succeeded",
          result: { images: [{ id: "1" }] },
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const promise = apiAsyncRequest<{ images: Array<{ id: string }> }>(
      "/studio/designs",
      {
        body: { prompt: "flag" },
        timeoutMs: 20_000,
      },
    );

    await vi.advanceTimersByTimeAsync(2_100);

    await expect(promise).resolves.toEqual({
      images: [{ id: "1" }],
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/async-jobs?id=job-123",
    );
  });

  it("fails immediately when async job status becomes failed", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ job_id: "job-2", status: "running" }), {
          status: 202,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            job_id: "job-2",
            status: "failed",
            error: "upload to s3 failed",
            upstream_status: 500,
          }),
          {
            status: 200,
            headers: { "content-type": "application/json" },
          },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const promise = apiAsyncRequest<{ images: Array<{ id: string }> }>(
      "/studio/designs",
      {
        body: { prompt: "flag" },
        timeoutMs: 20_000,
      },
    );

    const assertion = expect(promise).rejects.toMatchObject({
      message: "upload to s3 failed",
      status: 500,
    });

    await vi.advanceTimersByTimeAsync(2_100);

    await assertion;
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
