import { afterEach, describe, expect, it, vi } from "vitest";

import { buildAuthConfig } from "@/auth.config";

const canonicalIdentity = {
  tenantId: "org-286",
  userId: "zitadel-subject-123",
  username: "admin",
  userType: "zitadel",
  roles: ["listingkit_admin"],
};

function encodeIDToken(payload: Record<string, unknown>) {
  return `header.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.signature`;
}

function expiredToken(overrides: Record<string, unknown> = {}) {
  return {
    accessToken: "old-access-token",
    refreshToken: "refresh-token",
    expiresAt: Math.floor(Date.now() / 1000) - 60,
    identity: canonicalIdentity,
    identityVersion: 1,
    ...overrides,
  };
}

async function refreshSession(
  token: Record<string, unknown>,
  responsePayload: Record<string, unknown>,
) {
  vi.stubGlobal(
    "fetch",
    vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ token_endpoint: "https://issuer.example.com/token" }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify(responsePayload), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      ),
  );

  const jwt = buildAuthConfig().callbacks?.jwt;
  if (!jwt) {
    throw new Error("Auth.js JWT callback is not configured");
  }
  const result = await jwt({ token } as never);
  if (!result) {
    throw new Error("Auth.js JWT callback unexpectedly returned null");
  }
  return result;
}

describe("ListingKit Auth.js canonical ZITADEL identity", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("marks an identity only after a ZITADEL profile supplies sub", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    const jwt = buildAuthConfig().callbacks?.jwt;
    if (!jwt) {
      throw new Error("Auth.js JWT callback is not configured");
    }

    const result = await jwt({
      token: {},
      account: {
        provider: "zitadel",
        access_token: "access-token-1",
      },
      profile: {
        sub: "zitadel-subject-123",
        user_id: "legacy-user-id",
        "urn:zitadel:iam:user:resourceowner:id": "org-286",
      },
    } as never);
    if (!result) {
      throw new Error("Auth.js JWT callback unexpectedly returned null");
    }

    expect(result.identity).toMatchObject({ userId: "zitadel-subject-123" });
    expect(result.identityVersion).toBe(1);
  });

  it("invalidates identity when a refreshed ID token lacks sub", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");

    const result = await refreshSession(expiredToken(), {
      access_token: "new-access-token",
      id_token: encodeIDToken({
        user_id: "legacy-user-id",
        "urn:zitadel:iam:user:resourceowner:id": "org-286",
      }),
    });

    expect(result.identity).toBeNull();
    expect(result.identityVersion).toBeUndefined();
    expect(result.error).toBe("Refreshed ZITADEL ID token is missing a canonical subject");
  });

  it("retains identity only when a refresh omits ID token and the JWT is marked", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");

    const marked = await refreshSession(expiredToken(), {
      access_token: "new-access-token",
    });
    const unmarked = await refreshSession(
      expiredToken({ identityVersion: undefined }),
      { access_token: "new-access-token" },
    );

    expect(marked.identity).toEqual(canonicalIdentity);
    expect(marked.identityVersion).toBe(1);
    expect(unmarked.identity).toBeNull();
    expect(unmarked.identityVersion).toBeUndefined();
  });
});
