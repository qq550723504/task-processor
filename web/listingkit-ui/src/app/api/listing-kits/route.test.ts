import { afterEach, describe, expect, it, vi } from "vitest";

import {
  buildListingKitUpstreamHeaders,
  resolveListingKitProxyTimeoutMs,
  shouldBypassYudaoTokenVerification,
  shouldProxyListingKitResponseAsBinary,
  verifyYudaoAccessToken,
} from "@/app/api/listing-kits/[...path]/route";

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
  it("maps yudao gateway login-user and tenant-id headers to listingkit identity headers", () => {
    const loginUser = encodeURIComponent(
      JSON.stringify({ id: 42, tenantId: 286, userType: 2 }),
    );
    const request = new Request("http://localhost/api/listing-kits/tasks", {
      headers: {
        accept: "application/json",
        authorization: "Bearer user-token",
        "tenant-id": "286",
        "login-user": loginUser,
      },
    });

    const headers = buildListingKitUpstreamHeaders(request.headers);

    expect(headers.get("Authorization")).toBe("Bearer user-token");
    expect(headers.get("tenant-id")).toBe("286");
    expect(headers.get("login-user")).toBe(loginUser);
    expect(headers.get("X-Tenant-ID")).toBe("286");
    expect(headers.get("X-User-ID")).toBe("42");
    expect(headers.get("X-User-Type")).toBe("2");
  });

  it("prefers verified yudao token identity over browser-supplied tenant headers", () => {
    const request = new Request("http://localhost/api/listing-kits/tasks", {
      headers: {
        accept: "application/json",
        authorization: "Bearer user-token",
        "tenant-id": "999",
      },
    });

    const headers = buildListingKitUpstreamHeaders(request.headers, {
      tenantId: 286,
      userId: 42,
      userType: 2,
    });

    expect(headers.get("Authorization")).toBe("Bearer user-token");
    expect(headers.get("tenant-id")).toBe("286");
    expect(headers.get("X-Tenant-ID")).toBe("286");
    expect(headers.get("X-User-ID")).toBe("42");
    expect(headers.get("X-User-Type")).toBe("2");
  });
});

describe("verifyYudaoAccessToken", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
  });

  it("can bypass token verification in non-production local dev", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("YUDAO_DEV_BYPASS_TOKEN_VERIFICATION", "1");

    await expect(
      verifyYudaoAccessToken("Bearer access-token-1", {
        checkTokenUrl: "http://127.0.0.1:48080/admin-api/system/oauth2/check-token",
        clientId: "default",
        clientSecret: "secret",
        tenantId: "286",
      }),
    ).resolves.toEqual({
      tenantId: "286",
      userId: "dev-user",
      userType: "1",
    });
  });

  it("checks bearer token through yudao oauth check-token", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          code: 0,
          data: {
            user_id: 42,
            user_type: 2,
            tenant_id: 286,
          },
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      verifyYudaoAccessToken("Bearer access-token-1", {
        checkTokenUrl: "http://127.0.0.1:48080/admin-api/system/oauth2/check-token",
        clientId: "default",
        clientSecret: "secret",
        tenantId: "286",
      }),
    ).resolves.toEqual({
      tenantId: 286,
      userId: 42,
      userType: 2,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "http://127.0.0.1:48080/admin-api/system/oauth2/check-token",
      expect.objectContaining({
        body: "client_id=default&client_secret=secret&token=access-token-1",
        method: "POST",
      }),
    );
    const init = fetchMock.mock.calls[0]?.[1];
    const headers = new Headers(init?.headers);
    expect(headers.has("Authorization")).toBe(false);
    expect(headers.get("tenant-id")).toBe("286");
    expect(headers.get("Content-Type")).toBe(
      "application/x-www-form-urlencoded",
    );
  });
});

describe("shouldBypassYudaoTokenVerification", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("only enables bypass outside production when the env flag is set", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("YUDAO_DEV_BYPASS_TOKEN_VERIFICATION", "1");
    expect(shouldBypassYudaoTokenVerification()).toBe(true);

    vi.stubEnv("NODE_ENV", "production");
    expect(shouldBypassYudaoTokenVerification()).toBe(false);
  });
});
