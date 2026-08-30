import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

const mockedAuthState = vi.hoisted(() => ({
  session: null as Record<string, unknown> | null,
}));

const mockedServerToken = vi.hoisted(() => ({
  accessToken: "",
}));

vi.mock("@/auth", () => ({
  auth: vi.fn(() => Promise.resolve(mockedAuthState.session)),
}));

vi.mock("@/lib/server/zitadel-server-token", () => ({
  readZitadelServerAccessToken: vi.fn(() =>
    Promise.resolve(mockedServerToken.accessToken),
  ),
}));

import { GET } from "@/app/api/zitadel-auth/session/route";

function request() {
  return new NextRequest("http://localhost:3000/api/zitadel-auth/session");
}

describe("GET /api/zitadel-auth/session", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
    mockedAuthState.session = null;
    mockedServerToken.accessToken = "";
  });

  it("returns 503 when zitadel auth is unavailable", async () => {
    const response = await GET(request());
    const payload = (await response.json()) as {
      error?: string;
      message?: string;
    };

    expect(response.status).toBe(503);
    expect(payload.error).toBe("zitadel_auth_not_configured");
  });

  it("does not return a local debug identity when a retired auth bypass variable is set", async () => {
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");

    const response = await GET(request());
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

    const response = await GET(request());

    expect(response.status).toBe(401);
    expect(response.headers.get("X-User-ID")).toBeNull();
  });

  it("returns identity without exposing the server-side access token", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    mockedAuthState.session = {
      identityVersion: 2,
      identity: {
        tenantId: "org-286",
        userId: "user-1",
        roles: ["listingkit_operator"],
      },
    };
    mockedServerToken.accessToken = "access-token-1";

    const response = await GET(request());
    const payload = (await response.json()) as {
      ok?: boolean;
      identity?: { tenantId?: string };
      accessToken?: string;
    };

    expect(response.status).toBe(200);
    expect(payload.ok).toBe(true);
    expect(payload.identity?.tenantId).toBe("org-286");
    expect(payload.accessToken).toBeUndefined();
  });
});
