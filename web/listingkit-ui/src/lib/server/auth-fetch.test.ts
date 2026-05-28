import { describe, expect, it, vi } from "vitest";

import { createResilientOidcFetch } from "@/lib/server/auth-fetch";

describe("createResilientOidcFetch", () => {
  it("retries connect timeout errors for OIDC endpoints", async () => {
    const response = new Response(JSON.stringify({ ok: true }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockRejectedValueOnce(
        Object.assign(new TypeError("fetch failed"), {
          cause: { code: "UND_ERR_CONNECT_TIMEOUT" },
        }),
      )
      .mockResolvedValueOnce(response);

    const oidcFetch = createResilientOidcFetch({
      fetchImpl,
      retries: 1,
      retryDelayMs: 0,
      timeoutMs: 1000,
    });

    await expect(
      oidcFetch("https://auth.shuomiai.com/oauth/v2/token", {
        method: "POST",
      }),
    ).resolves.toBe(response);
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it("does not retry completed HTTP responses", async () => {
    const response = new Response(
      JSON.stringify({ error: "invalid_request" }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    );
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(response);

    const oidcFetch = createResilientOidcFetch({
      fetchImpl,
      retries: 2,
      retryDelayMs: 0,
      timeoutMs: 1000,
    });

    await expect(
      oidcFetch("https://auth.shuomiai.com/oauth/v2/token", {
        method: "POST",
      }),
    ).resolves.toBe(response);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });
});
