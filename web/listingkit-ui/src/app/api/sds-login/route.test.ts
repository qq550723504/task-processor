import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/auth", () => ({
  auth: vi.fn(async () => null),
}));

import {
  buildSDSLoginUpstreamHeaders,
  GET,
} from "@/app/api/sds-login/[...path]/route";

describe("buildSDSLoginUpstreamHeaders", () => {
  it("forwards only generic request headers", () => {
    const headers = buildSDSLoginUpstreamHeaders(
      new Headers({
        accept: "application/json",
        authorization: "Bearer token-1",
        "tenant-id": "286",
        "visit-tenant-id": "389",
        "login-user": encodeURIComponent(JSON.stringify({ id: 42, tenantId: 286 })),
      }),
    );

    expect(headers.get("Authorization")).toBe("Bearer token-1");
    expect(headers.get("tenant-id")).toBe("286");
    expect(headers.get("X-Tenant-ID")).toBe("286");
    expect(headers.get("visit-tenant-id")).toBeNull();
    expect(headers.get("login-user")).toBeNull();
  });
});

describe("SDS login proxy ZITADEL auth", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("rejects requests without a valid ZITADEL token when auth is configured", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        Response.json({
          authorization_endpoint: "https://issuer.example/oauth/v2/authorize",
          token_endpoint: "https://issuer.example/oauth/v2/token",
          introspection_endpoint: "https://issuer.example/oauth/v2/introspect",
        }),
      ),
    );

    const response = await GET(
      new NextRequest("http://localhost/api/sds-login/status"),
      { params: Promise.resolve({ path: ["status"] }) },
    );

    expect(response.status).toBe(401);
    await expect(response.json()).resolves.toEqual(
      expect.objectContaining({ error: "zitadel_token_invalid" }),
    );
  });

  it("forwards a verified ZITADEL bearer token upstream", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        Response.json({
          authorization_endpoint: "https://issuer.example/oauth/v2/authorize",
          token_endpoint: "https://issuer.example/oauth/v2/token",
          introspection_endpoint: "https://issuer.example/oauth/v2/introspect",
        }),
      )
      .mockResolvedValueOnce(Response.json({ active: true, sub: "user-42" }))
      .mockResolvedValueOnce(
        new Response('{"success":true}', {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(
      new NextRequest("http://localhost/api/sds-login/status", {
        headers: { authorization: "Bearer access-token-1" },
      }),
      { params: Promise.resolve({ path: ["status"] }) },
    );

    expect(response.status).toBe(200);
    const upstreamInit = fetchMock.mock.calls[2]?.[1];
    expect(new Headers(upstreamInit?.headers).get("Authorization")).toBe(
      "Bearer access-token-1",
    );
  });
});
