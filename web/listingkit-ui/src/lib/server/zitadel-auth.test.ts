import { afterEach, describe, expect, it, vi } from "vitest";

import {
  authorizeZitadelIdentity,
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
    expect(getZitadelAuthOptions()?.scopes.split(/\s+/)).toContain(
      "urn:zitadel:iam:org:project:role:listingkit_admin",
    );
  });
});

describe("authorizeZitadelIdentity", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("does not authorize a disallowed subject from a configured username", () => {
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "1-admin");
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USER_IDS", "allowed-subject");

    expect(
      authorizeZitadelIdentity({
        tenantId: "org-1",
        userId: "disallowed-subject",
        username: "1-admin",
        roles: ["listingkit_viewer"],
      }),
    ).toEqual({
      authorized: false,
      required: true,
      reason: "ZITADEL username allowlists are obsolete; configure canonical allowlists",
    });
  });

  it("fails closed when only an obsolete username allowlist is configured", () => {
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "legacy-admin");

    expect(
      authorizeZitadelIdentity({
        tenantId: "org-1",
        userId: "zitadel-subject-123",
        username: "legacy-admin",
        roles: ["listingkit_admin"],
      }),
    ).toEqual({
      authorized: false,
      required: true,
      reason: "ZITADEL username allowlists are obsolete; configure canonical allowlists",
    });
  });

  it("denies access when authorization is required but identity does not match", () => {
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USER_IDS", "allowed-subject");

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

  it("uses sub when user_id and username differ", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            active: true,
            sub: "zitadel-subject-123",
            user_id: "373211204509761704",
            username: "legacy-username",
            "urn:zitadel:iam:user:resourceowner:id": "org-286",
            "urn:zitadel:iam:org:project:roles": {
              listingkit_admin: {},
            },
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
      userId: "zitadel-subject-123",
      username: "legacy-username",
      roles: ["listingkit_admin"],
      userType: "zitadel",
    });
  });

  it("rejects an otherwise valid token without sub", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            active: true,
            user_id: "373211204509761704",
            username: "legacy-username",
            "urn:zitadel:iam:user:resourceowner:id": "org-286",
            "urn:zitadel:iam:org:project:roles": {
              listingkit_admin: {},
            },
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
    ).resolves.toBeNull();
  });
});

describe("readZitadelIdentityFromSession", () => {
  it("maps Auth.js session identity into the ListingKit identity contract", () => {
    expect(
      readZitadelIdentityFromSession({
        expires: "2026-05-17T00:00:00.000Z",
        accessToken: "access-token-1",
        identityVersion: 1,
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

  it("rejects an unmarked legacy Auth.js identity", () => {
    expect(
      readZitadelIdentityFromSession({
        expires: "2026-05-17T00:00:00.000Z",
        accessToken: "access-token-1",
        identity: {
          tenantId: "org-1",
          userId: "legacy-user-id",
          username: "admin",
          userType: "zitadel",
          roles: ["listingkit_admin"],
        },
      }),
    ).toBeNull();
  });
});
