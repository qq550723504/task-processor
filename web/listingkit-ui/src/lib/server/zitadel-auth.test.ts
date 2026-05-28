import { afterEach, describe, expect, it, vi } from "vitest";

import {
  authorizeZitadelIdentity,
  getZitadelAuthOptions,
  readZitadelIdentityFromSession,
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
