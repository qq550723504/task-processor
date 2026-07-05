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

  it("returns a local debug identity when auth gate bypass is enabled", async () => {
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");

    const response = await GET();
    const payload = (await response.json()) as {
      ok?: boolean;
      identity?: { username?: string; roles?: string[] };
    };

    expect(response.status).toBe(200);
    expect(payload.ok).toBe(true);
    expect(payload.identity?.username).toBe("local-debug");
    expect(payload.identity?.roles).toContain("platform_admin");
  });
});
