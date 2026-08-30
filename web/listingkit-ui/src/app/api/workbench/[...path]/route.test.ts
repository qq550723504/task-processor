import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

const authState = vi.hoisted(() => ({
  session: null as Record<string, unknown> | null,
  token: "",
}));

const authMocks = vi.hoisted(() => ({
  wrapper: vi.fn(
    (handler: (request: NextRequest & { auth?: unknown }, context: unknown) => unknown) =>
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

import { GET, PUT } from "@/app/api/workbench/[...path]/route";

async function call(
  handler: typeof GET,
  request: NextRequest,
  path: string[],
) {
  const response = await handler(request, {
    params: Promise.resolve({ path }),
  });
  if (!(response instanceof Response)) {
    throw new Error("Workbench route did not return a response");
  }
  return response;
}

describe("/api/workbench BFF", () => {
  afterEach(() => {
    authState.session = null;
    authState.token = "";
    vi.clearAllMocks();
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
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

  it("sets the switch cookie from Go's response instead of the requested organization", async () => {
    authState.session = { accessToken: "private-token" };
    authState.token = "private-token";
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      Response.json({
        user: { id: "user-1" },
        homeOrganizationId: "org-a",
        effectiveOrganizationId: "org-canonical",
        selectionRequired: false,
        organizations: [],
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
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        Response.json(payload, { status: 400 }),
      ),
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
      new NextRequest("http://localhost/api/workbench/https://attacker.example"),
      ["https:", "", "attacker.example"],
    );

    expect(response.status).toBe(404);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
