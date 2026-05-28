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
});
