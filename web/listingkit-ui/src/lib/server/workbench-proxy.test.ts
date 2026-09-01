import { afterEach, describe, expect, it, vi } from "vitest";

import {
  buildWorkbenchBrowserResponse,
  buildWorkbenchUpstreamRequest,
  WORKBENCH_COOKIE_NAME,
} from "@/lib/server/workbench-proxy";

const contextPayload = {
  user: { id: "user-1" },
  homeOrganizationId: "org-a",
  effectiveOrganizationId: "org-b",
  selectionRequired: false,
  organizations: [{ id: "org-b", name: "Organization B", roles: [] }],
};

const storeId = "11111111-1111-4111-8111-11111111111a";
const operationKey = "22222222-2222-4222-8222-22222222222b";
const storePayload = {
  id: storeId,
  name: "Store",
  platform: "shein",
  region: "SG",
  externalStoreId: "external-1",
  lifecycleStatus: "active",
  connectionStatus: "connected",
  version: 2,
  createdAt: "2026-08-30T01:02:03Z",
  updatedAt: "2026-08-30T02:03:04Z",
};

describe("buildWorkbenchUpstreamRequest", () => {
  it("fails closed before reading a Store body when the captured Organization differs from the selection cookie", async () => {
    const request = new Request("http://localhost/api/workbench/stores", {
      method: "POST",
      headers: {
        cookie: "shuomi_effective_organization=org-cookie",
        "X-Expected-Organization-ID": "org-captured",
      },
      body: "{}",
    });

    const response = await buildWorkbenchUpstreamRequest(
      request,
      ["stores"],
      "token",
    );

    expect(response).toBeInstanceOf(Response);
    expect((response as Response).status).toBe(409);
    await expect((response as Response).json()).resolves.toMatchObject({
      code: "ORGANIZATION_CONTEXT_CHANGED",
    });
    expect(request.bodyUsed).toBe(false);
  });

  it.each([
    ["missing", undefined],
    ["empty", ""],
    ["comma-collapsed duplicate", "org-cookie, org-cookie"],
    ["unsafe", "org cookie"],
    ["oversized", `o${"x".repeat(128)}`],
  ])("rejects a %s Store expected Organization assertion", async (_name, expected) => {
    const headers = new Headers({ cookie: "shuomi_effective_organization=org-cookie" });
    if (expected !== undefined) headers.set("X-Expected-Organization-ID", expected);
    const response = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/stores?page=1", { method: "GET", headers }),
      ["stores"],
      "token",
    );
    expect(response).toBeInstanceOf(Response);
    expect((response as Response).status).toBe(409);
    await expect((response as Response).json()).resolves.toMatchObject({
      code: "ORGANIZATION_CONTEXT_CHANGED",
    });
  });

  it.each([
    ["list", "GET", ["stores"], "http://localhost/api/workbench/stores?page=1", {}, undefined],
    ["create", "POST", ["stores"], "http://localhost/api/workbench/stores", { "Idempotency-Key": operationKey }, JSON.stringify({ name: "Store", platform: "shein", region: "SG" })],
    ["get", "GET", ["stores", storeId], `http://localhost/api/workbench/stores/${storeId}`, {}, undefined],
    ["update", "PUT", ["stores", storeId], `http://localhost/api/workbench/stores/${storeId}`, { "If-Match": '"2"' }, JSON.stringify({ name: "Store", region: "SG" })],
    ["delete", "DELETE", ["stores", storeId], `http://localhost/api/workbench/stores/${storeId}`, { "Idempotency-Key": operationKey, "If-Match": '"2"' }, undefined],
    ["enable", "POST", ["stores", storeId, "enable"], `http://localhost/api/workbench/stores/${storeId}/enable`, { "If-Match": '"2"' }, undefined],
    ["disable", "POST", ["stores", storeId, "disable"], `http://localhost/api/workbench/stores/${storeId}/disable`, { "If-Match": '"2"' }, undefined],
    ["resume", "POST", ["stores", storeId, "resume"], `http://localhost/api/workbench/stores/${storeId}/resume`, { "If-Match": '"2"' }, undefined],
  ] as const)("binds %s to match/mismatch/no-cookie Organization assertions", async (_name, method, path, url, routeHeaders, body) => {
    for (const [mode, headers] of [
      ["match", { cookie: `${WORKBENCH_COOKIE_NAME}=org-cookie`, "X-Expected-Organization-ID": "org-cookie", ...routeHeaders }],
      ["mismatch", { cookie: `${WORKBENCH_COOKIE_NAME}=org-cookie`, "X-Expected-Organization-ID": "org-other", ...routeHeaders }],
      ["no-cookie", { "X-Expected-Organization-ID": "org-cookie", ...routeHeaders }],
    ] as const) {
      const request = new Request(url, { method, headers, body });
      const result = await buildWorkbenchUpstreamRequest(request, [...path], "token");
      if (mode === "match") {
        expect(result).not.toBeInstanceOf(Response);
        if (!(result instanceof Response)) {
          expect(new Headers(result.init.headers).get("X-Requested-Organization-ID")).toBe("org-cookie");
          expect(new Headers(result.init.headers).get("X-Expected-Organization-ID")).toBeNull();
        }
      } else {
        expect(result).toBeInstanceOf(Response);
        expect((result as Response).status).toBe(409);
        expect(request.bodyUsed).toBe(false);
      }
    }
  });

  it("rejects actual duplicate and differently-cased expected Organization headers", async () => {
    const headers = new Headers([
      ["cookie", `${WORKBENCH_COOKIE_NAME}=org-cookie`],
      ["X-Expected-Organization-ID", "org-cookie"],
      ["x-expected-organization-id", "org-cookie"],
    ]);
    const result = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/stores?page=1", { method: "GET", headers }),
      ["stores"],
      "token",
    );
    expect(result).toBeInstanceOf(Response);
    expect((result as Response).status).toBe(409);
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.useRealTimers();
  });

  it("uses the trusted selection cookie and strips every browser-supplied identity header", async () => {
    vi.stubEnv(
      "LISTINGKIT_SERVICE_API_BASE",
      "https://service.example/api/v1/",
    );
    const request = new Request(
      "https://browser.example/api/workbench/context?redirect=https://attacker.example",
      {
        headers: {
          accept: "text/html",
          authorization: "Bearer browser-token",
          cookie: `${WORKBENCH_COOKIE_NAME}=org-cookie; other=value`,
          host: "attacker.example",
          "x-forwarded-host": "attacker.example",
          "x-requested-organization-id": "org-forged",
          "x-tenant-id": "tenant-forged",
          "x-user-id": "user-forged",
          "x-user-roles": "admin",
        },
      },
    );

    const result = await buildWorkbenchUpstreamRequest(
      request,
      ["context"],
      "server-token",
    );

    expect(result).not.toBeInstanceOf(Response);
    if (result instanceof Response) return;
    expect(result.url).toBe("https://service.example/api/v1/workbench/context");
    expect(result.init.method).toBe("GET");
    expect(result.init.body).toBeUndefined();
    const headers = new Headers(result.init.headers);
    expect(headers.get("X-Request-ID")).toMatch(/^[A-Za-z0-9][A-Za-z0-9._:/-]*$/);
    headers.delete("X-Request-ID");
    expect(Object.fromEntries(headers.entries())).toEqual({
      accept: "application/json",
      authorization: "Bearer server-token",
      "x-requested-organization-id": "org-cookie",
    });
  });

  it.each([
    ["preserves", "request-123/worker", "request-123/worker"],
    ["trims", " request-123 ", "request-123"],
    ["replaces unsafe", "request id", null],
    ["replaces oversized", "r" + "x".repeat(128), null],
  ])("%s a bounded request ID for the upstream", async (_name, incoming, expected) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/context", {
        headers: { "X-Request-ID": incoming },
      }),
      ["context"],
      "server-token",
    );
    expect(result).not.toBeInstanceOf(Response);
    if (result instanceof Response) return;
    const requestId = new Headers(result.init.headers).get("X-Request-ID");
    expect(requestId).toBeTruthy();
    if (expected) expect(requestId).toBe(expected);
    else expect(requestId).not.toBe(incoming);
    expect(new TextEncoder().encode(requestId ?? "").byteLength).toBeLessThanOrEqual(128);
  });

  it("canonicalizes the validated PUT body and uses the same organization for its trusted header", async () => {
    const request = new Request(
      "http://localhost/api/workbench/context/effective-organization",
      {
        method: "PUT",
        headers: {
          "content-type": "application/json",
          "x-requested-organization-id": "org-forged",
        },
        body: '{ "organizationId" : "  org-body  " }',
      },
    );

    const result = await buildWorkbenchUpstreamRequest(
      request,
      ["context", "effective-organization"],
      "server-token",
    );

    expect(result).not.toBeInstanceOf(Response);
    if (result instanceof Response) return;
    expect(result.url).toBe(
      "http://localhost:8085/api/v1/workbench/context/effective-organization",
    );
    expect(result.init.body).toBe('{"organizationId":"org-body"}');
    const headers = new Headers(result.init.headers);
    expect(headers.get("Content-Type")).toBe("application/json");
    expect(headers.get("X-Requested-Organization-ID")).toBe("org-body");
  });

  it.each([
    ["unknown field", '{"organizationId":"org-a","extra":true}'],
    ["case-variant field", '{"OrganizationId":"org-a"}'],
    ["duplicate field", '{"organizationId":"org-a","organizationId":"org-b"}'],
    ["blank field", '{"organizationId":"   "}'],
    ["trailing JSON", '{"organizationId":"org-a"} {}'],
  ])("rejects a PUT body with a %s", async (_name, body) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request(
        "http://localhost/api/workbench/context/effective-organization",
        { method: "PUT", body },
      ),
      ["context", "effective-organization"],
      "server-token",
    );

    expect(result).toBeInstanceOf(Response);
    if (!(result instanceof Response)) return;
    expect(result.status).toBe(400);
    await expect(result.json()).resolves.toMatchObject({
      code: "INVALID_REQUEST",
    });
  });

  it.each([
    ["non-breaking space", '\u00a0{"organizationId":"org-a"}'],
    ["line separator", '\u2028{"organizationId":"org-a"}'],
  ])("rejects JSON surrounded by %s", async (_name, body) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request(
        "http://localhost/api/workbench/context/effective-organization",
        { method: "PUT", body },
      ),
      ["context", "effective-organization"],
      "server-token",
    );

    expect(result).toBeInstanceOf(Response);
    if (!(result instanceof Response)) return;
    expect(result.status).toBe(400);
    await expect(result.json()).resolves.toMatchObject({
      code: "INVALID_REQUEST",
    });
  });

  it.each([
    ["carriage return", '{"organizationId":"org\\u000dvalue"}'],
    ["NUL", '{"organizationId":"org\\u0000value"}'],
    [
      "overlong value",
      JSON.stringify({ organizationId: `o${"r".repeat(128)}` }),
    ],
  ])(
    "returns stable JSON for a %s in the PUT organization ID",
    async (_name, body) => {
      const result = await buildWorkbenchUpstreamRequest(
        new Request(
          "http://localhost/api/workbench/context/effective-organization",
          { method: "PUT", body },
        ),
        ["context", "effective-organization"],
        "server-token",
      );

      expect(result).toBeInstanceOf(Response);
      if (!(result instanceof Response)) return;
      expect(result.status).toBe(400);
      expect((await result.text()).length).toBeLessThan(256);
    },
  );

  it("returns stable JSON for an unsafe percent-decoded selection cookie", async () => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/context", {
        headers: {
          cookie: `${WORKBENCH_COOKIE_NAME}=org%0Dvalue`,
        },
      }),
      ["context"],
      "server-token",
    );

    expect(result).toBeInstanceOf(Response);
    if (!(result instanceof Response)) return;
    expect(result.status).toBe(400);
    await expect(result.json()).resolves.toMatchObject({
      code: "INVALID_REQUEST",
    });
  });

  it("returns stable JSON for a malformed percent-encoded selection cookie", async () => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/context", {
        headers: {
          cookie: `${WORKBENCH_COOKIE_NAME}=org%ZZvalue`,
        },
      }),
      ["context"],
      "server-token",
    );

    expect(result).toBeInstanceOf(Response);
    if (!(result instanceof Response)) return;
    expect(result.status).toBe(400);
    await expect(result.json()).resolves.toMatchObject({
      code: "INVALID_REQUEST",
    });
  });

  it("rejects a client body larger than 4 KiB before constructing an upstream request", async () => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request(
        "http://localhost/api/workbench/context/effective-organization",
        {
          method: "PUT",
          body: JSON.stringify({ organizationId: "x".repeat(4097) }),
        },
      ),
      ["context", "effective-organization"],
      "server-token",
    );

    expect(result).toBeInstanceOf(Response);
    if (!(result instanceof Response)) return;
    expect(result.status).toBe(413);
  });

  it("cancels a client body rejected by its oversized Content-Length", async () => {
    const cancel = vi.fn();
    const result = await buildWorkbenchUpstreamRequest(
      {
        method: "PUT",
        headers: new Headers({ "content-length": "4097" }),
        body: new ReadableStream<Uint8Array>({ cancel }),
      } as Request,
      ["context", "effective-organization"],
      "server-token",
    );

    expect(result).toBeInstanceOf(Response);
    if (!(result instanceof Response)) return;
    expect(result.status).toBe(413);
    expect(cancel).toHaveBeenCalledOnce();
  });

  it("times out and cancels a client body that never finishes", async () => {
    vi.useFakeTimers();
    const cancel = vi.fn();
    let streamController:
      ReadableStreamDefaultController<Uint8Array> | undefined;
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        streamController = controller;
        controller.enqueue(
          new TextEncoder().encode('{"organizationId":"org-a"}'),
        );
      },
      cancel,
    });
    const pending = buildWorkbenchUpstreamRequest(
      {
        method: "PUT",
        headers: new Headers(),
        body,
      } as Request,
      ["context", "effective-organization"],
      "server-token",
    );
    let result: Awaited<typeof pending> | undefined;
    void pending.then((value) => {
      result = value;
    });

    await vi.advanceTimersByTimeAsync(15_000);
    await Promise.resolve();
    try {
      expect(result).toBeInstanceOf(Response);
      if (!(result instanceof Response)) return;
      expect(result.status).toBe(408);
      await expect(result.json()).resolves.toMatchObject({
        code: "INVALID_REQUEST",
      });
      expect(cancel).toHaveBeenCalledOnce();
    } finally {
      if (!result) {
        streamController?.close();
        await pending;
      }
    }
  });

  it("returns at the body deadline even when stream cancellation cleanup stalls", async () => {
    vi.useFakeTimers();
    const cancel = vi.fn(() => new Promise<void>(() => undefined));
    const body = new ReadableStream<Uint8Array>({ cancel });
    const pending = buildWorkbenchUpstreamRequest(
      {
        method: "PUT",
        headers: new Headers(),
        body,
      } as Request,
      ["context", "effective-organization"],
      "server-token",
    );
    let result: Awaited<typeof pending> | undefined;
    void pending.then((value) => {
      result = value;
    });

    await vi.advanceTimersByTimeAsync(15_000);
    await Promise.resolve();

    expect(cancel).toHaveBeenCalledOnce();
    expect(result).toBeInstanceOf(Response);
  });

  it.each([
    ["GET", ["context", "effective-organization"]],
    ["PUT", ["context"]],
    ["GET", ["context", ".."]],
    ["GET", ["https:%2F%2Fattacker.example"]],
  ])("rejects non-allowlisted %s path %j", async (method, path) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/anything", { method }),
      path,
      "server-token",
    );

    expect(result).toBeInstanceOf(Response);
    if (!(result instanceof Response)) return;
    expect(result.status).toBe(404);
  });
});

describe("buildWorkbenchBrowserResponse", () => {
  it("preserves a validated error status and body while forcing safe response headers", async () => {
    const payload = {
      code: "ORGANIZATION_SUSPENDED",
      message: "Organization is suspended",
      requestId: "req-body",
      fieldErrors: [],
    };
    const upstream = Response.json(payload, {
      status: 423,
      headers: {
        "cache-control": "no-store",
        etag: '"context-1"',
        location: "https://attacker.example",
        "set-cookie": "upstream-secret=value",
        "x-request-id": "req-header",
        connection: "close",
      },
    });

    const response = await buildWorkbenchBrowserResponse(upstream);

    expect(response.status).toBe(423);
    await expect(response.json()).resolves.toEqual(payload);
    expect(response.headers.get("Content-Type")).toBe("application/json");
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    expect(response.headers.get("X-Content-Type-Options")).toBe("nosniff");
    expect(response.headers.get("ETag")).toBeNull();
    expect(response.headers.get("X-Request-ID")).toBeNull();
    expect(response.headers.get("Location")).toBeNull();
    expect(response.headers.get("Connection")).toBeNull();
    expect(response.headers.get("Set-Cookie")).toBeNull();
  });

  it.each([
    ["partial context", { effectiveOrganizationId: "org-b" }],
    [
      "sole-Organization context without its required effective selection",
      {
        ...contextPayload,
        effectiveOrganizationId: null,
        selectionRequired: false,
      },
    ],
    ["unknown success field", { ...contextPayload, accessToken: "unsafe" }],
    [
      "duplicate organization",
      {
        ...contextPayload,
        organizations: [
          contextPayload.organizations[0],
          contextPayload.organizations[0],
        ],
      },
    ],
  ])(
    "maps an invalid %s success envelope to stable 502 JSON",
    async (_name, payload) => {
      const response = await buildWorkbenchBrowserResponse(
        Response.json(payload),
      );

      expect(response.status).toBe(502);
      expect(response.headers.get("Content-Type")).toBe("application/json");
      expect(response.headers.get("Cache-Control")).toBe("private, no-store");
      expect(response.headers.get("X-Content-Type-Options")).toBe("nosniff");
      await expect(response.json()).resolves.toEqual({
        code: "DEPENDENCY_UNAVAILABLE",
        message: "Workbench upstream response is invalid",
        requestId: "",
        fieldErrors: [],
      });
    },
  );

  it.each([
    [
      "unknown error field",
      {
        code: "PERMISSION_DENIED",
        message: "Denied",
        requestId: "req-1",
        fieldErrors: [],
        token: "unsafe",
      },
    ],
    [
      "malformed field errors",
      {
        code: "PERMISSION_DENIED",
        message: "Denied",
        requestId: "req-1",
        fieldErrors: ["unsafe"],
      },
    ],
  ])(
    "maps an invalid %s envelope to stable 502 JSON",
    async (_name, payload) => {
      const response = await buildWorkbenchBrowserResponse(
        Response.json(payload, { status: 403 }),
      );

      expect(response.status).toBe(502);
      await expect(response.json()).resolves.toMatchObject({
        code: "DEPENDENCY_UNAVAILABLE",
      });
    },
  );

  it.each([
    [
      "duplicate context success key",
      new Response(
        '{"user":{"id":"user-1"},"homeOrganizationId":"org-a","effectiveOrganizationId":"org-a","effectiveOrganizationId":"org-b","selectionRequired":false,"organizations":[{"id":"org-a","name":"Organization A","roles":[]},{"id":"org-b","name":"Organization B","roles":[]}]}',
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    ],
    [
      "duplicate context error key",
      new Response(
        '{"code":"ORGANIZATION_ACCESS_DENIED","code":"DEPENDENCY_UNAVAILABLE","message":"Denied","requestId":"req-duplicate","fieldErrors":[]}',
        { status: 403, headers: { "Content-Type": "application/json" } },
      ),
    ],
  ])(
    "rejects %s before accepting the parsed payload",
    async (_name, upstream) => {
      const response = await buildWorkbenchBrowserResponse(upstream);

      expect(response.status).toBe(502);
      await expect(response.json()).resolves.toMatchObject({
        code: "DEPENDENCY_UNAVAILABLE",
      });
    },
  );

  it.each([
    [
      "success status carrying an error",
      Response.json({
        code: "PERMISSION_DENIED",
        message: "Denied",
        requestId: "",
        fieldErrors: [],
      }),
    ],
    [
      "error status carrying a context",
      Response.json(contextPayload, { status: 403 }),
    ],
    [
      "unexpected success status",
      Response.json(contextPayload, { status: 201 }),
    ],
  ])("rejects a body/status mismatch: %s", async (_name, upstream) => {
    const response = await buildWorkbenchBrowserResponse(upstream);
    expect(response.status).toBe(502);
    await expect(response.json()).resolves.toMatchObject({
      code: "DEPENDENCY_UNAVAILABLE",
    });
  });

  it("rejects invalid UTF-8 before JSON parsing", async () => {
    const response = await buildWorkbenchBrowserResponse(
      new Response(new Uint8Array([0xff, 0xfe]), { status: 200 }),
    );

    expect(response.status).toBe(502);
    await expect(response.json()).resolves.toMatchObject({
      code: "DEPENDENCY_UNAVAILABLE",
    });
  });

  it("sets the HttpOnly session cookie from Go's successful effective organization", async () => {
    vi.stubEnv("NODE_ENV", "production");
    const response = await buildWorkbenchBrowserResponse(
      Response.json(contextPayload),
    );

    const cookie = response.headers.get("Set-Cookie");
    expect(cookie).toContain(`${WORKBENCH_COOKIE_NAME}=org-b`);
    expect(cookie).toContain("Path=/");
    expect(cookie).toContain("HttpOnly");
    expect(cookie).toContain("Secure");
    expect(cookie).toContain("SameSite=lax");
    expect(cookie).not.toContain("Domain=");
  });

  it("does not change the cookie after an ordinary failed switch", async () => {
    const response = await buildWorkbenchBrowserResponse(
      Response.json(
        {
          code: "ORGANIZATION_SUSPENDED",
          message: "Organization is suspended",
          requestId: "req-1",
          fieldErrors: [],
        },
        { status: 403 },
      ),
      "context-switch",
    );

    expect(response.headers.get("Set-Cookie")).toBeNull();
  });

  it.each(["ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED"])(
    "clears a stale selection when Go returns %s",
    async (code) => {
      const response = await buildWorkbenchBrowserResponse(
        Response.json(
          { code, message: "Access lost", requestId: "req-1", fieldErrors: [] },
          { status: 403 },
        ),
      );

      expect(response.headers.get("Set-Cookie")).toContain(
        `${WORKBENCH_COOKIE_NAME}=;`,
      );
      expect(response.headers.get("Set-Cookie")).toContain("Max-Age=0");
      expect(response.headers.get("Set-Cookie")).toContain("HttpOnly");
    },
  );

  it("clears a stale selection when a zero-Organization context has no effective organization", async () => {
    const response = await buildWorkbenchBrowserResponse(
      Response.json({
        ...contextPayload,
        effectiveOrganizationId: null,
        organizations: [],
      }),
    );

    expect(response.headers.get("Set-Cookie")).toContain(
      `${WORKBENCH_COOKIE_NAME}=;`,
    );
    expect(response.headers.get("Set-Cookie")).toContain("Max-Age=0");
  });

  it("maps a response larger than 1 MiB to a bounded 502 error", async () => {
    const upstream = new Response("x".repeat(1024 * 1024 + 1), {
      status: 200,
      headers: { "content-type": "application/json" },
    });

    const response = await buildWorkbenchBrowserResponse(upstream);
    const text = await response.text();

    expect(response.status).toBe(502);
    expect(text.length).toBeLessThan(1024);
    expect(JSON.parse(text)).toMatchObject({ code: "DEPENDENCY_UNAVAILABLE" });
  });

  it("cancels an upstream body rejected by its oversized Content-Length", async () => {
    const cancel = vi.fn();
    const upstream = new Response(new ReadableStream<Uint8Array>({ cancel }), {
      status: 200,
      headers: {
        "content-length": String(1024 * 1024 + 1),
        "content-type": "application/json",
      },
    });

    const response = await buildWorkbenchBrowserResponse(upstream);

    expect(response.status).toBe(502);
    expect(cancel).toHaveBeenCalledOnce();
  });

  it("maps an unsafe effective organization from upstream to a bounded 502", async () => {
    const response = await buildWorkbenchBrowserResponse(
      Response.json({
        ...contextPayload,
        effectiveOrganizationId: "org\rvalue",
      }),
    );
    const text = await response.text();

    expect(response.status).toBe(502);
    expect(response.headers.get("Set-Cookie")).toBeNull();
    expect(text.length).toBeLessThan(256);
    expect(JSON.parse(text)).toMatchObject({ code: "DEPENDENCY_UNAVAILABLE" });
  });
});

describe("strict Store Center request boundary", () => {
  const storeHeaders = (headers: HeadersInit = {}) => ({
    cookie: `${WORKBENCH_COOKIE_NAME}=org-cookie`,
    "X-Expected-Organization-ID": "org-cookie",
    ...headers,
  });
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it.each([
    {
      name: "list",
      method: "GET",
      path: ["stores"],
      url: "http://localhost/api/workbench/stores?page=2&pageSize=50&platform=shein&status=active",
      headers: {},
      body: undefined,
      wantURL:
        "http://localhost:8085/api/v1/workbench/stores?page=2&pageSize=50&platform=shein&status=active",
      contract: "store-list",
    },
    {
      name: "create",
      method: "POST",
      path: ["stores"],
      url: "http://localhost/api/workbench/stores",
      headers: { "Idempotency-Key": operationKey },
      body: JSON.stringify({
        name: " Store ",
        platform: "shein",
        region: " SG ",
        externalStoreId: " external-1 ",
      }),
      wantURL: "http://localhost:8085/api/v1/workbench/stores",
      contract: "store-create",
    },
    {
      name: "get",
      method: "GET",
      path: ["stores", storeId],
      url: `http://localhost/api/workbench/stores/${storeId}`,
      headers: {},
      body: undefined,
      wantURL: `http://localhost:8085/api/v1/workbench/stores/${storeId}`,
      contract: "store-item",
    },
    {
      name: "update",
      method: "PUT",
      path: ["stores", storeId],
      url: `http://localhost/api/workbench/stores/${storeId}`,
      headers: { "If-Match": '"2"' },
      body: JSON.stringify({ name: " Store ", region: " SG " }),
      wantURL: `http://localhost:8085/api/v1/workbench/stores/${storeId}`,
      contract: "store-item",
    },
    {
      name: "delete",
      method: "DELETE",
      path: ["stores", storeId],
      url: `http://localhost/api/workbench/stores/${storeId}`,
      headers: {
        "Idempotency-Key": operationKey,
        "If-Match": '"2"',
      },
      body: undefined,
      wantURL: `http://localhost:8085/api/v1/workbench/stores/${storeId}`,
      contract: "store-delete",
    },
    ...["enable", "disable"].map((action) => ({
      name: action,
      method: "POST",
      path: ["stores", storeId, action],
      url: `http://localhost/api/workbench/stores/${storeId}/${action}`,
      headers: { "If-Match": '"2"' },
      body: undefined,
      wantURL: `http://localhost:8085/api/v1/workbench/stores/${storeId}/${action}`,
      contract: "store-item",
    })),
  ])("maps the exact $name route through one typed descriptor", async (testCase) => {
    const requestHeaders = new Headers({
      cookie: `${WORKBENCH_COOKIE_NAME}=org-cookie`,
      "X-Expected-Organization-ID": "org-cookie",
      authorization: "Bearer browser-secret",
      "x-requested-organization-id": "org-forged",
      "x-tenant-id": "tenant-forged",
      "x-subject-id": "subject-forged",
      "x-forwarded-for": "127.0.0.1",
    });
    for (const [name, value] of Object.entries(testCase.headers)) {
      if (value !== undefined) requestHeaders.set(name, value);
    }
    const request = new Request(testCase.url, {
      method: testCase.method,
      headers: requestHeaders,
      body: testCase.body,
    });

    const result = await buildWorkbenchUpstreamRequest(
      request,
      testCase.path,
      "server-token",
    );

    expect(result).not.toBeInstanceOf(Response);
    if (result instanceof Response) return;
    expect(result.url).toBe(testCase.wantURL);
    expect(result.init.method).toBe(testCase.method);
    expect(result.responseContract).toBe(testCase.contract);
    const headers = new Headers(result.init.headers);
    expect(headers.get("Authorization")).toBe("Bearer server-token");
    expect(headers.get("X-Requested-Organization-ID")).toBe("org-cookie");
    expect(headers.get("X-Expected-Organization-ID")).toBeNull();
    expect(headers.get("Cookie")).toBeNull();
    expect(headers.get("X-Tenant-ID")).toBeNull();
    expect(headers.get("X-Subject-ID")).toBeNull();
    expect(headers.get("X-Forwarded-For")).toBeNull();
    expect(headers.get("Idempotency-Key")).toBe(
      testCase.name === "create" || testCase.name === "delete"
        ? operationKey
        : null,
    );
    expect(headers.get("If-Match")).toBe(
      ["update", "delete", "enable", "disable"].includes(testCase.name)
        ? '"2"'
        : null,
    );
    if (testCase.name === "create") {
      expect(result.init.body).toBe(
        JSON.stringify({
          name: "Store",
          platform: "shein",
          region: "SG",
          externalStoreId: "external-1",
        }),
      );
    }
    if (testCase.name === "update") {
      expect(result.init.body).toBe(
        JSON.stringify({ name: "Store", region: "SG" }),
      );
    }
  });

  it.each([
    ["uppercase UUID", ["stores", storeId.toUpperCase()]],
    ["nil UUID", ["stores", "00000000-0000-0000-0000-000000000000"]],
    ["invalid version", ["stores", "11111111-1111-6111-8111-111111111111"]],
    ["invalid variant", ["stores", "11111111-1111-4111-7111-111111111111"]],
    ["encoded slash", ["stores", "%2f"]],
    ["decoded slash", ["stores", `prefix/${storeId}`]],
    ["decoded backslash", ["stores", `prefix\\${storeId}`]],
    ["dot segment", ["stores", ".."]],
    ["empty segment", ["stores", ""]],
    ["trailing segment", ["stores", storeId, "extra"]],
    ["repeated stores segment", ["stores", "stores", storeId]],
  ])("rejects the %s path before fetch", async (_name, path) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/stores", { method: "GET" }),
      path,
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(404);
  });

  it("rejects an alternate percent-encoding of an otherwise canonical Store UUID", async () => {
    const encodedStoreId = `%31${storeId.slice(1)}`;
    const result = await buildWorkbenchUpstreamRequest(
      new Request(`http://localhost/api/workbench/stores/${encodedStoreId}`, {
        method: "GET", headers: storeHeaders(),
      }),
      ["stores", storeId],
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(404);
  });

  it.each([
    ["PUT collection", "PUT", ["stores"]],
    ["DELETE collection", "DELETE", ["stores"]],
    ["POST item", "POST", ["stores", storeId]],
    ["GET action", "GET", ["stores", storeId, "enable"]],
    ["PUT action", "PUT", ["stores", storeId, "disable"]],
    ["unknown action", "POST", ["stores", storeId, "archive"]],
  ])("rejects non-allowlisted Store method/path: %s", async (_name, method, path) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/stores", { method }),
      path,
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(404);
  });

  it.each([
    "page=0",
    "page=01",
    "page=+1",
    "page=9007199254740992",
    "pageSize=0",
    "pageSize=101",
    "pageSize=01",
    "platform=amazon",
    "status=unknown",
    "page=1&page=2",
    "organizationId=org-forged",
    "organization_id=org-forged",
    "tenantId=tenant-forged",
    "unknown=value",
  ])("rejects invalid Store list query %s", async (query) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request(`http://localhost/api/workbench/stores?${query}`, {
        method: "GET",
        headers: storeHeaders(),
      }),
      ["stores"],
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(400);
  });

  it("rejects a rebuilt Store query larger than 2 KiB", async () => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request(
        `http://localhost/api/workbench/stores?platform=${"s".repeat(2100)}`,
        { method: "GET", headers: storeHeaders() },
      ),
      ["stores"],
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(400);
  });

  it.each([
    ["create", "POST", ["stores"], { "Idempotency-Key": operationKey }, JSON.stringify({ name: "A", platform: "shein", region: "SG" })],
    ["item", "GET", ["stores", storeId], {}, undefined],
    ["update", "PUT", ["stores", storeId], { "If-Match": '"2"' }, JSON.stringify({ name: "A", region: "SG" })],
    ["delete", "DELETE", ["stores", storeId], { "Idempotency-Key": operationKey, "If-Match": '"2"' }, undefined],
    ["action", "POST", ["stores", storeId, "enable"], { "If-Match": '"2"' }, undefined],
  ] as const)("rejects query input on Store %s", async (_name, method, path, headers, body) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request(
        `http://localhost/api/workbench/${path.join("/")}?organizationId=org-forged`,
        { method, headers: storeHeaders(headers), body },
      ),
      [...path],
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(400);
  });

  it.each([
    ["duplicate create key", `{"name":"A","name":"B","platform":"shein","region":"SG"}`],
    ["unknown create key", `{"name":"A","platform":"shein","region":"SG","organizationId":"org-forged"}`],
    ["case variant", `{"Name":"A","platform":"shein","region":"SG"}`],
    ["missing required", `{"name":"A","platform":"shein"}`],
    ["non-string", `{"name":1,"platform":"shein","region":"SG"}`],
    ["trailing JSON", `{"name":"A","platform":"shein","region":"SG"}{}`],
    ["comment", `{"name":"A","platform":"shein","region":"SG"/*x*/}`],
    ["trailing comma", `{"name":"A","platform":"shein","region":"SG",}`],
    ["blank", `{"name":"  ","platform":"shein","region":"SG"}`],
    ["control", `{"name":"A\\u0000","platform":"shein","region":"SG"}`],
    ["name too long", JSON.stringify({ name: "界".repeat(121), platform: "shein", region: "SG" })],
    ["region too long", JSON.stringify({ name: "A", platform: "shein", region: "界".repeat(65) })],
    ["external id too long", JSON.stringify({ name: "A", platform: "shein", region: "SG", externalStoreId: "界".repeat(129) })],
  ])("rejects invalid create body: %s", async (_name, body) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/stores", {
        method: "POST",
        headers: storeHeaders({ "Idempotency-Key": operationKey }),
        body,
      }),
      ["stores"],
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(400);
  });

  it.each([
    ["duplicate key", `{"name":"A","name":"B","region":"SG"}`],
    ["unknown key", `{"name":"A","region":"SG","platform":"shein"}`],
    ["missing key", `{"name":"A"}`],
    ["non-string", `{"name":"A","region":1}`],
    ["blank", `{"name":"A","region":"  "}`],
  ])("rejects invalid update body: %s", async (_name, body) => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request(`http://localhost/api/workbench/stores/${storeId}`, {
        method: "PUT",
        headers: storeHeaders({ "If-Match": '"2"' }),
        body,
      }),
      ["stores", storeId],
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(400);
  });

  it("rejects invalid UTF-8 and oversized Store bodies", async () => {
    const invalidUTF8 = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/stores", {
        method: "POST",
        headers: storeHeaders({ "Idempotency-Key": operationKey }),
        body: new Uint8Array([0xff]),
      }),
      ["stores"],
      "server-token",
    );
    expect(invalidUTF8).toBeInstanceOf(Response);
    if (invalidUTF8 instanceof Response) expect(invalidUTF8.status).toBe(400);

    const oversized = await buildWorkbenchUpstreamRequest(
      new Request("http://localhost/api/workbench/stores", {
        method: "POST",
        headers: storeHeaders({ "Idempotency-Key": operationKey }),
        body: "x".repeat(16 * 1024 + 1),
      }),
      ["stores"],
      "server-token",
    );
    expect(oversized).toBeInstanceOf(Response);
    if (oversized instanceof Response) expect(oversized.status).toBe(413);
  });

  it.each([
    ["create missing idempotency", "POST", ["stores"], {}],
    ["create malformed idempotency", "POST", ["stores"], { "Idempotency-Key": storeId.toUpperCase() }],
    ["create repeated idempotency", "POST", ["stores"], { "Idempotency-Key": `${operationKey}, ${operationKey}` }],
    ["create nil idempotency", "POST", ["stores"], { "Idempotency-Key": "00000000-0000-0000-0000-000000000000" }],
    ["update missing If-Match", "PUT", ["stores", storeId], {}],
    ["update weak If-Match", "PUT", ["stores", storeId], { "If-Match": 'W/"2"' }],
    ["update list If-Match", "PUT", ["stores", storeId], { "If-Match": '"2", "3"' }],
    ["update leading-zero If-Match", "PUT", ["stores", storeId], { "If-Match": '"02"' }],
    ["update unsafe If-Match", "PUT", ["stores", storeId], { "If-Match": '"9007199254740992"' }],
    ["delete missing idempotency", "DELETE", ["stores", storeId], { "If-Match": '"2"' }],
    ["delete missing If-Match", "DELETE", ["stores", storeId], { "Idempotency-Key": operationKey }],
  ])("rejects required Store header contract: %s", async (_name, method, path, headers) => {
    const body = method === "POST" ? JSON.stringify({ name: "A", platform: "shein", region: "SG" }) : method === "PUT" ? JSON.stringify({ name: "A", region: "SG" }) : undefined;
    const result = await buildWorkbenchUpstreamRequest(
      new Request(`http://localhost/api/workbench/${path.join("/")}`, {
        method,
        headers: storeHeaders(headers),
        body,
      }),
      path as string[],
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(400);
  });

  it.each(["enable", "disable"])(
    "rejects any body on the %s action",
    async (action) => {
      const result = await buildWorkbenchUpstreamRequest(
        new Request(`http://localhost/api/workbench/stores/${storeId}/${action}`, {
          method: "POST",
          headers: storeHeaders({ "If-Match": '"2"' }),
          body: "{}",
        }),
        ["stores", storeId, action],
        "server-token",
      );
      expect(result).toBeInstanceOf(Response);
      if (result instanceof Response) expect(result.status).toBe(400);
    },
  );

  it("rejects any body on delete", async () => {
    const result = await buildWorkbenchUpstreamRequest(
      new Request(`http://localhost/api/workbench/stores/${storeId}`, {
        method: "DELETE",
        headers: storeHeaders({
          "Idempotency-Key": operationKey,
          "If-Match": '"2"',
        }),
        body: "x",
      }),
      ["stores", storeId],
      "server-token",
    );
    expect(result).toBeInstanceOf(Response);
    if (result instanceof Response) expect(result.status).toBe(400);
  });
});

describe("strict Store Center response boundary", () => {
  const rawStore = JSON.stringify(storePayload);
  const rawList = JSON.stringify({
    items: [storePayload],
    quota: { used: 1, reserved: 1, limit: 5, allowed: true, reason: "" },
    pagination: { page: 1, pageSize: 20, total: 1 },
  });

  it.each(["array", "object"] as const)(
    "maps a Store success DTO with a 4000-level unknown %s to bounded 502 without rejecting",
    async (kind) => {
      const depth = 4_000;
      const nested =
        kind === "array"
          ? `${"[".repeat(depth)}0${"]".repeat(depth)}`
          : `${'{"value":'.repeat(depth)}0${"}".repeat(depth)}`;
      const raw = `${rawStore.slice(0, -1)},"unknown":${nested}}`;

      const response = await buildWorkbenchBrowserResponse(
        new Response(raw),
        "store-item",
      );
      const text = await response.text();

      expect(response.status).toBe(502);
      expect(text.length).toBeLessThan(1024);
      expect(JSON.parse(text)).toMatchObject({
        code: "DEPENDENCY_UNAVAILABLE",
      });
    },
  );

  it.each([
    [
      "Store item version",
      "store-item",
      200,
      rawStore.replace('"version":2', '"version":2.00000000000000001'),
    ],
    [
      "duplicate Store item version using a lossy last token",
      "store-item",
      200,
      rawStore.replace(
        '"version":2',
        '"version":2,"version":2.00000000000000001',
      ),
    ],
    [
      "escaped duplicate Store item version",
      "store-item",
      200,
      rawStore.replace(
        '"version":2',
        '"version":2,"\\u0076ersion":2.00000000000000001',
      ),
    ],
    [
      "Store list item version",
      "store-list",
      200,
      rawList.replace('"version":2', '"version":2.00000000000000001'),
    ],
    [
      "delete version",
      "store-delete",
      200,
      `{"id":"${storeId}","deleted":true,"version":9007199254740991.1}`,
    ],
    [
      "quota used",
      "store-list",
      200,
      rawList.replace('"used":1', '"used":1.00000000000000001'),
    ],
    [
      "quota reserved",
      "store-list",
      200,
      rawList.replace('"reserved":1', '"reserved":1.00000000000000001'),
    ],
    [
      "non-null quota limit",
      "store-list",
      200,
      rawList.replace('"limit":5', '"limit":5.0000000000000001'),
    ],
    [
      "pagination page",
      "store-list",
      200,
      rawList.replace('"page":1', '"page":1.00000000000000001'),
    ],
    [
      "pagination pageSize",
      "store-list",
      200,
      rawList.replace('"pageSize":20', '"pageSize":20.000000000000001'),
    ],
    [
      "pagination total",
      "store-list",
      200,
      rawList.replace('"total":1', '"total":1.00000000000000001'),
    ],
  ] as const)(
    "rejects a lossy fractional JSON number token for %s",
    async (_name, contract, status, raw) => {
      const response = await buildWorkbenchBrowserResponse(
        new Response(raw, {
          status,
          headers: { "content-type": "application/json" },
        }),
        contract,
      );

      expect(response.status).toBe(502);
      await expect(response.json()).resolves.toMatchObject({
        code: "DEPENDENCY_UNAVAILABLE",
      });
    },
  );

  it("accepts canonical integer tokens, nullable quota limit, and numeric text in strings", async () => {
    const store = await buildWorkbenchBrowserResponse(
      new Response(
        rawStore
          .replace('"name":"Store"', '"name":"Store 2.00000000000000001"')
          .replace('"version":2', '"version":9007199254740991'),
      ),
      "store-item",
    );
    expect(store.status).toBe(200);
    await expect(store.json()).resolves.toMatchObject({
      name: "Store 2.00000000000000001",
      version: Number.MAX_SAFE_INTEGER,
    });

    const list = await buildWorkbenchBrowserResponse(
      new Response(
        JSON.stringify({
          items: [],
          quota: {
            used: 0,
            reserved: 0,
            limit: null,
            allowed: false,
            reason: "subscription_required",
          },
          pagination: { page: 1, pageSize: 20, total: 0 },
        }),
      ),
      "store-list",
    );
    expect(list.status).toBe(200);
    await expect(list.json()).resolves.toMatchObject({
      quota: { limit: null },
    });
  });

  it.each([
    [
      "exponent Store version",
      "store-item",
      rawStore.replace('"version":2', '"version":2e0'),
    ],
    [
      "negative-zero quota count",
      "store-list",
      rawList.replace('"used":1', '"used":-0'),
    ],
    [
      "decimal pagination integer",
      "store-list",
      rawList.replace('"page":1', '"page":1.0'),
    ],
    [
      "lossy version in a later Store list item",
      "store-list",
      JSON.stringify({
        items: [
          storePayload,
          {
            ...storePayload,
            id: "33333333-3333-4333-8333-33333333333c",
            version: 3,
          },
        ],
        quota: { used: 1, reserved: 1, limit: 5, allowed: true, reason: "" },
        pagination: { page: 1, pageSize: 20, total: 2 },
      }).replace('"version":3', '"version":3.00000000000000001'),
    ],
  ] as const)("rejects a noncanonical raw integer representation: %s", async (_name, contract, raw) => {
    const response = await buildWorkbenchBrowserResponse(
      new Response(raw),
      contract,
    );
    expect(response.status).toBe(502);
  });

  it.each([
    ["store-create", 201],
    ["store-item", 200],
  ] as const)("accepts the exact %s success DTO", async (contract, status) => {
    const response = await buildWorkbenchBrowserResponse(
      Response.json(storePayload, { status }),
      contract,
    );
    expect(response.status).toBe(status);
    await expect(response.json()).resolves.toEqual(storePayload);
    expect(response.headers.get("Set-Cookie")).toBeNull();
  });

  it("accepts a valid UTC RFC3339Nano Store timestamp", async () => {
    const payload = { ...storePayload, updatedAt: "2026-08-30T02:03:04.123456789Z" };
    const response = await buildWorkbenchBrowserResponse(
      Response.json(payload),
      "store-item",
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual(payload);
  });

  it("accepts the exact list and delete DTOs", async () => {
    const listPayload = {
      items: [storePayload],
      quota: { used: 1, reserved: 0, limit: 5, allowed: true, reason: "" },
      pagination: { page: 1, pageSize: 20, total: 1 },
    };
    const list = await buildWorkbenchBrowserResponse(
      Response.json(listPayload),
      "store-list",
    );
    expect(list.status).toBe(200);
    await expect(list.json()).resolves.toEqual(listPayload);

    const deletePayload = { id: storeId, deleted: true, version: 3 };
    const deleted = await buildWorkbenchBrowserResponse(
      Response.json(deletePayload),
      "store-delete",
    );
    expect(deleted.status).toBe(200);
    await expect(deleted.json()).resolves.toEqual(deletePayload);
  });

  it.each([
    ["unknown field", { ...storePayload, organizationId: "org-secret" }],
    ["credential field", { ...storePayload, token: "secret" }],
    ["connection reference", { ...storePayload, connectionRef: "private" }],
    ["nil UUID", { ...storePayload, id: "00000000-0000-0000-0000-000000000000" }],
    ["unsafe version", { ...storePayload, version: Number.MAX_SAFE_INTEGER + 1 }],
    ["invalid lifecycle", { ...storePayload, lifecycleStatus: "deleted" }],
    ["invalid connection", { ...storePayload, connectionStatus: "unknown" }],
    ["offset timestamp", { ...storePayload, createdAt: "2026-08-30T09:02:03+08:00" }],
    ["impossible timestamp", { ...storePayload, updatedAt: "2026-02-30T02:03:04Z" }],
  ])("maps invalid Store %s to bounded 502", async (_name, payload) => {
    const response = await buildWorkbenchBrowserResponse(
      Response.json(payload),
      "store-item",
    );
    expect(response.status).toBe(502);
    await expect(response.json()).resolves.toMatchObject({
      code: "DEPENDENCY_UNAVAILABLE",
    });
  });

  it.each([
    ["create wrong status", "store-create", 200, storePayload],
    ["item wrong status", "store-item", 201, storePayload],
    ["delete wrong status", "store-delete", 204, null],
    ["list wrong shape", "store-list", 200, storePayload],
    ["too many items", "store-list", 200, { items: Array.from({ length: 101 }, () => storePayload), quota: { used: 0, reserved: 0, limit: null, allowed: false, reason: "subscription_required" }, pagination: { page: 1, pageSize: 100, total: 101 } }],
    ["negative count", "store-list", 200, { items: [], quota: { used: -1, reserved: 0, limit: 5, allowed: true, reason: "" }, pagination: { page: 1, pageSize: 20, total: 0 } }],
  ] as const)("rejects Store status/body mismatch: %s", async (_name, contract, status, payload) => {
    const response = await buildWorkbenchBrowserResponse(
      payload === null ? new Response(null, { status }) : Response.json(payload, { status }),
      contract,
    );
    expect(response.status).toBe(502);
  });

  it.each([
    ["invalid JSON", new Response("{"), "store-item"],
    ["invalid UTF-8", new Response(new Uint8Array([0xff]), { status: 200 }), "store-item"],
    ["success carrying error", Response.json({ code: "STORE_NOT_FOUND", message: "Missing", requestId: "", fieldErrors: [] }), "store-item"],
    ["error carrying success", Response.json(storePayload, { status: 404 }), "store-item"],
    ["redirect", new Response(null, { status: 302, headers: { location: "https://attacker.example" } }), "store-item"],
  ] as const)("rejects Store transport mismatch: %s", async (_name, upstream, contract) => {
    const response = await buildWorkbenchBrowserResponse(upstream, contract);
    expect(response.status).toBe(502);
    expect(response.headers.get("Location")).toBeNull();
  });

  it("preserves a strict Store error without mutating the selection cookie", async () => {
    const payload = {
      code: "STORE_VERSION_CONFLICT",
      message: "Store version conflict",
      requestId: "req-1",
      fieldErrors: [],
    };
    const response = await buildWorkbenchBrowserResponse(
      Response.json(payload, {
        status: 409,
        headers: {
          location: "https://attacker.example",
          "set-cookie": "secret=value",
          etag: '"3"',
        },
      }),
      "store-item",
    );
    expect(response.status).toBe(409);
    await expect(response.json()).resolves.toEqual(payload);
    expect(response.headers.get("Location")).toBeNull();
    expect(response.headers.get("Set-Cookie")).toBeNull();
    expect(response.headers.get("ETag")).toBeNull();
  });
});
