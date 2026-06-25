import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  pollAsyncJob,
  resumeOrRestartAsyncJob,
  startAsyncJob,
} from "@/lib/api/async-job";

function buildPollRequest(jobId: string) {
  return {
    url: `/api/listing-kits/studio/async-jobs/${jobId}`,
    init: {
      headers: new Headers({ Accept: "application/json" }),
      cache: "no-store" as const,
    },
  };
}

function buildStartRequest(input: { path: string; body: unknown; sessionId?: string }) {
  return {
    url: "/api/listing-kits/studio/async-jobs",
    init: {
      method: "POST",
      headers: new Headers({
        Accept: "application/json",
        "Content-Type": "application/json",
      }),
      body: JSON.stringify({
        path: input.path,
        body: input.body,
        session_id: input.sessionId,
      }),
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

describe("startAsyncJob", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("starts a backend async job through the provided request builder", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ job_id: "job-new", status: "running" }), {
        status: 202,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      startAsyncJob({
        input: {
          path: "/studio/designs",
          body: { prompt: "flag" },
          sessionId: "session-1",
        },
        buildStartRequest,
      }),
    ).resolves.toEqual({ jobId: "job-new" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/async-jobs",
    );
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      path: "/studio/designs",
      body: { prompt: "flag" },
      session_id: "session-1",
    });
  });
});

describe("resumeOrRestartAsyncJob", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("resumes an existing job without starting a replacement", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          job_id: "job-existing",
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

    const promise = resumeOrRestartAsyncJob<
      { ok: boolean },
      { path: string; body: unknown; sessionId?: string }
    >(
      { path: "/studio/designs", body: { prompt: "flag" } },
      {
        jobId: "job-existing",
        timeoutMs: 10_000,
        buildStartRequest,
        buildPollRequest,
      },
    );

    await vi.advanceTimersByTimeAsync(2_100);

    await expect(promise).resolves.toEqual({ ok: true });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/async-jobs/job-existing",
    );
  });

  it("restarts a missing resumed job without losing session or start callback context", async () => {
    const onJobStarted = vi.fn();
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ message: "missing job" }), {
          status: 404,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ job_id: "job-restarted", status: "running" }), {
          status: 202,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            job_id: "job-restarted",
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

    const promise = resumeOrRestartAsyncJob<
      { ok: boolean },
      { path: string; body: unknown; sessionId?: string }
    >(
      {
        path: "/studio/designs",
        body: { prompt: "flag" },
        sessionId: "session-1",
      },
      {
        jobId: "job-missing",
        timeoutMs: 10_000,
        onJobStarted,
        buildStartRequest,
        buildPollRequest,
      },
    );

    await vi.advanceTimersByTimeAsync(4_300);

    await expect(promise).resolves.toEqual({ ok: true });
    expect(onJobStarted).toHaveBeenCalledWith("job-restarted");
    expect(fetchMock.mock.calls.map((call) => call[0])).toEqual([
      "/api/listing-kits/studio/async-jobs/job-missing",
      "/api/listing-kits/studio/async-jobs",
      "/api/listing-kits/studio/async-jobs/job-restarted",
    ]);
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toEqual({
      path: "/studio/designs",
      body: { prompt: "flag" },
      session_id: "session-1",
    });
  });
});
