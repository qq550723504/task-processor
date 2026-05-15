import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { proxy } from "./proxy";

function makeRequest(path: string) {
  return new NextRequest(`http://localhost${path}`);
}

function encodeSession(session: unknown) {
  return Buffer.from(JSON.stringify(session), "utf8")
    .toString("base64")
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/g, "");
}

describe("ListingKit ZITADEL proxy", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("redirects unauthenticated ListingKit pages to ZITADEL login", () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");

    const response = proxy(makeRequest("/listing-kits/shein?step=generate"));

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "http://localhost/api/zitadel-auth/login?returnTo=%2Flisting-kits%2Fshein%3Fstep%3Dgenerate",
    );
  });

  it("allows ListingKit pages with a valid ZITADEL session cookie", () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    const request = makeRequest("/listing-kits/style-gallery");
    request.cookies.set(
      "listingkit_zitadel_session",
      encodeSession({
        accessToken: "token",
        expiresAt: Math.floor(Date.now() / 1000) + 60,
      }),
    );

    const response = proxy(request);

    expect(response.status).toBe(200);
    expect(response.headers.get("location")).toBeNull();
  });

  it("keeps the local bypass path available outside production", () => {
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");

    const response = proxy(makeRequest("/listing-kits/shein"));

    expect(response.status).toBe(200);
    expect(response.headers.get("location")).toBeNull();
  });

  it("returns 503 when ZITADEL auth is required but not configured", async () => {
    const response = proxy(makeRequest("/listing-kits/shein"));

    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toEqual({
      error: "ZITADEL auth is not configured",
    });
  });
});
