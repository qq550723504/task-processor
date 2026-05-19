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

  it("returns a local bypass identity in local development", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_TENANT_ID", "1");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_USER_ID", "local-admin");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_USER_TYPE", "local");
    vi.stubEnv("LISTINGKIT_UI_LOCAL_ROLES", "platform_admin,listingkit_admin");

    const response = await GET();
    const payload = (await response.json()) as {
      ok?: boolean;
      identity?: {
        tenantId?: string;
        userId?: string;
        userType?: string;
        roles?: string[];
      };
    };

    expect(response.status).toBe(200);
    expect(payload).toEqual({
      ok: true,
      identity: {
        tenantId: "1",
        userId: "local-admin",
        userType: "local",
        roles: ["platform_admin", "listingkit_admin"],
      },
    });
  });

  it("returns 503 when zitadel auth is unavailable and no bypass is enabled", async () => {
    const response = await GET();
    const payload = (await response.json()) as {
      error?: string;
      message?: string;
    };

    expect(response.status).toBe(503);
    expect(payload.error).toBe("zitadel_auth_not_configured");
  });
});
