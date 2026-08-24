import { afterEach, describe, expect, it, vi } from "vitest";

const mockedAuthState = vi.hoisted(() => ({
  session: null as Record<string, unknown> | null,
}));

vi.mock("@/auth", () => ({
  auth: vi.fn(() => Promise.resolve(mockedAuthState.session)),
}));

import { GET } from "@/app/api/zitadel-auth/session/route";

describe("GET /api/zitadel-auth/session", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
    mockedAuthState.session = null;
  });

  it("returns 503 when zitadel auth is unavailable", async () => {
    const response = await GET();
    const payload = (await response.json()) as {
      error?: string;
      message?: string;
    };

    expect(response.status).toBe(503);
    expect(payload.error).toBe("zitadel_auth_not_configured");
    expect(payload.error).toBe("zitadel_auth_not_configured");
  });

  it("does not return a local debug identity when a retired auth bypass variable is set", async () => {
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");

    const response = await GET();
    expect(response.status).toBe(503);
  });

  it("rejects a session identity without a ZITADEL subject", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "listingkit-client");
    mockedAuthState.session = {
      accessToken: "access-token-1",
      identity: {
        tenantId: "org-286",
        user_id: "373211204509761704",
        roles: ["listingkit_admin"],
      },
    };

    const response = await GET();

    expect(response.status).toBe(401);
    expect(response.headers.get("X-User-ID")).toBeNull();
  });
});
