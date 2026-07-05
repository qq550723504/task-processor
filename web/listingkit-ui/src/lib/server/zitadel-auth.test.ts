import { afterEach, describe, expect, it, vi } from "vitest";

import {
  authorizeZitadelIdentity,
  getListingKitLocalDebugIdentity,
  getZitadelAuthOptions,
  readZitadelIdentityFromSession,
  verifyZitadelAccessToken,
} from "@/lib/server/zitadel-auth";

describe("getZitadelAuthOptions", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("requests the ZITADEL resource owner scope by default for tenant identity", () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "http://localhost:8080");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");

    expect(getZitadelAuthOptions()?.scopes.split(/\s+/)).toContain(
      "urn:zitadel:iam:user:resourceowner",
    );
  });
});

describe("getListingKitLocalDebugIdentity", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("uses the backend default tenant and platform admin role for local data scope", () => {
    expect(getListingKitLocalDebugIdentity()).toEqual({
      tenantId: "default",
      userId: "local-debug",
      username: "local-debug",
      userType: "local_debug",
      roles: ["platform_admin", "listingkit_admin", "listingkit_operator"],
    });
  });

  it("allows local debug data scope to be overridden from environment", () => {
    vi.stubEnv("LISTINGKIT_UI_LOCAL_DEBUG_TENANT_ID", "org-286");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_DEBUG_USER_ID", "user-42");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_DEBUG_USERNAME", "debug-admin");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_DEBUG_ROLES", "listingkit_admin,platform_admin");

    expect(getListingKitLocalDebugIdentity()).toEqual({
      tenantId: "org-286",
      userId: "user-42",
      username: "debug-admin",
      userType: "local_debug",
      roles: ["listingkit_admin", "platform_admin"],
    });
  });
});

describe("authorizeZitadelIdentity", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("allows a configured username allowlist entry", () => {
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "1-admin");

    expect(
      authorizeZitadelIdentity({
        tenantId: "org-1",
        userId: "user-1",
        username: "1-admin",
        roles: ["listingkit_viewer"],
      }),
    ).toEqual({ authorized: true, required: true });
  });

  it("denies access when authorization is required but identity does not match", () => {
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "1-admin");

    expect(
      authorizeZitadelIdentity({
        tenantId: "org-2",
        userId: "user-2",
        username: "2-guest",
        roles: ["listingkit_viewer"],
      }),
    ).toEqual({
      authorized: false,
      required: true,
      reason: "ZITADEL identity is not allowed to access ListingKit",
    });
  });
});

describe("verifyZitadelAccessToken", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("prefers the business user_id over the ZITADEL subject", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            active: true,
            sub: "zitadel-subject-42",
            user_id: "373211204509761704",
            "urn:zitadel:iam:user:resourceowner:id": "org-286",
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
      ),
    );

    await expect(
      verifyZitadelAccessToken(
        "access-token-1",
        {
          issuerUrl: "https://issuer.example.com",
          clientId: "client-1",
          scopes: "openid profile",
        },
        {
          authorization_endpoint:
            "https://issuer.example.com/oauth/v2/authorize",
          token_endpoint: "https://issuer.example.com/oauth/v2/token",
          introspection_endpoint:
            "https://issuer.example.com/oauth/v2/introspect",
        },
      ),
    ).resolves.toMatchObject({
      tenantId: "org-286",
      userId: "373211204509761704",
      userType: "zitadel",
    });
  });
});

describe("readZitadelIdentityFromSession", () => {
  it("maps Auth.js session identity into the ListingKit identity contract", () => {
    expect(
      readZitadelIdentityFromSession({
        expires: "2026-05-17T00:00:00.000Z",
        accessToken: "access-token-1",
        identity: {
          tenantId: "org-1",
          userId: "user-1",
          username: "admin",
          userType: "zitadel",
          roles: ["listingkit_admin"],
        },
      }),
    ).toEqual({
      tenantId: "org-1",
      userId: "user-1",
      username: "admin",
      userType: "zitadel",
      roles: ["listingkit_admin"],
    });
  });
});
