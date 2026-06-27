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
  PROXY_CHILD_TASK_RETRY_UPSTREAM_TIMEOUT_MS,
  PROXY_TASK_DETAIL_UPSTREAM_TIMEOUT_MS,
  PROXY_TASK_PREVIEW_UPSTREAM_TIMEOUT_MS,
  PROXY_REVISION_UPSTREAM_TIMEOUT_MS,
  PROXY_SHEIN_ENROLLMENT_DASHBOARD_UPSTREAM_TIMEOUT_MS,
  PROXY_SHEIN_ENROLLMENT_EXECUTE_UPSTREAM_TIMEOUT_MS,
  PROXY_SHEIN_CATEGORY_SEARCH_UPSTREAM_TIMEOUT_MS,
  PROXY_STUDIO_BATCH_TASK_CREATION_UPSTREAM_TIMEOUT_MS,
  PROXY_STUDIO_UPSTREAM_TIMEOUT_MS,
  buildListingKitProxyFailureMessage,
  resolveListingKitProxyTimeoutMs,
  shouldProxyListingKitResponseAsBinary,
} from "@/app/api/listing-kits/proxy-response";
import {
  buildListingKitUpstreamHeaders,
  verifyListingKitRequestIdentity,
} from "@/app/api/listing-kits/proxy-auth";
import { verifyZitadelAccessToken } from "@/lib/server/zitadel-auth";

describe("resolveListingKitProxyTimeoutMs", () => {
  it("keeps the default timeout for regular listingkit requests", () => {
    expect(resolveListingKitProxyTimeoutMs("GET", ["tasks"])).toBe(15_000);
    expect(resolveListingKitProxyTimeoutMs("POST", ["generate"])).toBe(15_000);
  });

  it("extends the timeout for task detail and preview reads", () => {
    expect(resolveListingKitProxyTimeoutMs("GET", ["tasks", "123"])).toBe(
      PROXY_TASK_DETAIL_UPSTREAM_TIMEOUT_MS,
    );
    expect(resolveListingKitProxyTimeoutMs("GET", ["tasks", "123", "preview"])).toBe(
      PROXY_TASK_PREVIEW_UPSTREAM_TIMEOUT_MS,
    );
  });

  it("extends the timeout for task submit requests", () => {
    expect(resolveListingKitProxyTimeoutMs("POST", ["tasks", "123", "submit"])).toBe(
      180_000,
    );
  });

  it("extends the timeout for task revision requests", () => {
    expect(resolveListingKitProxyTimeoutMs("POST", ["tasks", "123", "revision"])).toBe(
      PROXY_REVISION_UPSTREAM_TIMEOUT_MS,
    );
  });

  it("extends the timeout for child task retry requests", () => {
    expect(
      resolveListingKitProxyTimeoutMs("POST", ["tasks", "123", "child-tasks", "retry"]),
    ).toBe(PROXY_CHILD_TASK_RETRY_UPSTREAM_TIMEOUT_MS);
  });

  it("extends the timeout for shein category search requests", () => {
    expect(
      resolveListingKitProxyTimeoutMs("GET", ["tasks", "123", "shein", "categories"]),
    ).toBe(PROXY_SHEIN_CATEGORY_SEARCH_UPSTREAM_TIMEOUT_MS);
  });

  it("extends the timeout for shein enrollment dashboard requests", () => {
    expect(resolveListingKitProxyTimeoutMs("GET", ["shein-sync", "dashboard"])).toBe(
      PROXY_SHEIN_ENROLLMENT_DASHBOARD_UPSTREAM_TIMEOUT_MS,
    );
  });

  it("extends the timeout for shein enrollment execution requests", () => {
    expect(
      resolveListingKitProxyTimeoutMs("POST", [
        "shein-sync",
        "stores",
        "870",
        "enrollments",
      ]),
    ).toBe(PROXY_SHEIN_ENROLLMENT_EXECUTE_UPSTREAM_TIMEOUT_MS);
  });

  it("extends the timeout for slow admin collection reads", () => {
    expect(resolveListingKitProxyTimeoutMs("GET", ["admin", "product-import-mappings"])).toBe(
      PROXY_ADMIN_COLLECTION_UPSTREAM_TIMEOUT_MS,
    );
    expect(resolveListingKitProxyTimeoutMs("GET", ["admin", "product-data"])).toBe(
      PROXY_ADMIN_COLLECTION_UPSTREAM_TIMEOUT_MS,
    );
  });

  it("extends the timeout for studio batch generation routes", () => {
    expect(resolveListingKitProxyTimeoutMs("POST", ["studio", "async-jobs"])).toBe(
      PROXY_STUDIO_UPSTREAM_TIMEOUT_MS,
    );
    expect(resolveListingKitProxyTimeoutMs("POST", ["studio", "batch-runs"])).toBe(
      PROXY_STUDIO_UPSTREAM_TIMEOUT_MS,
    );
    expect(
      resolveListingKitProxyTimeoutMs("GET", ["studio", "batch-runs", "run-1"]),
    ).toBe(PROXY_STUDIO_UPSTREAM_TIMEOUT_MS);
    expect(
      resolveListingKitProxyTimeoutMs("POST", ["studio", "sessions", "session-1", "designs", "append"]),
    ).toBe(PROXY_STUDIO_UPSTREAM_TIMEOUT_MS);
    expect(resolveListingKitProxyTimeoutMs("POST", ["studio", "batches"])).toBe(
      PROXY_STUDIO_UPSTREAM_TIMEOUT_MS,
    );
  });

  it("extends the timeout for studio batch task creation routes", () => {
    expect(
      resolveListingKitProxyTimeoutMs("POST", ["studio", "batches", "batch-1", "tasks"]),
    ).toBe(PROXY_STUDIO_BATCH_TASK_CREATION_UPSTREAM_TIMEOUT_MS);
  });
});

describe("buildListingKitProxyFailureMessage", () => {
  it("returns a deterministic timeout message when the proxy abort controller fired", () => {
    expect(
      buildListingKitProxyFailureMessage(new Error("This operation was aborted"), {
        timeoutMs: 60_000,
        timedOut: true,
      }),
    ).toBe("ListingKit upstream request timed out after 60000ms");
  });

  it("preserves the upstream error message when the request did not time out", () => {
    expect(
      buildListingKitProxyFailureMessage(new Error("upstream connection reset"), {
        timeoutMs: 60_000,
        timedOut: false,
      }),
    ).toBe("upstream connection reset");
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

  it("forwards shein studio trace headers to the upstream request", () => {
    const request = new Request("http://localhost/api/listing-kits/studio/async-jobs", {
      headers: {
        accept: "application/json",
        "x-listingkit-batch-run-id": "run-1",
        "x-listingkit-batch-id": "batch-1",
        "x-listingkit-studio-session-id": "session-1",
        "x-listingkit-queue-mode": "generate",
        "x-listingkit-queue-index": "2",
        "x-listingkit-queue-total": "5",
      },
    });

    const headers = buildListingKitUpstreamHeaders(request.headers);

    expect(headers.get("X-ListingKit-Batch-Run-Id")).toBe("run-1");
    expect(headers.get("X-ListingKit-Batch-Id")).toBe("batch-1");
    expect(headers.get("X-ListingKit-Studio-Session-Id")).toBe("session-1");
    expect(headers.get("X-ListingKit-Queue-Mode")).toBe("generate");
    expect(headers.get("X-ListingKit-Queue-Index")).toBe("2");
    expect(headers.get("X-ListingKit-Queue-Total")).toBe("5");
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

  it("rejects requests when no real ZITADEL session is present", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "client-1");

    const result = await verifyListingKitRequestIdentity(
      new NextRequest("http://localhost/api/listing-kits/tasks"),
    );

    expect(result.identity).toBeUndefined();
    expect(result.token).toBeUndefined();
    expect(result.response?.status).toBe(401);
  });

  it("accepts an incoming bearer token without ZITADEL introspection in the proxy", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "client-1");
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

    const result = await verifyListingKitRequestIdentity(
      new NextRequest("http://localhost/api/listing-kits/tasks", {
        headers: { authorization: "Bearer caller-token-1" },
      }),
    );

    expect(result.response).toBeUndefined();
    expect(result.token).toBe("caller-token-1");
    expect(result.identity).toBeUndefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("returns the Auth.js session identity when a session is present", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "client-1");
    mockedAuthState.session = {
      accessToken: "session-token-1",
      issuerUrl: "https://issuer.example.com",
      clientId: "client-1",
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

  it("forwards an Auth.js session token without revalidating the token in the proxy", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "client-1");
    mockedAuthState.session = {
      accessToken: "session-token-1",
      issuerUrl: "https://issuer.example.com",
      clientId: "client-1",
      identity: {
        tenantId: "org-286",
        userId: "user-42",
        username: "admin",
        userType: "zitadel",
        roles: ["listingkit_admin"],
      },
    };
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

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
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("forwards legacy stored sessions without proxy-side introspection", async () => {
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
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

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
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects stored sessions created for a different ZITADEL issuer or client", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "client-1");
    mockedAuthState.session = {
      accessToken: "session-token-1",
      issuerUrl: "http://localhost:8080",
      clientId: "old-client",
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

    expect(result.identity).toBeUndefined();
    expect(result.token).toBeUndefined();
    expect(result.response?.status).toBe(401);
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
