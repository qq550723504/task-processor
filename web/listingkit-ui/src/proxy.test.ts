import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

const mockedAuthState = vi.hoisted(() => ({
  session: null as Record<string, unknown> | null,
}));
const mockedServerToken = vi.hoisted(() => ({ accessToken: "" }));

vi.mock("@/auth", () => ({
  serverAuth: vi.fn((handler?: (request: NextRequest & { auth?: unknown }) => unknown) => {
    if (typeof handler === "function") {
      return (request: NextRequest) =>
        handler(Object.assign(request, { auth: mockedAuthState.session }));
    }
    return Promise.resolve(mockedAuthState.session);
  }),
}));
vi.mock("@/lib/server/zitadel-server-token", () => ({
  readZitadelServerAccessToken: vi.fn(() => mockedServerToken.accessToken),
}));

import { config, proxy } from "./proxy";

type NextRequestInit = ConstructorParameters<typeof NextRequest>[1];

function makeRequest(path: string, init?: NextRequestInit) {
  return new NextRequest(`http://localhost${path}`, init);
}

async function callProxy(path: string, init?: NextRequestInit) {
  return (await proxy(makeRequest(path, init), {} as never)) as Response;
}

function configureZitadel() {
  vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
  vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
}

function setCanonicalSession(roles: string[] = []) {
  mockedServerToken.accessToken = "token-1";
  mockedAuthState.session = {
    expires: "2026-09-01T00:00:00.000Z",
    identityVersion: 3,
    identity: {
      tenantId: "home-org",
      userId: "user-1",
      username: "operator",
      userType: "zitadel",
      roles,
    },
  };
}

describe("ListingKit ZITADEL proxy", () => {
  afterEach(() => {
    mockedAuthState.session = null;
    mockedServerToken.accessToken = "";
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
    mockedServerToken.accessToken = "token-1";
    mockedAuthState.session = {
      identityVersion: 3,
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

  it("redirects sessions issued before the current identity version", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    mockedServerToken.accessToken = "token-1";
    mockedAuthState.session = {
      identityVersion: 2,
      identity: {
        tenantId: "org-1",
        userId: "user-1",
        username: "admin",
        userType: "zitadel",
        roles: ["platform_admin"],
      },
    };

    const response = await callProxy("/listing-kits/style-gallery");

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "http://localhost/login?returnTo=%2Flisting-kits%2Fstyle-gallery",
    );
  });

  it("redirects sessions without a ZITADEL subject to login", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    mockedServerToken.accessToken = "token-1";
    mockedAuthState.session = {
      identityVersion: 3,
      identity: {
        tenantId: "org-1",
        userId: "",
        user_id: "legacy-user-1",
        roles: ["listingkit_admin"],
      },
    };

    const response = await callProxy("/listing-kits/style-gallery");

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "http://localhost/login?returnTo=%2Flisting-kits%2Fstyle-gallery",
    );
    expect(response.headers.get("X-User-ID")).toBeNull();
  });

  it("redirects non-platform administrators away from the SDS login page", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    mockedServerToken.accessToken = "token-1";
    mockedAuthState.session = {
      identityVersion: 3,
      identity: {
        tenantId: "org-1",
        userId: "user-1",
        username: "operator",
        userType: "zitadel",
        roles: ["listingkit_admin"],
      },
    };

    const response = await callProxy("/listing-kits/sds-login");

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost/unauthorized");
  });

  it("allows platform administrators to access the SDS login page", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    mockedServerToken.accessToken = "token-1";
    mockedAuthState.session = {
      identityVersion: 3,
      identity: {
        tenantId: "org-1",
        userId: "user-1",
        username: "platform-admin",
        userType: "zitadel",
        roles: ["platform_admin"],
      },
    };

    const response = await callProxy("/listing-kits/sds-login");

    expect(response.status).toBe(200);
    expect(response.headers.get("location")).toBeNull();
  });

  it("redirects authenticated but unauthorized users to the unauthorized page", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USER_IDS", "allowed-subject");
    mockedServerToken.accessToken = "token-1";
    mockedAuthState.session = {
      identityVersion: 3,
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

  it("does not allow ListingKit pages when a retired local auth bypass variable is set", async () => {
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");

    const response = await callProxy("/listing-kits/sds?step=generate");

    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toEqual({
      error: "ZITADEL auth is not configured",
    });
  });

  it("keeps the public product homepage outside the auth gate", async () => {
    const response = await callProxy("/");

    expect(response.status).toBe(200);
    expect(response.headers.get("location")).toBeNull();
  });
});

describe("Workbench ZITADEL proxy", () => {
  afterEach(() => {
    mockedAuthState.session = null;
    mockedServerToken.accessToken = "";
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  it.each([
    ["/workbench", "/workbench"],
    ["/workbench/stores?organization=org-b", "/workbench/stores?organization=org-b"],
    ["/workbench/no-organization", "/workbench/no-organization"],
  ])(
    "redirects an unauthenticated %s request to login with its local return target",
    async (path, returnTo) => {
      configureZitadel();

      const response = await callProxy(path);
      const location = new URL(response.headers.get("location")!);

      expect(response.status).toBe(307);
      expect(location.origin).toBe("http://localhost");
      expect(location.pathname).toBe("/login");
      expect(location.searchParams.get("returnTo")).toBe(returnTo);
    },
  );

  it("keeps hostile query values nested inside the same-origin login return target", async () => {
    configureZitadel();

    const response = await callProxy(
      "/workbench/stores?returnTo=https://evil.example/steal&next=//evil.example",
    );
    const location = new URL(response.headers.get("location")!);

    expect(response.status).toBe(307);
    expect(location.origin).toBe("http://localhost");
    expect(location.pathname).toBe("/login");
    expect(location.searchParams.get("returnTo")).toBe(
      "/workbench/stores?returnTo=https://evil.example/steal&next=//evil.example",
    );
  });

  it("keeps an encoded hostile path on the same-origin login redirect", async () => {
    configureZitadel();

    const response = await callProxy(
      "/workbench/%2F%2Fevil.example?next=https://evil.example",
    );
    const location = new URL(response.headers.get("location")!);

    expect(response.status).toBe(307);
    expect(location.origin).toBe("http://localhost");
    expect(location.pathname).toBe("/login");
    expect(location.searchParams.get("returnTo")).toBe(
      "/workbench/%2F%2Fevil.example?next=https://evil.example",
    );
  });

  it("does not substitute browser token or subject values for the server session token", async () => {
    configureZitadel();
    setCanonicalSession(["listingkit_admin"]);
    mockedServerToken.accessToken = "";

    const response = await callProxy(
      "/workbench?access_token=browser-token",
      {
        headers: {
          cookie: "zitadel_access_token=browser-token",
          "x-user-id": "browser-subject",
        },
      },
    );

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "http://localhost/login?returnTo=%2Fworkbench%3Faccess_token%3Dbrowser-token",
    );
  });

  it("fails closed when ZITADEL is not configured", async () => {
    const response = await callProxy("/workbench");

    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toEqual({
      error: "ZITADEL auth is not configured",
    });
  });

  it("allows a canonical subject with empty flattened roles", async () => {
    configureZitadel();
    setCanonicalSession([]);

    const response = await callProxy("/workbench/stores");

    expect(response.status).toBe(200);
    expect(response.headers.get("location")).toBeNull();
  });

  it("does not apply legacy tenant, user, role, or username allowlists", async () => {
    configureZitadel();
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS", "different-org");
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USER_IDS", "different-user");
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_ROLES", "platform_admin");
    vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "operator");
    setCanonicalSession(["listingkit_viewer"]);

    const response = await callProxy("/workbench/no-organization");

    expect(response.status).toBe(200);
    expect(response.headers.get("location")).toBeNull();
  });

  it.each([["listingkit_admin"], ["listingkit_viewer"]])(
    "admits the canonical subject without interpreting its %s flattened role",
    async (role) => {
      configureZitadel();
      vi.stubEnv("LISTINGKIT_ZITADEL_ALLOWED_ROLES", "platform_admin");
      setCanonicalSession([role]);

      const response = await callProxy("/workbench/stores");

      expect(response.status).toBe(200);
      expect(response.headers.get("location")).toBeNull();
    },
  );

  it("redirects a version-stale canonical session to login", async () => {
    configureZitadel();
    setCanonicalSession(["listingkit_admin"]);
    mockedAuthState.session = {
      ...mockedAuthState.session,
      identityVersion: 2,
    };

    const response = await callProxy("/workbench");

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "http://localhost/login?returnTo=%2Fworkbench",
    );
  });

  it("redirects a subjectless session to login", async () => {
    configureZitadel();
    setCanonicalSession(["listingkit_admin"]);
    mockedAuthState.session = {
      ...mockedAuthState.session,
      identity: {
        tenantId: "home-org",
        userId: "   ",
        user_id: "browser-controlled-legacy-id",
        roles: ["listingkit_admin"],
      },
    };

    const response = await callProxy("/workbench/stores");

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "http://localhost/login?returnTo=%2Fworkbench%2Fstores",
    );
  });

  it("redirects a session carrying an Auth.js refresh error to login", async () => {
    configureZitadel();
    setCanonicalSession(["listingkit_admin"]);
    mockedAuthState.session = {
      ...mockedAuthState.session,
      error: "RefreshAccessTokenError",
    };

    const response = await callProxy("/workbench/no-organization");

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "http://localhost/login?returnTo=%2Fworkbench%2Fno-organization",
    );
  });

  it("keeps unmatched API paths outside the page authorization branches", async () => {
    const response = await callProxy("/api/workbench/stores");

    expect(response.status).toBe(200);
    expect(response.headers.get("location")).toBeNull();
  });

  it("registers both protected page families in the static matcher", () => {
    expect(config.matcher).toEqual([
      "/listing-kits/:path*",
      "/workbench/:path*",
    ]);
  });
});
