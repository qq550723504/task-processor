import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

const mockedAuthState = vi.hoisted(() => ({
  session: null as Record<string, unknown> | null,
}));

vi.mock("@/auth", () => ({
  auth: vi.fn((handler?: (request: NextRequest & { auth?: unknown }) => unknown) => {
    if (typeof handler === "function") {
      return (request: NextRequest) =>
        handler(Object.assign(request, { auth: mockedAuthState.session }));
    }
    return Promise.resolve(mockedAuthState.session);
  }),
}));

import { proxy } from "./proxy";

function makeRequest(path: string) {
  return new NextRequest(`http://localhost${path}`);
}

async function callProxy(path: string) {
  return (await proxy(makeRequest(path), {} as never)) as Response;
}

describe("ListingKit ZITADEL proxy", () => {
  afterEach(() => {
    mockedAuthState.session = null;
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  it("redirects unauthenticated ListingKit pages to ZITADEL login", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");

    const response = await callProxy("/listing-kits/sds?step=generate");

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "http://localhost/login?returnTo=%2Flisting-kits%2Fsds%3Fstep%3Dgenerate",
    );
  });

  it("allows ListingKit pages with a valid Auth.js session", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    mockedAuthState.session = {
      accessToken: "token-1",
      identity: {
        tenantId: "org-1",
        userId: "user-1",
        username: "admin",
        userType: "zitadel",
        roles: [],
      },
    };

    const response = await callProxy("/listing-kits/style-gallery");

    expect(response.status).toBe(200);
    expect(response.headers.get("location")).toBeNull();
  });

  it("redirects authenticated but unauthorized users to the unauthorized page", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "1-admin");
    mockedAuthState.session = {
      accessToken: "token-1",
      identity: {
        tenantId: "org-1",
        userId: "user-2",
        username: "guest",
        userType: "zitadel",
        roles: [],
      },
    };

    const response = await callProxy("/listing-kits/admin/stores");

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost/unauthorized");
  });

  it("returns 503 when ZITADEL auth is required but not configured", async () => {
    const response = await callProxy("/listing-kits/sds");

    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toEqual({
      error: "ZITADEL auth is not configured",
    });
  });
});
