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

describe("buildWorkbenchUpstreamRequest", () => {
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
    expect(Object.fromEntries(headers.entries())).toEqual({
      accept: "application/json",
      authorization: "Bearer server-token",
      "x-requested-organization-id": "org-cookie",
    });
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

  it("clears a stale selection when a successful context has no effective organization", async () => {
    const response = await buildWorkbenchBrowserResponse(
      Response.json({ ...contextPayload, effectiveOrganizationId: null }),
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
