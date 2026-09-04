import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";

import { apiRequest } from "@/lib/api/client";

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
