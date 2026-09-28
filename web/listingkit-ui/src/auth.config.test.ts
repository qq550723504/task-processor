import { afterEach, describe, expect, it, vi } from "vitest";

const mockedAcceptanceHandoff = vi.hoisted(() => ({
  persist: vi.fn(async () => false),
}));

vi.mock("@/lib/server/zitadel-acceptance-token", () => ({
  persistZitadelAcceptanceToken: mockedAcceptanceHandoff.persist,
}));

import {
  buildAuthConfig,
  buildServerAuthConfig,
  getZitadelAuthOptions,
} from "@/auth.config";

const canonicalIdentity = {
  tenantId: "org-286",
  userId: "zitadel-subject-123",
  username: "admin",
  userType: "zitadel",
  roles: ["listingkit_admin"],
};

afterEach(() => {
  mockedAcceptanceHandoff.persist.mockClear();
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

it("preserves only the validated OIDC authentication time across refresh and hides it after identity loss", async () => {
  vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
  vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
  const config = buildAuthConfig();
  const jwt = config.callbacks!.jwt!;
  const session = config.callbacks!.session!;
  const signedIn = await jwt({ token: {}, account: { provider: "zitadel", access_token: "access-token", expires_at: Math.floor(Date.now() / 1000) + 3600 }, profile: { sub: "zitadel-subject-123", "urn:zitadel:iam:user:resourceowner:id": "org-286", auth_time: 1788228000 } } as never);
  expect(signedIn).toMatchObject({ authenticatedAt: "2026-09-01T02:00:00.000Z" });
  expect(await jwt({ token: signedIn } as never)).toMatchObject({ authenticatedAt: "2026-09-01T02:00:00.000Z" });
  const refreshed = await refreshSession(expiredToken({ authenticatedAt: "2026-09-01T02:00:00.000Z" }), { access_token: "new-access-token", expires_in: 3600, id_token: encodeIDToken({ sub: canonicalIdentity.userId, "urn:zitadel:iam:user:resourceowner:id": canonicalIdentity.tenantId, auth_time: 1788229000 }) });
  expect(refreshed).toMatchObject({ accessToken: "new-access-token", authenticatedAt: "2026-09-01T02:00:00.000Z" });
  const visible = await session({ session: {}, token: signedIn } as never);
  expect(visible).toMatchObject({ authenticatedAt: "2026-09-01T02:00:00.000Z" });
  const invalid = await session({ session: {}, token: { ...signedIn, identity: null } } as never);
  expect(invalid).toMatchObject({ authenticatedAt: null });
});

it.each([undefined, 0, -1, "1788228000", 1.5, Math.floor(Date.now() / 1000) + 86400, Number.MAX_SAFE_INTEGER])("does not synthesize authentication time for invalid auth_time %s", async auth_time => {
  vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
  vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
  const jwt = buildAuthConfig().callbacks!.jwt!;
  const result = await jwt({ token: { authenticatedAt: "old actor date" }, account: { provider: "zitadel", access_token: "access-token" }, profile: { sub: "zitadel-subject-123", "urn:zitadel:iam:user:resourceowner:id": "org-286", auth_time } } as never);
  expect(result).toMatchObject({ authenticatedAt: null });
});

it("does not attach a previous subject's authentication time to a changed refresh identity", async () => {
  vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
  vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
  const result = await refreshSession(expiredToken({ authenticatedAt: "2026-09-01T02:00:00.000Z" }), { access_token: "new-access-token", expires_in: 3600, id_token: encodeIDToken({ sub: "another-subject", "urn:zitadel:iam:user:resourceowner:id": "org-286" }) });
  expect(result).toMatchObject({ authenticatedAt: null });
});

function encodeIDToken(payload: Record<string, unknown>) {
  return `header.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.signature`;
}

function expiredToken(overrides: Record<string, unknown> = {}) {
  return {
    accessToken: "old-access-token",
    refreshToken: "refresh-token",
    expiresAt: Math.floor(Date.now() / 1000) - 60,
    identity: canonicalIdentity,
    identityVersion: 3,
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

  it.each([
    "/workbench/stores?tab=active",
    "https://console.shuomiai.com/listing-kits/canonical-products",
  ])("keeps an allowlisted Auth.js return target %s", async (url) => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://auth.shuomiai.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    const redirect = buildAuthConfig().callbacks?.redirect;
    if (!redirect) {
      throw new Error("Auth.js redirect callback is not configured");
    }

    await expect(
      redirect({ url, baseUrl: "https://console.shuomiai.com" }),
    ).resolves.toBe(
      url.startsWith("/") ? `https://console.shuomiai.com${url}` : url,
    );
  });

  it.each([
    "/api/auth/callback/zitadel",
    "/\\evil.example/workbench",
    "https://console.shuomiai.com/admin",
    "https://evil.example/workbench",
  ])("rejects an unsafe Auth.js return target %s", async (url) => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://auth.shuomiai.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    const redirect = buildAuthConfig().callbacks?.redirect;
    if (!redirect) {
      throw new Error("Auth.js redirect callback is not configured");
    }

    await expect(
      redirect({ url, baseUrl: "https://console.shuomiai.com" }),
    ).resolves.toBe("https://console.shuomiai.com/");
  });

  it("preserves the trusted ZITADEL end-session redirect", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://auth.shuomiai.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    const redirect = buildAuthConfig().callbacks?.redirect;
    if (!redirect) {
      throw new Error("Auth.js redirect callback is not configured");
    }
    const endSessionUrl =
      "https://auth.shuomiai.com/oidc/v1/end_session?client_id=listingkit-client";

    await expect(
      redirect({
        url: endSessionUrl,
        baseUrl: "https://console.shuomiai.com",
      }),
    ).resolves.toBe(endSessionUrl);
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
    expect(result.identityVersion).toBe(3);
    expect(mockedAcceptanceHandoff.persist).toHaveBeenCalledWith(
      "access-token-1",
    );
  });

  it("uses the same confidential Basic client contract provisioned in ZITADEL", () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    vi.stubEnv("ZITADEL_CLIENT_SECRET", "listingkit-secret");

    const provider = buildAuthConfig().providers?.[0];
    if (!provider || typeof provider === "function") {
      throw new Error("ZITADEL provider is not configured");
    }
    expect(provider.options?.client).toMatchObject({
      token_endpoint_auth_method: "client_secret_basic",
    });
  });

  it("derives the exact ZITADEL project from the configured roles scope", () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    vi.stubEnv(
      "ZITADEL_SCOPES",
      "openid urn:zitadel:iam:org:project:project-1:roles",
    );

    expect(getZitadelAuthOptions()?.projectId).toBe("project-1");
  });

  it("keeps foreign project roles out of the Auth.js identity", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    vi.stubEnv(
      "ZITADEL_SCOPES",
      "openid urn:zitadel:iam:org:project:project-1:roles",
    );
    const jwt = buildAuthConfig().callbacks?.jwt;
    if (!jwt) {
      throw new Error("Auth.js JWT callback is not configured");
    }

    const result = await jwt({
      token: {},
      account: { provider: "zitadel", access_token: "access-token-1" },
      profile: {
        sub: "zitadel-subject-123",
        "urn:zitadel:iam:user:resourceowner:id": "org-286",
        "urn:zitadel:iam:org:project:project-1:roles": [
          { listingkit_operator: {} },
        ],
        "urn:zitadel:iam:org:project:foreign-project:roles": [
          { platform_admin: {} },
        ],
      },
    } as never);

    expect(result?.identity?.roles).toEqual(["listingkit_operator"]);
  });

  it("keeps access and ID tokens out of the browser-visible session", async () => {
    const session = buildAuthConfig().callbacks?.session;
    if (!session) {
      throw new Error("Auth.js session callback is not configured");
    }

    const result = await session({
      session: { user: {}, expires: new Date(Date.now() + 60_000).toISOString() },
      token: {
        accessToken: "access-token-1",
        idToken: "id-token-1",
        expiresAt: Math.floor(Date.now() / 1000) + 60,
        identity: canonicalIdentity,
        identityVersion: 3,
      },
    } as never);

    expect(result).not.toHaveProperty("accessToken");
    expect(result).not.toHaveProperty("idToken");
    expect(result).toMatchObject({ identity: canonicalIdentity });
  });

  it("exposes refreshed tokens only through the server auth session", async () => {
    const session = buildServerAuthConfig().callbacks?.session;
    if (!session) {
      throw new Error("Server Auth.js session callback is not configured");
    }

    const result = await session({
      session: { user: {}, expires: new Date(Date.now() + 60_000).toISOString() },
      token: {
        accessToken: "refreshed-access-token",
        idToken: "refreshed-id-token",
        expiresAt: Math.floor(Date.now() / 1000) + 60,
        identity: canonicalIdentity,
        identityVersion: 3,
      },
    } as never);

    expect(result).toMatchObject({
      accessToken: "refreshed-access-token",
      idToken: "refreshed-id-token",
      identity: canonicalIdentity,
    });
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

  it("does not upgrade a legacy session during refresh", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");

    const result = await refreshSession(expiredToken({ identityVersion: 1 }), {
      access_token: "new-access-token",
      id_token: encodeIDToken({
        sub: "zitadel-subject-123",
        preferred_username: "admin",
        "urn:zitadel:iam:user:resourceowner:id": "org-286",
        "urn:zitadel:iam:org:project:roles": {
          listingkit_admin: {},
        },
      }),
    });

    expect(result.identity).toMatchObject({ userId: "zitadel-subject-123" });
    expect(result.identityVersion).toBeUndefined();
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
    expect(marked.identityVersion).toBe(3);
    expect(unmarked.identity).toBeNull();
    expect(unmarked.identityVersion).toBeUndefined();
  });
});
