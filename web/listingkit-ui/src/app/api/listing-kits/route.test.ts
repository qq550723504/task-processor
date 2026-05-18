import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

const mockedAuthState = vi.hoisted(() => ({
  session: null as Record<string, unknown> | null,
}));

vi.mock("@/auth", () => ({
  auth: vi.fn(() => Promise.resolve(mockedAuthState.session)),
}));

import {
  PROXY_ADMIN_COLLECTION_UPSTREAM_TIMEOUT_MS,
  resolveListingKitProxyTimeoutMs,
  shouldProxyListingKitResponseAsBinary,
} from "@/app/api/listing-kits/proxy-response";
import {
  buildListingKitUpstreamHeaders,
  shouldBypassListingKitProxyAuth,
  verifyListingKitRequestIdentity,
} from "@/app/api/listing-kits/proxy-auth";
import { verifyZitadelAccessToken } from "@/lib/server/zitadel-auth";

describe("resolveListingKitProxyTimeoutMs", () => {
  it("keeps the default timeout for regular listingkit requests", () => {
    expect(resolveListingKitProxyTimeoutMs("GET", ["tasks"])).toBe(15_000);
    expect(resolveListingKitProxyTimeoutMs("POST", ["generate"])).toBe(15_000);
    expect(resolveListingKitProxyTimeoutMs("POST", ["tasks", "123", "preview"])).toBe(
      15_000,
    );
  });

  it("extends the timeout for task submit requests", () => {
    expect(resolveListingKitProxyTimeoutMs("POST", ["tasks", "123", "submit"])).toBe(
      180_000,
    );
  });

  it("extends the timeout for slow admin collection reads", () => {
    expect(resolveListingKitProxyTimeoutMs("GET", ["admin", "product-import-mappings"])).toBe(
      PROXY_ADMIN_COLLECTION_UPSTREAM_TIMEOUT_MS,
    );
    expect(resolveListingKitProxyTimeoutMs("GET", ["admin", "product-data"])).toBe(
      PROXY_ADMIN_COLLECTION_UPSTREAM_TIMEOUT_MS,
    );
  });
});

describe("shouldProxyListingKitResponseAsBinary", () => {
  it("treats uploaded file routes as binary even when content type is generic", () => {
    expect(
      shouldProxyListingKitResponseAsBinary("application/octet-stream", [
        "uploads",
        "files",
        "20260505",
        "demo.png",
      ]),
    ).toBe(true);
  });

  it("keeps json task endpoints on text mode", () => {
    expect(
      shouldProxyListingKitResponseAsBinary("application/json; charset=utf-8", [
        "tasks",
        "123",
      ]),
    ).toBe(false);
  });
});

describe("buildListingKitUpstreamHeaders", () => {
  it("maps verified ZITADEL identity to listingkit identity headers", () => {
    const request = new Request("http://localhost/api/listing-kits/tasks", {
      headers: {
        accept: "application/json",
        authorization: "Bearer user-token",
      },
    });

    const headers = buildListingKitUpstreamHeaders(request.headers, {
      tenantId: "org-286",
      userId: "user-42",
      userType: "zitadel",
      roles: ["platform_admin", "listingkit_admin"],
    });

    expect(headers.get("Authorization")).toBe("Bearer user-token");
    expect(headers.get("tenant-id")).toBe("org-286");
    expect(headers.get("X-Tenant-ID")).toBe("org-286");
    expect(headers.get("X-User-ID")).toBe("user-42");
    expect(headers.get("X-User-Type")).toBe("zitadel");
    expect(headers.get("X-User-Roles")).toBe("platform_admin,listingkit_admin");
  });

  it("does not forward legacy gateway-only headers", () => {
    const request = new Request("http://localhost/api/listing-kits/tasks", {
      headers: {
        accept: "application/json",
        "visit-tenant-id": "389",
        "login-user": encodeURIComponent(JSON.stringify({ id: 42 })),
      },
    });

    const headers = buildListingKitUpstreamHeaders(request.headers);

    expect(headers.get("visit-tenant-id")).toBeNull();
    expect(headers.get("login-user")).toBeNull();
  });
});

describe("shouldBypassListingKitProxyAuth", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("only allows proxy auth bypass outside production with the local bypass flag", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");
    expect(shouldBypassListingKitProxyAuth()).toBe(true);

    vi.stubEnv("NODE_ENV", "production");
    expect(shouldBypassListingKitProxyAuth()).toBe(false);
  });
});

describe("verifyListingKitRequestIdentity", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
    mockedAuthState.session = null;
  });

  it("returns 503 when ZITADEL auth is required but not configured", async () => {
    const request = new NextRequest("http://localhost/api/sds/products");

    const result = await verifyListingKitRequestIdentity(request);

    expect(result.response?.status).toBe(503);
  });

  it("returns a local bypass identity when auth is disabled in local development", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_TENANT_ID", "1");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_USER_ID", "local-admin");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_USER_TYPE", "local");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_ROLES", "platform_admin,listingkit_admin");

    const result = await verifyListingKitRequestIdentity(
      new NextRequest("http://localhost/api/listing-kits/tasks"),
    );

    expect(result.response).toBeUndefined();
    expect(result.token).toBe("");
    expect(result.identity).toEqual({
      tenantId: "1",
      userId: "local-admin",
      userType: "local",
      roles: ["platform_admin", "listingkit_admin"],
    });
  });

  it("omits a local bypass user id unless explicitly configured", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_TENANT_ID", "1");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_USER_ID", "");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_USER_TYPE", "local");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_ROLES", "platform_admin,listingkit_admin");

    const result = await verifyListingKitRequestIdentity(
      new NextRequest("http://localhost/api/listing-kits/tasks"),
    );

    expect(result.response).toBeUndefined();
    expect(result.identity).toEqual({
      tenantId: "1",
      userType: "local",
      roles: ["platform_admin", "listingkit_admin"],
    });

    const headers = buildListingKitUpstreamHeaders(new Headers(), result.identity);
    expect(headers.get("X-User-ID")).toBeNull();
    expect(headers.get("X-User-Roles")).toBe("platform_admin,listingkit_admin");
  });

  it("returns the Auth.js session identity when a session is present", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "client-1");
    mockedAuthState.session = {
      accessToken: "session-token-1",
      identity: {
        tenantId: "org-286",
        userId: "user-42",
        username: "admin",
        userType: "zitadel",
        roles: ["listingkit_admin"],
      },
    };

    const result = await verifyListingKitRequestIdentity(
      new NextRequest("http://localhost/api/listing-kits/tasks"),
    );

    expect(result.response).toBeUndefined();
    expect(result.token).toBe("session-token-1");
    expect(result.identity).toEqual({
      tenantId: "org-286",
      userId: "user-42",
      username: "admin",
      userType: "zitadel",
      roles: ["listingkit_admin"],
    });
  });
});

describe("verifyZitadelAccessToken", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("checks bearer token through ZITADEL introspection", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          active: true,
          sub: "user-42",
          "urn:zitadel:iam:user:resourceowner:id": "org-286",
          "urn:zitadel:iam:org:project:roles": {
            platform_admin: {},
          },
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      verifyZitadelAccessToken(
        "access-token-1",
        {
          issuerUrl: "https://issuer.example.com",
          clientId: "client-1",
          clientSecret: "secret-1",
          scopes: "openid profile",
        },
        {
          authorization_endpoint: "https://issuer.example.com/oauth/v2/authorize",
          token_endpoint: "https://issuer.example.com/oauth/v2/token",
          introspection_endpoint:
            "https://issuer.example.com/oauth/v2/introspect",
        },
      ),
    ).resolves.toEqual({
      tenantId: "org-286",
      userId: "user-42",
      userType: "zitadel",
      roles: ["platform_admin"],
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "https://issuer.example.com/oauth/v2/introspect",
      expect.objectContaining({
        body: "token=access-token-1&token_type_hint=access_token",
        method: "POST",
      }),
    );
    const init = fetchMock.mock.calls[0]?.[1];
    const headers = new Headers(init?.headers);
    expect(headers.get("Authorization")).toMatch(/^Basic /);
    expect(headers.get("Content-Type")).toBe(
      "application/x-www-form-urlencoded",
    );
  });
});

