import { createServer, type RequestListener } from "node:http";
import type { AddressInfo } from "node:net";

import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

const authState = vi.hoisted(() => ({
  session: null as Record<string, unknown> | null,
  token: "",
}));

const authMocks = vi.hoisted(() => ({
  wrapper: vi.fn(
    (
      handler: (
        request: NextRequest & { auth?: unknown },
        context: unknown,
      ) => unknown,
    ) =>
      (request: NextRequest, context: unknown) =>
        handler(Object.assign(request, { auth: authState.session }), context),
  ),
  readToken: vi.fn((session: unknown) => {
    void session;
    return authState.token;
  }),
}));

vi.mock("@/auth", () => ({ serverAuth: authMocks.wrapper }));
vi.mock("@/lib/server/zitadel-server-token", () => ({
  readZitadelServerAccessToken: authMocks.readToken,
}));

import * as workbenchRoute from "@/app/api/workbench/[...path]/route";

const { GET, PUT, POST, DELETE } = workbenchRoute;

const storeId = "11111111-1111-4111-8111-11111111111a";
const operationKey = "22222222-2222-4222-8222-22222222222b";
const storePayload = {
  id: storeId,
  name: "Store",
  platform: "shein",
  region: "SG",
  externalStoreId: "",
  lifecycleStatus: "active",
  connectionStatus: "disconnected",
  version: 2,
  createdAt: "2026-08-30T01:02:03Z",
  updatedAt: "2026-08-30T02:03:04Z",
};

async function call(handler: typeof GET, request: NextRequest, path: string[]) {
  const response = await handler(request, {
    params: Promise.resolve({ path }),
  });
  if (!(response instanceof Response)) {
    throw new Error("Workbench route did not return a response");
  }
  return response;
}

async function listen(handler: RequestListener) {
  const server = createServer(handler);
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const { port } = server.address() as AddressInfo;
  return {
    url: `http://127.0.0.1:${port}`,
    close: () =>
      new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      ),
  };
}

describe("/api/workbench BFF", () => {
  afterEach(() => {
    authState.session = null;
    authState.token = "";
    vi.clearAllMocks();
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
    vi.useRealTimers();
  });

  it("uses the merged serverAuth wrapper and reads the token from request.auth", async () => {
    const session = { user: { id: "user-1" }, accessToken: "private-token" };
    authState.session = session;
    authState.token = "private-token";
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      Response.json({
        user: { id: "user-1" },
        homeOrganizationId: "org-a",
        effectiveOrganizationId: "org-a",
        selectionRequired: false,
        organizations: [],
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const response = await call(
      GET,
      new NextRequest("http://localhost/api/workbench/context", {
        headers: { authorization: "Bearer browser-token" },
      }),
      ["context"],
    );

    expect(authMocks.wrapper).toHaveBeenCalledOnce();
    expect(authMocks.readToken).toHaveBeenCalledWith(session);
    const headers = new Headers(fetchMock.mock.calls[0]?.[1]?.headers);
    expect(headers.get("Authorization")).toBe("Bearer private-token");
    const responseText = await response.text();
    expect(responseText).not.toContain("private-token");
    expect(response.headers.get("Authorization")).toBeNull();
  });

  it("returns bounded 401 JSON without an open redirect when the server session has no token", async () => {
    authState.session = { user: { id: "user-1" } };
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

    const response = await call(
      GET,
      new NextRequest(
        "http://localhost/api/workbench/context?redirect=https://attacker.example",
        { headers: { authorization: "Bearer browser-token" } },
      ),
      ["context"],
    );

    expect(response.status).toBe(401);
    expect(response.headers.get("Location")).toBeNull();
    await expect(response.json()).resolves.toEqual({
      code: "AUTHENTICATION_REQUIRED",
      message: "Authentication is required",
      requestId: "",
      fieldErrors: [],
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("does not follow an upstream redirect outside the fixed endpoint allowlist", async () => {
    authState.session = { accessToken: "private-token" };
    authState.token = "private-token";
    let redirectTargetHits = 0;
    const target = await listen((_request, response) => {
      redirectTargetHits += 1;
      response.setHeader("Content-Type", "application/json");
      response.end(JSON.stringify({ effectiveOrganizationId: "org-target" }));
    });
    const source = await listen((_request, response) => {
      response.writeHead(302, { Location: `${target.url}/not-allowlisted` });
      response.end();
    });
    vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", `${source.url}/api/v1`);

    try {
      const response = await call(
        GET,
        new NextRequest("http://localhost/api/workbench/context", {
          headers: { cookie: "shuomi_effective_organization=org-cookie" },
        }),
        ["context"],
      );

      expect(response.status).toBe(502);
      expect(response.headers.get("Location")).toBeNull();
      await expect(response.json()).resolves.toMatchObject({
        code: "DEPENDENCY_UNAVAILABLE",
      });
      expect(redirectTargetHits).toBe(0);
    } finally {
      await source.close();
      await target.close();
    }
  });

  it("aborts an upstream request at 15 seconds and returns bounded 502 JSON", async () => {
    vi.useFakeTimers();
    authState.session = { accessToken: "private-token" };
    authState.token = "private-token";
    const fetchMock = vi.fn<typeof fetch>((_input, init) =>
      new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener("abort", () =>
          reject(new DOMException("aborted", "AbortError")),
        );
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const pending = call(
      GET,
      new NextRequest("http://localhost/api/workbench/context"),
      ["context"],
    );
    await vi.advanceTimersByTimeAsync(15_000);
    const response = await pending;

    expect(response.status).toBe(502);
    await expect(response.json()).resolves.toEqual({
      code: "DEPENDENCY_UNAVAILABLE",
      message: "Workbench upstream is unavailable",
      requestId: "",
      fieldErrors: [],
    });
  });

  it("sets the switch cookie from Go's response instead of the requested organization", async () => {
    authState.session = { accessToken: "private-token" };
    authState.token = "private-token";
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      Response.json({
        user: { id: "user-1" },
        homeOrganizationId: "org-a",
        effectiveOrganizationId: "org-canonical",
        selectionRequired: false,
        organizations: [
          {
            id: "org-canonical",
            name: "Canonical Organization",
            roles: ["listingkit_viewer"],
          },
        ],
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const response = await call(
      PUT,
      new NextRequest(
        "http://localhost/api/workbench/context/effective-organization",
        {
          method: "PUT",
          body: JSON.stringify({ organizationId: "org-requested" }),
        },
      ),
      ["context", "effective-organization"],
    );

    expect(response.headers.get("Set-Cookie")).toContain(
      "shuomi_effective_organization=org-canonical",
    );
  });

  it("preserves a Go body/header mismatch rejection and leaves the cookie unchanged", async () => {
    authState.session = { accessToken: "private-token" };
    authState.token = "private-token";
    const payload = {
      code: "INVALID_REQUEST",
      message: "Request is invalid",
      requestId: "req-1",
      fieldErrors: [],
    };
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockResolvedValueOnce(Response.json(payload, { status: 400 })),
    );

    const response = await call(
      PUT,
      new NextRequest(
        "http://localhost/api/workbench/context/effective-organization",
        {
          method: "PUT",
          body: JSON.stringify({ organizationId: "org-requested" }),
        },
      ),
      ["context", "effective-organization"],
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toEqual(payload);
    expect(response.headers.get("Set-Cookie")).toBeNull();
  });

  it("rejects a non-allowlisted path before fetch", async () => {
    authState.session = { accessToken: "private-token" };
    authState.token = "private-token";
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

    const response = await call(
      GET,
      new NextRequest(
        "http://localhost/api/workbench/https://attacker.example",
      ),
      ["https:", "", "attacker.example"],
    );

    expect(response.status).toBe(404);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("proxies Store create with the typed 201 response contract", async () => {
    authState.session = { accessToken: "private-token" };
    authState.token = "private-token";
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(storePayload, { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await call(
      POST,
      new NextRequest("http://localhost/api/workbench/stores", {
        method: "POST",
        headers: {
          "Idempotency-Key": operationKey,
          cookie: "shuomi_effective_organization=org-cookie",
          "X-Expected-Organization-ID": "org-cookie",
        },
        body: JSON.stringify({ name: "Store", platform: "shein", region: "SG" }),
      }),
      ["stores"],
    );

    expect(response.status).toBe(201);
    await expect(response.json()).resolves.toEqual(storePayload);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0]?.[1]?.redirect).toBe("manual");
    const headers = new Headers(fetchMock.mock.calls[0]?.[1]?.headers);
    expect(headers.get("Idempotency-Key")).toBe(operationKey);
    expect(headers.get("X-Requested-Organization-ID")).toBe("org-cookie");
  });

  it("proxies Store delete and validates its response as delete, not item", async () => {
    authState.session = { accessToken: "private-token" };
    authState.token = "private-token";
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      Response.json({ id: storeId, deleted: true, version: 3 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const response = await call(
      DELETE,
      new NextRequest(`http://localhost/api/workbench/stores/${storeId}`, {
        method: "DELETE",
        headers: {
          "Idempotency-Key": operationKey,
          "If-Match": '"2"',
          cookie: "shuomi_effective_organization=org-cookie",
          "X-Expected-Organization-ID": "org-cookie",
        },
      }),
      ["stores", storeId],
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({
      id: storeId,
      deleted: true,
      version: 3,
    });
  });

  it.each(["HEAD", "PATCH", "OPTIONS"])(
    "explicitly rejects %s without contacting upstream",
    async (method) => {
      const fetchMock = vi.fn<typeof fetch>();
      vi.stubGlobal("fetch", fetchMock);
      const handler = (
        workbenchRoute as unknown as Record<string, typeof GET | undefined>
      )[method];

      expect(handler).toBeTypeOf("function");
      if (!handler) return;
      const response = await call(
        handler,
        new NextRequest("http://localhost/api/workbench/not-allowlisted", {
          method,
        }),
        ["not-allowlisted"],
      );

      expect(response.status).toBe(405);
      await expect(response.json()).resolves.toMatchObject({
        code: "INVALID_REQUEST",
      });
      expect(fetchMock).not.toHaveBeenCalled();
    },
  );
});
