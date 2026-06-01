import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { apiAsyncRequest, apiRequest } from "@/lib/api/client";
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
    saveAsyncJobResumeEntry(key, "job-123", "backend");

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
      "/api/listing-kits/studio/async-jobs/job-123",
    );
  });

  it("fails when the backend async job endpoint rejects the request", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ message: "backend unavailable" }), {
          status: 503,
          headers: { "content-type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const promise = apiAsyncRequest<{ images: Array<{ id: string }> }>("/studio/designs", {
      body: { prompt: "flag" },
      timeoutMs: 20_000,
    });

    await expect(promise).rejects.toMatchObject({
      message: "backend unavailable",
      status: 503,
    });
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/async-jobs",
    );
  });

  it("adds shein studio trace headers to async studio requests", async () => {
    window.sessionStorage.setItem(
      "listingkit:shein-studio:trace-context",
      JSON.stringify({
        batchRunId: "run-1",
        batchId: "batch-1",
        sessionId: "session-1",
        queueMode: "generate",
        queueIndex: 2,
        queueTotal: 5,
      }),
    );
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ message: "backend unavailable" }), {
          status: 503,
          headers: { "content-type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      apiAsyncRequest<{ images: Array<{ id: string }> }>("/studio/designs", {
        body: { prompt: "flag" },
        timeoutMs: 20_000,
      }),
    ).rejects.toMatchObject({
      message: "backend unavailable",
      status: 503,
    });

    const headers = fetchMock.mock.calls[0]?.[1]?.headers as Headers;
    expect(headers.get("X-ListingKit-Batch-Run-Id")).toBe("run-1");
    expect(headers.get("X-ListingKit-Batch-Id")).toBe("batch-1");
    expect(headers.get("X-ListingKit-Studio-Session-Id")).toBe("session-1");
    expect(headers.get("X-ListingKit-Queue-Mode")).toBe("generate");
    expect(headers.get("X-ListingKit-Queue-Index")).toBe("2");
    expect(headers.get("X-ListingKit-Queue-Total")).toBe("5");
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

describe("apiRequest", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uses plain JSON headers without browser-stored auth injection", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(apiRequest<{ ok: boolean }>("/tasks")).resolves.toEqual({
      ok: true,
    });

    const headers = fetchMock.mock.calls[0]?.[1]?.headers as Headers;
    expect(headers.get("Accept")).toBe("application/json");
    expect(headers.get("Authorization")).toBeNull();
    expect(headers.get("tenant-id")).toBeNull();
  });

  it("wraps invalid JSON responses in an ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        new Response("<html>bad gateway</html>", {
          status: 502,
          headers: { "content-type": "text/html" },
        }),
      ),
    );

    await expect(apiRequest("/tasks")).rejects.toMatchObject({
      message: "ListingKit API returned invalid JSON",
      status: 502,
      payload: { message: "Invalid JSON response: 502" },
    });
  });
});
