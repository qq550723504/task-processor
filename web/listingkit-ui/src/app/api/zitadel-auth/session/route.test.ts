import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

const mockedAuthState = vi.hoisted(() => ({
  session: null as Record<string, unknown> | null,
}));

const mockedServerToken = vi.hoisted(() => ({
  accessToken: "",
}));

vi.mock("@/auth", () => ({
  serverAuth: vi.fn(
    (handler?: (request: NextRequest & { auth?: unknown }) => unknown) => {
      if (typeof handler !== "function") {
        throw new Error("session route must use the Auth.js route wrapper");
      }
      return async (request: NextRequest) => {
        const response = await handler(
          Object.assign(request, { auth: mockedAuthState.session }),
        );
        if (!(response instanceof Response)) {
          return response;
        }
        const wrappedResponse = new Response(response.body, response);
        wrappedResponse.headers.append(
          "set-cookie",
          "authjs.session-token=refreshed-session; Path=/; HttpOnly",
        );
        return wrappedResponse;
      };
    },
  ),
}));

vi.mock("@/lib/server/zitadel-server-token", () => ({
  readZitadelServerAccessToken: vi.fn(() => mockedServerToken.accessToken),
}));

import { GET } from "@/app/api/zitadel-auth/session/route";

function request() {
  return new NextRequest("http://localhost:3000/api/zitadel-auth/session");
}

async function callGET() {
  const response = await GET(request(), {} as never);
  if (!(response instanceof Response)) {
    throw new Error("ZITADEL session route did not return a response");
  }
  return response;
}

describe("GET /api/zitadel-auth/session", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
    mockedAuthState.session = null;
    mockedServerToken.accessToken = "";
  });

  it("returns 503 when zitadel auth is unavailable", async () => {
    const response = await callGET();
    const payload = (await response.json()) as {
      error?: string;
      message?: string;
    };

    expect(response.status).toBe(503);
    expect(payload.error).toBe("zitadel_auth_not_configured");
  });

  it("does not return a local debug identity when a retired auth bypass variable is set", async () => {
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");

    const response = await callGET();
    expect(response.status).toBe(503);
  });

  it("rejects a session identity without a ZITADEL subject", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    mockedAuthState.session = {
      identity: {
        tenantId: "org-286",
        user_id: "373211204509761704",
        roles: ["listingkit_admin"],
      },
    };
    mockedServerToken.accessToken = "access-token-1";

    const response = await callGET();

    expect(response.status).toBe(401);
    expect(response.headers.get("X-User-ID")).toBeNull();
  });

  it("returns identity without exposing the server-side access token", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    mockedAuthState.session = {
      identityVersion: 3,
      identity: {
        tenantId: "org-286",
        userId: "user-1",
        roles: ["listingkit_operator"],
      },
    };
    mockedServerToken.accessToken = "access-token-1";

    const response = await callGET();
    const payload = (await response.json()) as {
      ok?: boolean;
      identity?: { tenantId?: string };
      accessToken?: string;
    };

    expect(response.status).toBe(200);
    expect(payload.ok).toBe(true);
    expect(payload.identity?.tenantId).toBe("org-286");
    expect(payload.accessToken).toBeUndefined();
    expect(response.headers.get("set-cookie")).toContain(
      "authjs.session-token=refreshed-session",
    );
  });
});
