import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

import { stageAsyncJobRequestIfNeeded } from "@/lib/api/async-job-staging";

describe("stageAsyncJobRequestIfNeeded", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uses plain JSON headers for staged async job uploads", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ stage_id: "stage-1" }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValue(
        new Response(JSON.stringify({ ok: true }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      stageAsyncJobRequestIfNeeded({
        path: "/studio/designs",
        body: { prompt: "x".repeat(1500) },
      }),
    ).resolves.toEqual({ staged: true, stageId: "stage-1" });

    for (const [, init] of fetchMock.mock.calls) {
      const headers = new Headers(init?.headers);
      expect(headers.get("Accept")).toBe("application/json");
      expect(headers.get("Content-Type")).toBe("application/json");
      expect(headers.get("Authorization")).toBeNull();
    }
  });
});
